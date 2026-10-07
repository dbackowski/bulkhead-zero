package game

import (
	"fmt"
	"image/color"
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
	vector.DrawFilledRect(dst, 4, ScreenH-22, 154, 18, hudBg, false)
	vector.StrokeRect(dst, 4, ScreenH-22, 154, 18, 1, hudLine, false)
	for i := 0; i < playerMaxHP; i++ {
		c := hpEmpty
		if i < p.HP {
			c = hpFull
		}
		vector.DrawFilledRect(dst, float32(8+i*6), ScreenH-17, 4, 8, c, false)
	}
	w := g.weapons[p.Slot]
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(70, ScreenH-19)
	dst.DrawImage(w.Img, op)
	ebitenutil.DebugPrintAt(dst, fmt.Sprintf("%d %s", p.Slot+1, strings.ToUpper(w.Name)), 96, ScreenH-21)

	// hostiles counter, top-left
	ebitenutil.DebugPrintAt(dst, fmt.Sprintf("HOSTILES %d/%d", g.aliveFoes(), g.totalFoes), 4, 2)

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
			printCentered(dst, "WASD MOVE  MOUSE AIM  LMB FIRE\n1-3 / Q / WHEEL WEAPON  ESC PAUSE", 40)
		}
	case statePaused:
		vector.DrawFilledRect(dst, 0, 0, ScreenW, ScreenH, dimScreen, false)
		printCentered(dst, "PAUSED\n\nESC TO RESUME   R TO RESTART   F FULLSCREEN", 110)
	case stateDead:
		vector.DrawFilledRect(dst, 0, 0, ScreenW, ScreenH, dimScreen, false)
		printCentered(dst, "YOU DIED\n\nR OR CLICK TO TRY AGAIN", 110)
	case stateClear:
		vector.DrawFilledRect(dst, 0, 0, ScreenW, ScreenH, dimScreen, false)
		printCentered(dst, "DECK CLEARED\n\nR OR CLICK TO PLAY AGAIN", 110)
	}
}
