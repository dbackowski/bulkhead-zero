package game

import (
	"testing"

	"github.com/dbackowski/bulkhead-zero/assets"
)

// The game plays sounds by name; this catches renames in tools/artgen/make_sfx.py.
func TestSoundsExist(t *testing.T) {
	clips, err := loadClips(assets.FS)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range append([]string{"hit"}, weaponOrder...) {
		if len(clips[name]) == 0 || len(clips[name][0]) == 0 {
			t.Errorf("missing sound %q", name)
		}
	}
}
