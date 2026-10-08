// Package assets embeds the generated art and sounds (see tools/artgen) into the binary.
package assets

import "embed"

//go:embed tiles/tileset.png tiles/tiles.json sprites/*.png sprites/sprites.json sounds/*.wav
var FS embed.FS
