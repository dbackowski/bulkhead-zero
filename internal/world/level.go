// Package world holds the tile map: parsing, wall autotiling, collision and
// line of sight. It has no graphics dependency so it is easy to unit test.
package world

import (
	"fmt"
	"math"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
)

// T is the tile size in pixels.
const T = 24

// Map characters. Anything not listed is plain floor.
//
//	walls:  # (face picked by hash)  L light  P pipe  ! warning  D closed door
//	doors:  = open door (a wall-face tile you can walk and shoot through)
//	floors: . plate  g grate  h hazard  v vent  s light strip
//	decals: a acid  z scorch  ~ cables
//	props:  x crate  c console  k locker  t terminal  b toxic barrel  B explosive barrel
//	spawns: @ player  C crawler  S spitter  R drone
const wallChars = "#LP!D="

var floorChars = map[byte]string{'g': "floor_grate", 'h': "floor_hazard", 'v': "floor_vent", 's': "floor_lightstrip"}
var decalChars = map[byte]string{'a': "decal_acid", 'z': "decal_scorch", '~': "decal_cables"}
var propChars = map[byte]string{'x': "crate", 'c': "console", 'k': "locker", 't': "terminal", 'b': "barrel_toxic", 'B': "barrel_explosive"}

// Prop is a solid one-tile object. Explosive barrels can be shot.
type Prop struct {
	Name      string
	TX, TY    int
	HP        int
	Alive     bool
	Explosive bool
}

// Feet is the prop's ground anchor (for y-sorting with characters).
func (p *Prop) Feet() geom.Vec { return geom.V(float64(p.TX*T+12), float64(p.TY*T+21)) }

// Center is the middle of the prop's tile.
func (p *Prop) Center() geom.Vec { return geom.V(float64(p.TX*T+12), float64(p.TY*T+12)) }

// Spawn is an enemy start position (feet).
type Spawn struct {
	Kind byte
	Pos  geom.Vec
}

// Decal is a flat overlay drawn on the floor.
type Decal struct {
	Name   string
	TX, TY int
}

// Level is a parsed map.
type Level struct {
	W, H   int
	Rows   []string
	Start  geom.Vec
	Spawns []Spawn
	Props  []*Prop
	Decals []Decal
	solid  []bool // walls + living props
	wall   []bool // walls only (doors '=' excluded)
	propAt map[int]*Prop
}

// Parse builds a level from equal-width rows.
func Parse(rows []string) (*Level, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("empty map")
	}
	l := &Level{W: len(rows[0]), H: len(rows), Rows: rows, propAt: map[int]*Prop{}}
	l.solid = make([]bool, l.W*l.H)
	l.wall = make([]bool, l.W*l.H)
	foundStart := false
	for y, row := range rows {
		if len(row) != l.W {
			return nil, fmt.Errorf("row %d is %d wide, want %d", y, len(row), l.W)
		}
		for x := 0; x < l.W; x++ {
			c := row[x]
			i := y*l.W + x
			feet := geom.V(float64(x*T+12), float64(y*T+16))
			switch {
			case c == '=':
				// open door: drawn as a wall face but passable
			case isWallChar(c):
				l.solid[i], l.wall[i] = true, true
			case c == '@':
				l.Start, foundStart = feet, true
			case c == 'C' || c == 'S' || c == 'R':
				l.Spawns = append(l.Spawns, Spawn{Kind: c, Pos: feet})
			}
			if name, ok := propChars[c]; ok {
				p := &Prop{Name: name, TX: x, TY: y, Alive: true, Explosive: c == 'B', HP: 3}
				l.Props = append(l.Props, p)
				l.propAt[i] = p
				l.solid[i] = true
			}
			if name, ok := decalChars[c]; ok {
				l.Decals = append(l.Decals, Decal{Name: name, TX: x, TY: y})
			}
		}
	}
	if !foundStart {
		return nil, fmt.Errorf("map has no player start '@'")
	}
	return l, nil
}

func isWallChar(c byte) bool {
	for i := 0; i < len(wallChars); i++ {
		if wallChars[i] == c {
			return true
		}
	}
	return false
}

func (l *Level) in(x, y int) bool { return x >= 0 && y >= 0 && x < l.W && y < l.H }

// IsWallCell reports whether (x, y) draws as wall for autotiling. Doors count
// as wall here; off-map counts as wall.
func (l *Level) IsWallCell(x, y int) bool {
	if !l.in(x, y) {
		return true
	}
	return isWallChar(l.Rows[y][x])
}

func hash(x, y int) int {
	h := uint32(x)*73856093 ^ uint32(y)*19349663
	h ^= h >> 13
	h *= 0x5bd1e995
	h ^= h >> 15
	return int(h % 100)
}

func faceTile(c byte, x, y int) string {
	switch c {
	case 'L':
		return "wall_face_light"
	case 'P':
		return "wall_face_pipe"
	case '!':
		return "wall_face_warning"
	case 'D':
		return "door_closed"
	case '=':
		return "door_open"
	}
	switch h := hash(x, y); {
	case h < 62:
		return "wall_face"
	case h < 85:
		return "wall_face_light"
	default:
		return "wall_face_pipe"
	}
}

