package game

// Development helpers, off unless their environment variables are set:
//
//	BZ_AUTOPILOT=1   a simple bot plays the game (handy for soak tests and demos);
//	                 BZ_AUTOPILOT=human gives it shaky aim and slow reactions
//	BZ_RECORD=dir    save every 3rd frame as PNG into dir, then quit after
//	                 BZ_RECORD_SECONDS (default 40) seconds of game time
//	BZ_DECK=2        start on that deck (1-based) instead of the first
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
	fieldAt   geom.Vec // target the field was built for
	fieldTime float64

	// handicap, to play roughly like a person: aim wanders up to aimErr px
	// around the target, and it holds fire for reaction s after spotting one
	// (a target out of sight for under 0.5 s still counts as spotted)
	aimErr, reaction float64
	wobble           geom.Vec
	seen, lost       float64

	// strafing side (flipped when that side is blocked), and getting unstuck:
	// wanting to move but staying put for a moment triggers a short detour
	flip            float64
	lastPos, detour geom.Vec
	stillT, detourT float64

	// weapon wheel: when nothing is close it switches guns like a person
	// would with Q (hold, sweep the mouse toward the gun, release)
	wheelTo     int // slot being picked, -1 when not using the wheel
	wheelT      float64
	wheelOrigin geom.Vec
}

func newAutopilot() *autopilot {
	return &autopilot{mouse: geom.V(ScreenW/2+60, ScreenH/2), flip: 1, wheelTo: -1}
}

// Bot timing for the weapon wheel: how long it holds Q, and how close an
// enemy may be before it switches with a number key instead.
const (
	botWheelHold = 0.45
	botWheelCalm = 120.0
)

func newHumanAutopilot() *autopilot {
	a := newAutopilot()
	a.aimErr, a.reaction = 10, 0.35
	return a
}

