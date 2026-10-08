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
	names := append([]string{"hit_flesh", "hit_metal", "hit_wall", "hurt", "explosion",
		"player_death", "laser", "spit", "acid_splash"}, weaponOrder...)
	for _, n := range deathSound {
		names = append(names, n)
	}
	for _, name := range names {
		if len(clips[name]) == 0 || len(clips[name][0]) == 0 {
			t.Errorf("missing sound %q", name)
		}
	}
}
