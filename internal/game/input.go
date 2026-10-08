package game

import (
	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Input is one tick of player intent. Real devices fill it in readInput; the
// autopilot (dev/test tool) can produce it instead.
type Input struct {
	Move        geom.Vec // raw direction, normalised later
	Mouse       geom.Vec // cursor in logical screen pixels
	Fire        bool     // trigger held
	FirePressed bool     // trigger pressed this tick (semi-auto weapons)
	SelectSlot  int      // 0..n-1, or -1
	Cycle       int      // -1, 0, +1
	Restart     bool
	Pause       bool
	Fullscreen  bool
	Mute        bool
}

func readInput(g *Game) Input {
	var in Input
	in.SelectSlot = -1
	if ebiten.IsKeyPressed(ebiten.KeyA) || ebiten.IsKeyPressed(ebiten.KeyArrowLeft) {
		in.Move.X--
	}
	if ebiten.IsKeyPressed(ebiten.KeyD) || ebiten.IsKeyPressed(ebiten.KeyArrowRight) {
		in.Move.X++
	}
	if ebiten.IsKeyPressed(ebiten.KeyW) || ebiten.IsKeyPressed(ebiten.KeyArrowUp) {
		in.Move.Y--
	}
	if ebiten.IsKeyPressed(ebiten.KeyS) || ebiten.IsKeyPressed(ebiten.KeyArrowDown) {
		in.Move.Y++
	}
	mx, my := ebiten.CursorPosition()
	in.Mouse = geom.V(float64(mx), float64(my))
	in.Fire = ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft)
	in.FirePressed = inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft)
	for i, k := range []ebiten.Key{ebiten.Key1, ebiten.Key2, ebiten.Key3} {
		if inpututil.IsKeyJustPressed(k) {
			in.SelectSlot = i
		}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyQ) {
		in.Cycle = 1
	}
	if _, wy := ebiten.Wheel(); wy > 0 {
		in.Cycle = -1
	} else if wy < 0 {
		in.Cycle = 1
	}
	in.Restart = inpututil.IsKeyJustPressed(ebiten.KeyR) ||
		((g.state == stateDead || g.state == stateClear) && g.stateT > 1 && in.FirePressed)
	in.Pause = inpututil.IsKeyJustPressed(ebiten.KeyEscape) || inpututil.IsKeyJustPressed(ebiten.KeyP)
	in.Fullscreen = inpututil.IsKeyJustPressed(ebiten.KeyF)
	in.Mute = inpututil.IsKeyJustPressed(ebiten.KeyM)
	return in
}
