package game

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/dbackowski/bulkhead-zero/internal/gfx"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var (
	hudBg     = color.RGBA{12, 14, 20, 230}
	hudLine   = color.RGBA{70, 214, 230, 255}
	hpFull    = color.RGBA{232, 68, 60, 255}
	hpEmpty   = color.RGBA{60, 24, 30, 255}
	bossHP    = color.RGBA{150, 232, 60, 255}
	dimScreen = color.RGBA{0, 0, 0, 150}

	textWhite = color.RGBA{232, 240, 248, 255}
	textDim   = color.RGBA{150, 160, 176, 255}
	textAmber = color.RGBA{238, 176, 58, 255}
	textRed   = color.RGBA{232, 68, 60, 255}
	textGreen = color.RGBA{150, 232, 60, 255}
)

// printCentered draws msg centred on the screen from y down. With a title
// colour, the first line is drawn at 2x in it; the rest is normal size.
func (g *Game) printCentered(dst *ebiten.Image, msg string, y int, title color.Color) {
	f := g.lib.Font
	for i, l := range strings.Split(msg, "\n") {
		scale, clr := 1, color.Color(textWhite)
		if i == 0 && title != nil {
			scale, clr = 2, title
		}
		f.Draw(dst, l, (ScreenW-f.Width(l, scale))/2, y, scale, clr)
		y += f.Height(scale) + 4*scale
	}
}

func (g *Game) drawHUD(dst *ebiten.Image) {
	p := g.player
	f := g.lib.Font

	// health pips + weapon box, bottom-left
	vector.FillRect(dst, 4, ScreenH-22, 214, 18, hudBg, false)
	vector.StrokeRect(dst, 4, ScreenH-22, 214, 18, 1, hudLine, false)
	for i := 0; i < playerMaxHP; i++ {
		c := hpEmpty
		if i < p.HP {
			c = hpFull
		}
		vector.FillRect(dst, float32(8+i*6), ScreenH-17, 4, 8, c, false)
	}
	w := g.weapons[p.Slot]
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(58, ScreenH-19)
	dst.DrawImage(w.Img, op)
	f.Draw(dst, fmt.Sprint(g.key(p.Slot)), 86, ScreenH-16, 1, textAmber)
	f.Draw(dst, w.Name, 94, ScreenH-16, 1, textWhite)
	ammo, ammoClr := fmt.Sprintf("%d/%d", p.Mag[p.Slot], p.Spare[p.Slot]), color.Color(textWhite)
	switch {
	case p.ReloadT > 0:
		ammo, ammoClr = "RELOAD", textAmber
		k := 1 - p.ReloadT/(w.ReloadMS/1000)
		vector.FillRect(dst, 172, ScreenH-7, float32(40*k), 1, textAmber, false)
	case w.Heavy:
		ammo, ammoClr = fmt.Sprint(p.Mag[p.Slot]), textAmber // all it has left
	case p.Mag[p.Slot] == 0 && !g.canReload(p.Slot):
		ammo, ammoClr = "EMPTY", textRed
	case w.Ammo == 0:
		ammo = fmt.Sprint(p.Mag[p.Slot]) // unlimited spare rounds
	}
	f.Draw(dst, ammo, 213-f.Width(ammo, 1), ScreenH-16, 1, ammoClr)

	// wave and hostiles, top-left; the boss gets a health bar instead
	if b := g.boss; b != nil {
		name := bossName[b.Kind]
		f.Draw(dst, name, (ScreenW-f.Width(name, 1))/2, 5, 1, textAmber)
		vector.FillRect(dst, ScreenW/2-80, 16, 160, 6, hpEmpty, false)
		vector.FillRect(dst, ScreenW/2-80, 16, 160*float32(max(b.HP, 0))/float32(stats[b.Kind].hp), 6, bossHP, false)
		vector.StrokeRect(dst, ScreenW/2-81, 15, 162, 8, 1, hudLine, false)
	} else {
		f.Draw(dst, fmt.Sprintf("DECK %d  WAVE %d/%d", g.level+1, g.wave, g.waveCount()), 5, 5, 1, textAmber)
		f.Draw(dst, fmt.Sprintf("HOSTILES %d/%d", g.aliveFoes()+len(g.queue), g.totalFoes), 5, 15, 1, textDim)
	}
	if g.state == statePlaying && g.bannerT > 0 && !g.wheel.open {
		title := color.Color(textAmber)
		if g.boss != nil {
			title = textRed
		}
		g.printCentered(dst, g.banner, 56, title)
	}
	if g.state == statePlaying && g.nextIn > 0 && g.aliveFoes() == 0 && len(g.queue) == 0 && !g.wheel.open {
		next := fmt.Sprintf("WAVE %d", g.wave+1)
		if g.wave == g.waveCount() {
			next = "SOMETHING BIG"
		}
		g.printCentered(dst, fmt.Sprintf("%s IN %d", next, int(math.Ceil(g.nextIn))), 100, nil)
	}

	g.drawWheel(dst)

	// crosshair at the cursor
	if g.state == statePlaying && !g.wheel.open {
		mx, my := ebiten.CursorPosition()
		if g.autopilot != nil {
			mx, my = int(g.autopilot.mouse.X), int(g.autopilot.mouse.Y)
		}
		gfx.DrawCentered(dst, g.lib.A("fx.crosshair").Frame(g.t), float64(mx), float64(my), 0)
	}

	switch g.state {
	case statePlaying:
		if g.t < 7 && g.bannerT <= 0 && !g.wheel.open {
			g.printCentered(dst, "WASD MOVE   MOUSE AIM   LMB FIRE   R RELOAD\n1-7 / HOLD Q / WHEEL WEAPON   ESC PAUSE", 40, nil)
		}
	case statePaused:
		vector.FillRect(dst, 0, 0, ScreenW, ScreenH, dimScreen, false)
		g.printCentered(dst, "PAUSED\n\nESC RESUME   R RESTART   F FULLSCREEN   M MUTE", 110, textWhite)
	case stateDead:
		vector.FillRect(dst, 0, 0, ScreenW, ScreenH, dimScreen, false)
		g.printCentered(dst, "YOU DIED\n\nR OR CLICK TO TRY AGAIN", 110, textRed)
	case stateClear:
		vector.FillRect(dst, 0, 0, ScreenW, ScreenH, dimScreen, false)
		g.printCentered(dst, "STATION CLEARED\nALL DECKS ARE SAFE\n\nR OR CLICK TO PLAY AGAIN", 110, textGreen)
	}
}
