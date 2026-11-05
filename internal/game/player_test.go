package game

import (
	"math"
	"testing"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
)

func TestFacingFor(t *testing.T) {
	f := []string{"right", "down_right", "down", "down_left", "left", "up_left", "up", "up_right"}
	cases := map[float64]string{
		0:                "right",
		math.Pi / 4:      "down_right",
		math.Pi / 2:      "down",
		math.Pi:          "left",
		-math.Pi:         "left",
		-math.Pi / 2:     "up",
		-math.Pi / 4:     "up_right",
		-3 * math.Pi / 4: "up_left",
		0.38:             "right",      // 21.8°, still inside the right sector
		0.42:             "down_right", // 24°, just past the boundary
	}
	for a, want := range cases {
		if got := facingFor(f, a); got != want {
			t.Errorf("facingFor(%.2f) = %s, want %s", a, got, want)
		}
	}
}

func TestWheelSlotAt(t *testing.T) {
	cases := map[float64]int{-math.Pi / 2: 0, -math.Pi / 6: 1, math.Pi / 6: 2, math.Pi / 2: 3, 5 * math.Pi / 6: 4, -5 * math.Pi / 6: 5, 0.1: 2, -0.1: 1}
	for a, want := range cases {
		if got := wheelSlotAt(a, 6); got != want {
			t.Errorf("wheelSlotAt(%.2f) = %d, want %d", a, got, want)
		}
	}
}

// Hold Q, point at a gun, release: that gun is selected; time runs slowly and
// the gun doesn't fire while the wheel is open. A quick tap still cycles.
func TestWeaponWheel(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	g.enemies = nil
	g.unlocked = 3 // through the shotgun
	mouse := geom.V(300, 120)
	step := func(in Input) Input {
		in.SelectSlot = -1
		in.Mouse = in.Mouse.Add(mouse)
		return g.updateWheel(in)
	}
	shotgun := g.slotOf("shotgun")
	dir := wheelSlotPos(shotgun, g.wheelSpots()).Sub(geom.V(ScreenW/2, ScreenH/2)).Norm().Scale(40)
	step(Input{WheelHeld: true})
	in := step(Input{WheelHeld: true, Fire: true, Mouse: dir})
	if !g.wheel.open || in.Fire || in.Mouse != mouse {
		t.Fatal("while open the wheel should hold the trigger and the aim")
	}
	if g.wheel.hover != shotgun {
		t.Fatalf("pointing at the shotgun highlights slot %d", g.wheel.hover)
	}
	in = step(Input{Mouse: dir})
	if g.wheel.open || in.SelectSlot != shotgun {
		t.Fatalf("releasing Q should pick the shotgun, got slot %d", in.SelectSlot)
	}
	// pointing at a locked gun does nothing
	locked := g.slotOf("launcher")
	dir = wheelSlotPos(locked, g.wheelSpots()).Sub(geom.V(ScreenW/2, ScreenH/2)).Norm().Scale(40)
	step(Input{WheelHeld: true})
	step(Input{WheelHeld: true, Mouse: dir})
	if in = step(Input{Mouse: dir}); in.SelectSlot != -1 {
		t.Fatalf("picked locked slot %d", in.SelectSlot)
	}
	// a quick tap cycles
	step(Input{WheelHeld: true})
	if in = step(Input{}); in.Cycle != 1 {
		t.Fatal("a tap of Q should cycle")
	}
	// time slows while it is open
	before := g.t
	for i := 0; i < 2; i++ {
		if err := g.step(Input{SelectSlot: -1, WheelHeld: true, Mouse: mouse}); err != nil {
			t.Fatal(err)
		}
	}
	if got, want := g.t-before, 2*tick*wheelSlowMo; math.Abs(got-want) > 1e-9 {
		t.Fatalf("game time advanced %.4f with the wheel open, want %.4f", got, want)
	}
}
