package game

import (
	"fmt"
	"image/color"
	"math"
	"strings"

	"github.com/dbackowski/bulkhead-zero/internal/gfx"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/ebitenutil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var (
	hudBg     = color.RGBA{12, 14, 20, 230}
	hudLine   = color.RGBA{70, 214, 230, 255}
	hpFull    = color.RGBA{232, 68, 60, 255}
	hpEmpty   = color.RGBA{60, 24, 30, 255}
	bossHP    = color.RGBA{150, 232, 60, 255}
	dimScreen = color.RGBA{0, 0, 0, 150}
)

const charW = 6 // ebitenutil debug font

func printCentered(dst *ebiten.Image, msg string, y int) {
	lines := strings.Split(msg, "\n")
	for i, l := range lines {
		ebitenutil.DebugPrintAt(dst, l, (ScreenW-len(l)*charW)/2, y+i*16)
	}
}

func (g *Game) drawHUD(dst *ebiten.Image) {
	p := g.player

	// health pips + weapon box, bottom-left
	vector.FillRect(dst, 4, ScreenH-22, 154, 18, hudBg, false)
	vector.StrokeRect(dst, 4, ScreenH-22, 154, 18, 1, hudLine, false)
	for i := 0; i < playerMaxHP; i++ {
		c := hpEmpty
		if i < p.HP {
			c = hpFull
		}
		vector.FillRect(dst, float32(8+i*6), ScreenH-17, 4, 8, c, false)
	}
	w := g.weapons[p.Slot]
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(70, ScreenH-19)
	dst.DrawImage(w.Img, op)
	ebitenutil.DebugPrintAt(dst, fmt.Sprintf("%d %s", p.Slot+1, strings.ToUpper(w.Name)), 96, ScreenH-21)

	// wave and hostiles, top-left; the boss gets a health bar instead
	if b := g.boss; b != nil {
		ebitenutil.DebugPrintAt(dst, "BROOD MOTHER", ScreenW/2-36, 2)
		vector.FillRect(dst, ScreenW/2-80, 18, 160, 6, hpEmpty, false)
		vector.FillRect(dst, ScreenW/2-80, 18, 160*float32(max(b.HP, 0))/float32(stats[boss].hp), 6, bossHP, false)
		vector.StrokeRect(dst, ScreenW/2-81, 17, 162, 8, 1, hudLine, false)
	} else {
		ebitenutil.DebugPrintAt(dst, fmt.Sprintf("WAVE %d/%d  HOSTILES %d/%d", g.wave, g.waveCount(),
			g.aliveFoes()+len(g.queue), g.totalFoes), 4, 2)
	}
	if g.state == statePlaying && g.bannerT > 0 {
		printCentered(dst, g.banner, 60)
	}
	if g.state == statePlaying && g.nextIn > 0 && g.aliveFoes() == 0 && len(g.queue) == 0 {
		next := fmt.Sprintf("WAVE %d", g.wave+1)
		if g.wave == g.waveCount() {
			next = "SOMETHING BIG"
		}
		printCentered(dst, fmt.Sprintf("%s IN %d", next, int(math.Ceil(g.nextIn))), 100)
	}

	// crosshair at the cursor
	if g.state == statePlaying {
		mx, my := ebiten.CursorPosition()
		if g.autopilot != nil {
			mx, my = int(g.autopilot.mouse.X), int(g.autopilot.mouse.Y)
		}
		gfx.DrawCentered(dst, g.lib.A("fx.crosshair").Frame(g.t), float64(mx), float64(my), 0)
	}

	switch g.state {
	case statePlaying:
		if g.t < 7 {
			printCentered(dst, "WASD MOVE  MOUSE AIM  LMB FIRE\n1-6 / Q / WHEEL WEAPON  ESC PAUSE", 40)
		}
	case statePaused:
		vector.FillRect(dst, 0, 0, ScreenW, ScreenH, dimScreen, false)
		printCentered(dst, "PAUSED\n\nESC TO RESUME   R TO RESTART   F FULLSCREEN   M MUTE", 110)
	case stateDead:
		vector.FillRect(dst, 0, 0, ScreenW, ScreenH, dimScreen, false)
		printCentered(dst, "YOU DIED\n\nR OR CLICK TO TRY AGAIN", 110)
	case stateClear:
		vector.FillRect(dst, 0, 0, ScreenW, ScreenH, dimScreen, false)
		printCentered(dst, "BROOD MOTHER DEAD - DECK CLEARED\n\nR OR CLICK TO PLAY AGAIN", 110)
	}
}
