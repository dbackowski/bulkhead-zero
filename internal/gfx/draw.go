package gfx

import (
	"image/color"
	"math"
	"strings"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/colorm"
)

// Feet is where the ground contact sits inside every 24x24 frame. Bigger
// frames keep the same rule: (width/2, height-3).
const FeetX, FeetY = 12, 21

// SpriteOpts tweaks how a frame is drawn.
type SpriteOpts struct {
	Flip  bool    // mirror horizontally (left facings)
	Flash float64 // 0..1 blend toward white (hit flash)
	Alpha float64 // 0 means fully opaque
}

// DrawAtFeet draws a frame so its feet land on screen position (x, y).
func DrawAtFeet(dst, img *ebiten.Image, x, y float64, o SpriteOpts) {
	w, h := img.Bounds().Dx(), img.Bounds().Dy()
	var g ebiten.GeoM
	if o.Flip {
		g.Scale(-1, 1)
		g.Translate(float64(w), 0)
	}
	g.Translate(math.Round(x-float64(w/2)), math.Round(y-float64(h-3)))
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

// Font is the generated pixel font (font.png): fixed-width, uppercase only.
type Font struct {
	glyphs  map[rune]*ebiten.Image
	advance int // px per character at scale 1
	height  int // glyph height without the outline
	origin  [2]int
}

// Width is how many pixels s takes at the given scale.
func (f *Font) Width(s string, scale int) int {
	n := len([]rune(s))
	if n == 0 {
		return 0
	}
	return (n*f.advance - 1) * scale
}

// Height is the glyph height (no outline) at the given scale.
func (f *Font) Height(scale int) int { return f.height * scale }

// Draw writes s with its top-left glyph pixel at (x, y), scaled by a whole
// number and tinted clr. Lower case is drawn as upper case; unknown characters
// leave a gap.
func (f *Font) Draw(dst *ebiten.Image, s string, x, y, scale int, clr color.Color) {
	for _, c := range strings.ToUpper(s) {
		g, ok := f.glyphs[c]
		if ok {
			op := &ebiten.DrawImageOptions{}
			op.GeoM.Translate(float64(-f.origin[0]), float64(-f.origin[1]))
			op.GeoM.Scale(float64(scale), float64(scale))
			op.GeoM.Translate(float64(x), float64(y))
			op.ColorScale.ScaleWithColor(clr)
			dst.DrawImage(g, op)
		}
		x += f.advance * scale
	}
}
