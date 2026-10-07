// Bulkhead Zero: a top-down sci-fi shooter prototype.
package main

import (
	"log"

	"github.com/dbackowski/bulkhead-zero/internal/game"
	"github.com/hajimehoshi/ebiten/v2"
)

func main() {
	ebiten.SetWindowTitle("Bulkhead Zero")
	ebiten.SetWindowSize(game.ScreenW*3, game.ScreenH*3)
	ebiten.SetWindowResizingMode(ebiten.WindowResizingModeEnabled)

	g, err := game.New()
	if err != nil {
		log.Fatal(err)
	}
	if err := ebiten.RunGame(g); err != nil && err != ebiten.Termination {
		log.Fatal(err)
	}
}
