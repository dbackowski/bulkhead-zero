package world

import (
	"math"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
)

// Unreachable marks tiles the flow field can't reach.
const Unreachable = math.MaxInt32

// DistanceField is a breadth-first flow field: steps from every walkable tile
// to the tile containing target. Enemies walk downhill on it to find the
// player around walls. It is cheap (40x26 tiles) so it's rebuilt a few times
// per second.
func (l *Level) DistanceField(target geom.Vec) []int {
	dist := make([]int, l.W*l.H)
	for i := range dist {
		dist[i] = Unreachable
	}
	tx, ty := int(target.X/T), int(target.Y/T)
	if !l.in(tx, ty) {
		return dist
	}
	queue := []int{ty*l.W + tx}
	dist[ty*l.W+tx] = 0
	for len(queue) > 0 {
		i := queue[0]
		queue = queue[1:]
		x, y := i%l.W, i/l.W
		for _, d := range [4][2]int{{1, 0}, {-1, 0}, {0, 1}, {0, -1}} {
			nx, ny := x+d[0], y+d[1]
			if l.SolidTile(nx, ny, false) {
				continue
			}
			j := ny*l.W + nx
			if dist[j] == Unreachable {
				dist[j] = dist[i] + 1
				queue = append(queue, j)
			}
		}
	}
	return dist
}

// NextStep returns the centre of the neighbouring tile that is closest to the
// field's target, allowing diagonals only when both sides are open (no corner
// cutting). ok is false when pos is unreachable or already on the target tile.
func (l *Level) NextStep(field []int, pos geom.Vec) (geom.Vec, bool) {
	x, y := int(pos.X/T), int(pos.Y/T)
	if !l.in(x, y) {
		return pos, false
	}
	best := field[y*l.W+x]
	if best == 0 || best == Unreachable {
		return pos, false
	}
	bx, by := x, y
	for dy := -1; dy <= 1; dy++ {
		for dx := -1; dx <= 1; dx++ {
			nx, ny := x+dx, y+dy
			if (dx == 0 && dy == 0) || l.SolidTile(nx, ny, false) {
				continue
			}
			if dx != 0 && dy != 0 && (l.SolidTile(x+dx, y, false) || l.SolidTile(x, y+dy, false)) {
				continue
			}
			if v := field[ny*l.W+nx]; v < best {
				best, bx, by = v, nx, ny
			}
		}
	}
	if bx == x && by == y {
		return pos, false
	}
	return geom.V(float64(bx*T+12), float64(by*T+14)), true
}
