// Package assets embeds the generated art, sounds and music (see tools/artgen) into the binary.
package assets

import "embed"

//go:embed tiles/tileset.png tiles/tiles.json sprites/*.png sprites/sprites.json sounds/*.wav music/*.wav
var FS embed.FS
