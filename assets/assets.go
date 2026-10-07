// Package assets embeds the generated art (see tools/artgen) into the binary.
package assets

import "embed"

//go:embed tiles/tileset.png tiles/tiles.json sprites/*.png sprites/sprites.json
var FS embed.FS