// WallTiles returns the tile names to stack for a wall cell (the autotile rule):
// a wall with floor directly below shows its front face; any other wall is a
// roof cap chosen by which sides touch floor (N=1, E=2, S=4, W=8), plus inner
// corner overlays where both orthogonal neighbours are wall but the diagonal
// is floor. The first element tells you which case it was.
func (l *Level) WallTiles(x, y int) []string {
	if !l.IsWallCell(x, y+1) {
		return []string{faceTile(l.Rows[y][x], x, y)}
	}
	mask := 0
	if !l.IsWallCell(x, y-1) {
		mask |= 1
	}
	if !l.IsWallCell(x+1, y) {
		mask |= 2
	}
	if !l.IsWallCell(x-1, y) {
		mask |= 8
	}
	out := []string{fmt.Sprintf("wall_cap_%02d", mask)}
	for _, d := range []struct {
		dx, dy int
		name   string
	}{{1, -1, "NE"}, {-1, -1, "NW"}, {1, 1, "SE"}, {-1, 1, "SW"}} {
		if l.IsWallCell(x+d.dx, y) && l.IsWallCell(x, y+d.dy) && !l.IsWallCell(x+d.dx, y+d.dy) {
			out = append(out, "wall_inner_"+d.name)
		}
	}
	return out
}

// FloorTile returns the floor tile to draw under cell (x, y).
func (l *Level) FloorTile(x, y int) string {
	if n, ok := floorChars[l.Rows[y][x]]; ok {
		return n
	}
	switch h := hash(x+101, y+57); {
	case h < 62:
		return "floor_plate_a"
	case h < 87:
		return "floor_plate_b"
	default:
		return "floor_plate_dirty"
	}
}

// SolidTile reports whether a tile blocks movement. Flyers ignore props.
func (l *Level) SolidTile(tx, ty int, flying bool) bool {
	if !l.in(tx, ty) {
		return true
	}
	i := ty*l.W + tx
	if flying {
		return l.wall[i]
	}
	return l.solid[i]
}

// SolidAt reports whether the pixel position is inside a blocking tile.
func (l *Level) SolidAt(p geom.Vec, flying bool) bool {
	return l.SolidTile(int(math.Floor(p.X/T)), int(math.Floor(p.Y/T)), flying)
}

// WallAt reports whether the pixel is inside a wall (props ignored).
func (l *Level) WallAt(p geom.Vec) bool { return l.SolidAt(p, true) }

// PropAt returns the living prop covering pixel p, if any.
func (l *Level) PropAt(p geom.Vec) *Prop {
	tx, ty := int(math.Floor(p.X/T)), int(math.Floor(p.Y/T))
	if !l.in(tx, ty) {
		return nil
	}
	pr := l.propAt[ty*l.W+tx]
	if pr == nil || !pr.Alive {
		return nil
	}
	return pr
}

// RemoveProp makes a prop disappear and stop blocking.
func (l *Level) RemoveProp(p *Prop) {
	p.Alive = false
	l.solid[p.TY*l.W+p.TX] = false
}

// RectBlocked reports whether an axis-aligned box centred at c overlaps a blocking tile.
func (l *Level) RectBlocked(c geom.Vec, hw, hh float64, flying bool) bool {
	x0, x1 := int(math.Floor((c.X-hw)/T)), int(math.Floor((c.X+hw-0.001)/T))
	y0, y1 := int(math.Floor((c.Y-hh)/T)), int(math.Floor((c.Y+hh-0.001)/T))
	for ty := y0; ty <= y1; ty++ {
		for tx := x0; tx <= x1; tx++ {
			if l.SolidTile(tx, ty, flying) {
				return true
			}
		}
	}
	return false
}

// MoveAndSlide moves a box by delta, one axis at a time, so it slides along walls.
// It moves in small steps so fast movers can't pass through thin walls.
func (l *Level) MoveAndSlide(pos, delta geom.Vec, hw, hh float64, flying bool) geom.Vec {
	steps := int(math.Ceil(delta.Len() / 4))
	if steps < 1 {
		steps = 1
	}
	d := delta.Scale(1 / float64(steps))
	for i := 0; i < steps; i++ {
		if nx := pos.Add(geom.V(d.X, 0)); !l.RectBlocked(nx, hw, hh, flying) {
			pos = nx
		}
		if ny := pos.Add(geom.V(0, d.Y)); !l.RectBlocked(ny, hw, hh, flying) {
			pos = ny
		}
	}
	return pos
}

// LineOfSight reports whether no wall lies between a and b (props don't block sight).
func (l *Level) LineOfSight(a, b geom.Vec) bool {
	d := b.Sub(a)
	n := int(d.Len()/6) + 1
	for i := 1; i < n; i++ {
		if l.WallAt(a.Add(d.Scale(float64(i) / float64(n)))) {
			return false
		}
	}
	return true
}

// PixelSize is the full map size in pixels.
func (l *Level) PixelSize() (int, int) { return l.W * T, l.H * T }
