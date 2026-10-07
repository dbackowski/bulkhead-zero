package game

// Development helpers, off unless their environment variables are set:
//
//	BZ_AUTOPILOT=1   a simple bot plays the game (handy for soak tests and demos)
//	BZ_RECORD=dir    save every 3rd frame as PNG into dir, then quit after
//	                 BZ_RECORD_SECONDS (default 40) seconds of game time
//
// Example: BZ_AUTOPILOT=1 BZ_RECORD=/tmp/frames go run .

import (
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strconv"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/hajimehoshi/ebiten/v2"
)

type autopilot struct {
	mouse     geom.Vec
	t         float64
	field     []int
	fieldFor  *Enemy
	fieldTime float64
}

func newAutopilot() *autopilot { return &autopilot{mouse: geom.V(ScreenW/2+60, ScreenH/2)} }

func (a *autopilot) input(g *Game, _ Input) Input {
	in := Input{SelectSlot: -1}
	a.t += dt
	p := g.player
	if g.state == stateDead || g.state == stateClear {
		in.Restart = g.stateT > 2.5
		return in
	}
	var vis, near *Enemy
	var dv, dn float64
	for _, e := range g.enemies {
		if e.State == stDead {
			continue
		}
		d := e.Pos.Dist(p.Pos)
		if near == nil || d < dn {
			near, dn = e, d
		}
		if d < 250 && g.lvl.LineOfSight(p.Pos.Sub(geom.V(0, 8)), e.eye()) && (vis == nil || d < dv) {
			vis, dv = e, d
		}
	}

	// aim
	aimAt := p.Pos.Add(geom.FromAngle(a.t * 0.8).Scale(70))
	if vis != nil {
		aimAt = vis.Pos.Sub(geom.V(0, vis.height()-2))
	}
	target := aimAt.Sub(geom.V(math.Round(g.cam.X), math.Round(g.cam.Y)))
	a.mouse = a.mouse.Lerp(target, 0.3)
	in.Mouse = a.mouse

	// weapon choice: shotgun up close, rifle otherwise, pistol now and then
	slot := 0
	switch {
	case vis != nil && dv < 70 && vis.Kind == crawler:
		slot = 1
	case math.Mod(a.t, 24) > 20:
		slot = 2
	}
	if slot != p.Slot {
		in.SelectSlot = slot
	}
	if vis != nil {
		onTarget := math.Abs(angleDiff(target.Sub(g.handPos().Sub(g.cam)).Angle(), p.Aim)) < 0.3
		in.Fire = onTarget
		in.FirePressed = onTarget && int(a.t*60)%14 == 0
	}

	// movement: back off from close crawlers, strafe while shooting, otherwise hunt
	switch {
	case vis != nil && vis.Kind == crawler && dv < 40:
		in.Move = p.Pos.Sub(vis.Pos).Norm().Add(geom.V(math.Sin(a.t*2), 0).Scale(0.6))
	case vis != nil:
		to := vis.Pos.Sub(p.Pos).Norm()
		in.Move = geom.V(-to.Y, to.X).Scale(math.Copysign(1, math.Sin(a.t*0.9)))
		if dv > 150 {
			in.Move = in.Move.Add(to)
		}
	case near != nil:
		a.fieldTime -= dt
		if a.fieldFor != near || a.fieldTime <= 0 {
			a.field, a.fieldFor, a.fieldTime = g.lvl.DistanceField(near.Pos), near, 0.5
		}
		if next, ok := g.lvl.NextStep(a.field, p.Pos); ok {
			in.Move = next.Sub(p.Pos)
		} else {
			in.Move = near.Pos.Sub(p.Pos)
		}
	}
	return in
}

func angleDiff(a, b float64) float64 {
	d := math.Mod(a-b+math.Pi, 2*math.Pi)
	if d < 0 {
		d += 2 * math.Pi
	}
	return d - math.Pi
}

type recorder struct {
	dir     string
	every   int
	limit   float64
	frame   int
	pending bool
	saved   int
	done    bool
}

func newRecorder(dir string) *recorder {
	limit := 40.0
	if s, err := strconv.ParseFloat(os.Getenv("BZ_RECORD_SECONDS"), 64); err == nil && s > 0 {
		limit = s
	}
	_ = os.MkdirAll(dir, 0o755)
	return &recorder{dir: dir, every: 3, limit: limit}
}

func (r *recorder) tick(g *Game) {
	r.frame++
	r.pending = r.frame%r.every == 0
	if float64(r.frame)*dt >= r.limit {
		r.done = true
	}
}

func (r *recorder) capture(src *ebiten.Image) {
	if !r.pending {
		return
	}
	r.pending = false
	b := src.Bounds()
	img := image.NewRGBA(b)
	src.ReadPixels(img.Pix)
	f, err := os.Create(filepath.Join(r.dir, fmt.Sprintf("frame_%05d.png", r.saved)))
	if err != nil {
		return
	}
	defer f.Close()
	_ = png.Encode(f, img)
	r.saved++
}

// Finished reports whether a recording run has reached its time limit.
func (g *Game) Finished() bool { return g.recorder != nil && g.recorder.done }
