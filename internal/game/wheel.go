package game

import (
	"fmt"
	"image/color"
	"math"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/dbackowski/bulkhead-zero/internal/gfx"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// The weapon wheel: hold Q and every gun appears in a ring (pistol at the top,
// then clockwise in key order). Moving the mouse away from where it was when
// the wheel opened highlights the gun in that direction; releasing Q switches
// to it. A quick tap of Q without moving still cycles to the next gun. Time
// runs slowly while the wheel is open, the aim holds still and nothing fires.
const (
	wheelSlowMo   = 0.25 // game speed while the wheel is open
	wheelDeadzone = 10.0 // px the mouse must move before a gun is picked
	wheelTap      = 0.25 // seconds: a shorter press without picking is a tap
	wheelRadius   = 68.0
	wheelSlot     = 17.0 // radius of each gun's circle
)

type weaponWheel struct {
	open   bool
	heldT  float64
	origin geom.Vec // cursor when the wheel opened
	hover  int      // highlighted slot, -1 for none
}

// wheelSlotAt maps a direction (screen radians, 0 = right, +y down) to one of
// n slots placed clockwise from the top.
func wheelSlotAt(angle float64, n int) int {
	i := int(math.Round((angle + math.Pi/2) / (2 * math.Pi / float64(n))))
	return ((i % n) + n) % n
}

func wheelSlotPos(i, n int) geom.Vec {
	a := -math.Pi/2 + float64(i)*2*math.Pi/float64(n)
	return geom.V(ScreenW/2, ScreenH/2).Add(geom.FromAngle(a).Scale(wheelRadius))
}

// updateWheel runs in real time (it is not slowed down) and turns Q into the
// slot to switch to; while the wheel is open it holds the aim and the trigger.
func (g *Game) updateWheel(in Input) Input {
	w := &g.wheel
	if g.state != statePlaying || g.player.Dead {
		w.open = false
		return in
	}
	switch {
	case in.WheelHeld && !w.open:
		*w = weaponWheel{open: true, origin: in.Mouse, hover: -1}
	case in.WheelHeld:
		w.heldT += tick
		if v := in.Mouse.Sub(w.origin); v.Len() > wheelDeadzone {
			w.hover = wheelSlotAt(v.Angle(), len(g.weapons))
		}
	case w.open: // Q released
		w.open = false
		switch {
		case w.hover >= 0 && w.hover < g.unlocked:
			in.SelectSlot = w.hover
		case w.hover < 0 && w.heldT < wheelTap:
			in.Cycle = 1
		}
	}
	if w.open {
		in.Fire, in.FirePressed, in.Mouse = false, false, w.origin
	}
	return in
}

func (g *Game) drawWheel(dst *ebiten.Image) {
	w := &g.wheel
	if !w.open {
		return
	}
	f, p := g.lib.Font, g.player
	vector.FillRect(dst, 0, 0, ScreenW, ScreenH, color.RGBA{0, 0, 0, 90}, false)
	for i, wp := range g.weapons {
		c := wheelSlotPos(i, len(g.weapons))
		locked := i >= g.unlocked
		vector.FillCircle(dst, float32(c.X), float32(c.Y), wheelSlot, hudBg, true)
		switch {
		case i == w.hover && !locked:
			vector.StrokeCircle(dst, float32(c.X), float32(c.Y), wheelSlot, 2, hudLine, true)
		case i == p.Slot:
			vector.StrokeCircle(dst, float32(c.X), float32(c.Y), wheelSlot, 1, textAmber, true)
		default:
			vector.StrokeCircle(dst, float32(c.X), float32(c.Y), wheelSlot, 1, color.RGBA{60, 70, 84, 255}, true)
		}
		var m ebiten.GeoM
		m.Translate(math.Round(c.X)-12, math.Round(c.Y)-7)
		o := gfx.SpriteOpts{}
		if locked {
			o.Alpha = 0.25
		}
		gfx.DrawWithGeoM(dst, wp.Img, m, o)
		key := fmt.Sprint(i + 1)
		clr := color.Color(textAmber)
		if locked {
			clr = textDim
		}
		f.Draw(dst, key, int(c.X)-2, int(c.Y)+6, 1, clr)
	}
	// the highlighted gun's name and ammo in the middle
	name, info := "MOVE THE MOUSE", "RELEASE Q TO SWITCH"
	if i := w.hover; i >= 0 {
		name = g.weapons[i].Name
		switch {
		case i >= g.unlocked:
			info = "LOCKED"
		case g.weapons[i].Ammo == 0:
			info = fmt.Sprintf("%d / UNLIMITED", p.Mag[i])
		default:
			info = fmt.Sprintf("%d / %d", p.Mag[i], p.Spare[i])
		}
	}
	cx, cy := ScreenW/2, ScreenH/2
	f.Draw(dst, name, cx-f.Width(name, 1)/2, cy-9, 1, textWhite)
	f.Draw(dst, info, cx-f.Width(info, 1)/2, cy+3, 1, textDim)
}
