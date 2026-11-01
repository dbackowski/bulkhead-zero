package world

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
)

func station(t *testing.T) *Level {
	t.Helper()
	l, err := Parse(Station)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

func TestStationParses(t *testing.T) {
	l := station(t)
	if l.W != 40 || l.H != 26 {
		t.Fatalf("size %dx%d", l.W, l.H)
	}
	if len(l.Spawns) == 0 {
		t.Fatal("no enemy spawns")
	}
	if l.RectBlocked(l.Start, 4, 3, false) {
		t.Fatal("player starts inside something solid")
	}
	if l.Boss == (geom.Vec{}) || l.RectBlocked(l.Boss, 8, 5, false) {
		t.Fatalf("bad boss spawn %v", l.Boss)
	}
	inner, outer := 0, 0
	for _, d := range l.Doors {
		if d.Inner {
			inner++
		} else {
			outer++
		}
	}
	if inner != 2 || outer != 5 {
		t.Fatalf("want 2 inner and 5 outer doors, got %d and %d", inner, outer)
	}
}

func TestDoorsBlockUntilOpen(t *testing.T) {
	l := station(t)
	d := l.DoorAt(6, 14)
	c := d.Center()
	if d == nil || !l.WallAt(c) || l.LineOfSight(c.Sub(geom.V(0, T)), c.Add(geom.V(0, T))) {
		t.Fatal("a closed door should block like a wall")
	}
	l.SetDoorOpen(d, 1)
	if l.WallAt(c) || l.SolidAt(c, false) {
		t.Fatal("an open door should let things through")
	}
}

// Every enemy must be able to reach the player and vice versa, and every floor
// tile should be reachable (catches walled-off rooms when editing a map).
// Every map needs a boss spot and outer doors for the waves.
func TestMapsAreConnected(t *testing.T) {
	for name, rows := range map[string][]string{"Station": Station, "Reactor": Reactor} {
		t.Run(name, func(t *testing.T) { checkConnected(t, rows) })
	}
}

func checkConnected(t *testing.T, rows []string) {
	l, err := Parse(rows)
	if err != nil {
		t.Fatal(err)
	}
	if l.Boss == (geom.Vec{}) || l.RectBlocked(l.Boss, 8, 5, false) {
		t.Errorf("bad boss spot %v", l.Boss)
	}
	outer := 0
	for _, d := range l.Doors {
		if !d.Inner {
			outer++
			if l.SolidAt(d.Center().Add(geom.V(0, T)), false) {
				t.Errorf("outer door %d,%d opens onto something solid", d.TX, d.TY)
			}
		}
	}
	if outer == 0 {
		t.Error("no outer doors for waves to come in through")
	}
	field := l.DistanceField(l.Start)
	for _, s := range l.Spawns {
		tx, ty := int(s.Pos.X/T), int(s.Pos.Y/T)
		if field[ty*l.W+tx] == Unreachable {
			t.Errorf("spawn %c at tile %d,%d is unreachable", s.Kind, tx, ty)
		}
		if l.RectBlocked(s.Pos, 4, 3, false) {
			t.Errorf("spawn %c at tile %d,%d starts inside something solid", s.Kind, tx, ty)
		}
	}
	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			if !l.SolidTile(x, y, false) && field[y*l.W+x] == Unreachable {
				t.Errorf("floor tile %d,%d is unreachable", x, y)
			}
		}
	}
}

// The Go autotiler must agree with tools/artgen/make_tiles.py, which drew the
// previews. Face variants differ on purpose ('#' picks one by hash), so faces
// are compared as a category.
func TestAutotileMatchesPython(t *testing.T) {
	b, err := os.ReadFile("testdata/autotile_golden.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Rows  []string            `json:"rows"`
		Cells map[string][]string `json:"cells"`
	}
	if err := json.Unmarshal(b, &golden); err != nil {
		t.Fatal(err)
	}
	// the sample has no '@'; add one on a floor tile so Parse accepts it
	rows := append([]string(nil), golden.Rows...)
	rows[2] = rows[2][:1] + "@" + rows[2][2:]
	l, err := Parse(rows)
	if err != nil {
		t.Fatal(err)
	}
	for key, want := range golden.Cells {
		var x, y int
		fmt.Sscanf(key, "%d,%d", &x, &y)
		got := l.WallTiles(x, y)
		if strings.HasPrefix(got[0], "wall_face") || strings.HasPrefix(got[0], "door") {
			got = append([]string{"FACE"}, got[1:]...)
		}
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("cell %s: got %v, want %v", key, got, want)
		}
	}
}

func TestMoveAndSlideStopsAtWalls(t *testing.T) {
	l := station(t)
	p := l.Start
	for i := 0; i < 600; i++ { // push hard to the left for 10 seconds
		p = l.MoveAndSlide(p, geom.V(-3, 0.4), 4, 3, false)
		if l.RectBlocked(p, 4, 3, false) {
			t.Fatalf("ended inside a wall at %v", p)
		}
	}
	if p.X > T*2 {
		t.Fatalf("expected to reach the west wall, stopped at %v", p)
	}
}

func TestLineOfSight(t *testing.T) {
	l := station(t)
	// across the open hall
	if !l.LineOfSight(geom.V(16*T, 3*T), geom.V(26*T, 3*T)) {
		t.Error("expected clear sight across the hall")
	}
	// start room to the lab, through the wall between them
	if l.LineOfSight(geom.V(4*T, 20*T), geom.V(4*T, 8*T)) {
		t.Error("expected the wall between rooms to block sight")
	}
}

func TestNextStepWalksDownhill(t *testing.T) {
	l := station(t)
	target := geom.V(20*T+12, 4*T+14) // hall
	for _, d := range l.Doors {
		l.SetDoorOpen(d, 1) // the game opens doors for whoever walks up
	}
	field := l.DistanceField(target)
	p := l.Start
	for i := 0; i < 2000; i++ {
		next, ok := l.NextStep(field, p)
		if !ok {
			break
		}
		p = l.MoveAndSlide(p, next.Sub(p).Norm().Scale(1.5), 4, 3, false)
	}
	if p.Dist(target) > T*1.5 {
		t.Fatalf("follower got stuck at %v, target %v", p, target)
	}
}