func (a *autopilot) input(g *Game, _ Input) Input {
	in := Input{SelectSlot: -1}
	a.t += dt
	p := g.player
	if g.state == stateDead || g.state == stateClear {
		in.Restart = g.stateT > 2.5
		return in
	}
	var vis, near, dig *Enemy
	var dv, dn, score float64
	for _, e := range g.enemies {
		if _, warn, wt := e.mawPhase(); warn && wt > a.reaction && e.Pos.Dist(p.Pos) < mawBurst {
			dig = e // about to burst out right here
		}
		if e.State == stDead || e.hidden() || e.Shielded {
			continue // a shielded Overmind isn't worth a shot: go for its nodes
		}
		d := e.Pos.Dist(p.Pos)
		if near == nil || d < dn {
			near, dn = e, d
		}
		s := d // rather shoot something whose shield isn't in the way
		if e.blocks(e.Pos.Sub(p.Pos)) {
			s += 150
		}
		if d < 250 && g.lvl.LineOfSight(p.Pos.Sub(geom.V(0, 8)), e.eye()) && (vis == nil || s < score) {
			vis, dv, score = e, d, s
		}
	}

	// aim
	aimAt := p.Pos.Add(geom.FromAngle(a.t * 0.8).Scale(70))
	if vis != nil {
		aimAt = vis.Pos.Sub(geom.V(0, vis.height()-2))
	}
	if a.aimErr > 0 {
		a.wobble = a.wobble.Add(geom.V(g.rng.NormFloat64(), g.rng.NormFloat64()).Scale(a.aimErr * 0.25))
		if l := a.wobble.Len(); l > a.aimErr {
			a.wobble = a.wobble.Scale(a.aimErr / l)
		}
		aimAt = aimAt.Add(a.wobble)
	}
	if vis != nil {
		a.seen, a.lost = a.seen+dt, 0
	} else if a.lost += dt; a.lost > 0.5 {
		a.seen = 0
	}
	target := aimAt.Sub(geom.V(math.Round(g.cam.X), math.Round(g.cam.Y)))
	a.mouse = a.mouse.Lerp(target, 0.3)
	in.Mouse = a.mouse

	// weapon choice: shotgun up close, rifle otherwise, pistol now and then
	rifle, smg, plasma, pistol := g.slotOf("rifle"), g.slotOf("smg"), g.slotOf("plasma"), g.slotOf("pistol")
	slot := rifle
	switch {
	case vis != nil && dv < 70 && vis.Kind == crawler && g.slotOf("shotgun") < g.unlocked:
		slot = g.slotOf("shotgun")
	case math.Mod(a.t, 24) > 20:
		slot = pistol
	}
	if slot == rifle && smg < g.unlocked {
		slot = smg // the smg out-damages the rifle
	}
	if slot == smg && plasma < g.unlocked && vis != nil && vis.Kind == warden {
		slot = plasma // accurate at the range the Warden keeps
	}
	if h := p.Heavy; h >= 0 && vis != nil && (!g.weapons[h].Flame || dv < 80) {
		slot = h // a heavy weapon for whatever is in sight (fire only up close)
	}
	if p.ReloadT > 0 {
		slot = p.Slot // switching would throw the reload away
	}
	if p.Mag[slot] == 0 && !g.canReload(slot) {
		slot = pistol // it never runs dry
	}
	switch {
	case a.wheelTo >= 0: // mid-pick: hold Q and sweep toward the gun, then let go
		a.wheelT += tick
		in.WheelHeld = a.wheelT < botWheelHold
		dir := wheelSlotPos(min(a.wheelTo, g.guns), g.wheelSpots()).Sub(geom.V(ScreenW/2, ScreenH/2)).Norm()
		in.Mouse = a.wheelOrigin.Add(dir.Scale(40 * math.Min(a.wheelT/(botWheelHold*0.7), 1)))
		if !in.WheelHeld {
			a.wheelTo = -1
		}
	case slot != p.Slot && (vis == nil || dv > botWheelCalm) && p.ReloadT == 0:
		a.wheelTo, a.wheelT, a.wheelOrigin = slot, 0, in.Mouse
		in.WheelHeld = true
	case slot != p.Slot:
		in.SelectSlot = slot
	}
	guarded := vis != nil && vis.blocks(vis.Pos.Sub(p.Pos)) // its shield faces us: circle it
	if vis != nil && !guarded {
		onTarget := math.Abs(angleDiff(target.Sub(g.handPos().Sub(g.cam)).Angle(), p.Aim)) < 0.3 && a.seen >= a.reaction
		in.Fire = onTarget
		in.FirePressed = onTarget && int(a.t*60)%14 == 0
	}
	if vis != nil && g.weapons[p.Slot].SpinMS > 0 {
		in.Fire = true // keep the minigun spinning between bursts
	}

	in.Reload = vis == nil && p.Mag[p.Slot] < g.weapons[p.Slot].Mag/2

	// the nearest pickup it has a use for
	var pick *Pickup
	for _, pk := range g.pickups {
		if g.wants(pk) && (pk.Kind == pickWeapon || pk.T < pickupLife-3) && (pick == nil || pk.Pos.Dist(p.Pos) < pick.Pos.Dist(p.Pos)) {
			pick = pk
		}
	}
	free := func(dir geom.Vec, px float64) bool { // a step of px that way isn't blocked
		return !g.lvl.RectBlocked(p.Pos.Add(dir.Norm().Scale(px)), playerHW, playerHH, false)
	}
	goTo := func(target geom.Vec) {
		a.fieldTime -= dt
		if a.fieldTime <= 0 || a.fieldAt.Dist(target) > 12 {
			a.field, a.fieldAt, a.fieldTime = g.lvl.DistanceField(target), target, 0.5
		}
		if next, ok := g.lvl.NextStep(a.field, p.Pos); ok {
			in.Move = next.Sub(p.Pos)
		} else {
			in.Move = target.Sub(p.Pos)
		}
	}

	// movement: back off from close melee enemies, strafe while shooting at
	// mid range, walk the path toward far targets, otherwise grab pickups,
	// otherwise hunt. Sideways moves switch sides when that side is blocked.
	backOff := map[enemyKind]float64{crawler: 40, bomber: 70, brute: 55, brood: 60, maw: 50, splitter: 45, shielder: 45, overmind: 60}
	var to, side geom.Vec
	if vis != nil {
		to = vis.Pos.Sub(p.Pos).Norm()
		side = geom.V(-to.Y, to.X).Scale(math.Copysign(1, math.Sin(a.t*0.9)) * a.flip)
		if !free(side, 14) {
			a.flip, side = -a.flip, side.Scale(-1)
		}
		if !free(side, 14) {
			side = geom.Vec{} // no room to strafe (a corridor, a doorway)
		}
	}
	switch {
	case dig != nil: // step off the marked spot
		in.Move = p.Pos.Sub(dig.Pos)
		if in.Move.Len() < 1 {
			in.Move = geom.FromAngle(a.flip)
		}
	case a.detourT > 0:
		a.detourT -= dt
		in.Move = a.detour
	case guarded && dv < 90 && side != (geom.Vec{}):
		in.Move = side.Add(to.Scale((dv - 60) / 60))
	case vis != nil && dv < backOff[vis.Kind]:
		in.Move = to.Scale(-1).Add(side.Scale(0.6))
		if !free(in.Move, 6) {
			in.Move = side
		}
	case vis != nil && dv <= 150 && side != (geom.Vec{}):
		in.Move = side
	case vis != nil:
		goTo(vis.Pos)
	case pick != nil:
		goTo(pick.Pos)
	case near != nil:
		goTo(near.Pos)
	}

	// safety net: wanting to move but not getting anywhere for a moment means
	// something is in the way, so take a short detour in a free direction
	if in.Move.Len() < 0.1 || p.Pos.Dist(a.lastPos) > 3 {
		a.lastPos, a.stillT = p.Pos, 0
	} else if a.stillT += dt; a.stillT > 0.75 && a.detourT <= 0 {
		for i := 0; i < 8; i++ {
			d := geom.FromAngle(in.Move.Angle() + math.Pi/2 + float64(i)*math.Pi/4)
			if free(d, 6) {
				a.detour, a.detourT = d, 0.5
				break
			}
		}
		a.stillT = 0
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
	if float64(r.frame)*tick >= r.limit {
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
