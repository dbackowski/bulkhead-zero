package gfx

import (
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
)

// Feet is where the ground contact sits inside every 24x24 frame.
const FeetX, FeetY = 12, 21

// SpriteOpts tweaks how a frame is drawn.
type SpriteOpts struct {
	Flip  bool    // mirror horizontally (left facings)
	Flash float64 // 0..1 blend toward white (hit flash)
	Alpha float64 // 0 means fully opaque
}

// DrawAtFeet draws a 24x24 frame so its feet land on screen position (x, y).
func DrawAtFeet(dst, img *ebiten.Image, x, y float64, o SpriteOpts) {
	var g ebiten.GeoM
	if o.Flip {
		g.Scale(-1, 1)
		g.Translate(float64(img.Bounds().Dx()), 0)
	}
	g.Translate(math.Round(x-FeetX), math.Round(y-FeetY))
	draw(dst, img, g, o)
}

// DrawCentered draws img centred on (x, y), rotated by rot radians.
func DrawCentered(dst, img *ebiten.Image, x, y, rot float64) {
	w, h := float64(img.Bounds().Dx()), float64(img.Bounds().Dy())
	var g ebiten.GeoM
	g.Translate(-w/2, -h/2)
	if rot != 0 {
		g.Rotate(rot)
	}
	g.Translate(math.Round(x), math.Round(y))
	draw(dst, img, g, SpriteOpts{})
}

// DrawWithGeoM draws img with a caller-built transform.
func DrawWithGeoM(dst, img *ebiten.Image, g ebiten.GeoM, o SpriteOpts) { draw(dst, img, g, o) }

func draw(dst, img *ebiten.Image, g ebiten.GeoM, o SpriteOpts) {
	alpha := 1.0
	if o.Alpha > 0 {
		alpha = o.Alpha
	}
	if o.Flash > 0 {
		var cm colorm.ColorM
		k := o.Flash
		cm.Scale(1-k, 1-k, 1-k, alpha)
		cm.Translate(k, k, k, 0)
		op := &colorm.DrawImageOptions{}
		op.GeoM = g
		colorm.DrawImage(dst, img, cm, op)
		return
	}
	op := &ebiten.DrawImageOptions{GeoM: g}
	if alpha < 1 {
		op.ColorScale.ScaleAlpha(float32(alpha))
	}
	dst.DrawImage(img, op)
}
