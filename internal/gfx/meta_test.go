package gfx

import (
	"testing"

	"github.com/dbackowski/bulkhead-zero/assets"
)

// The game looks animations up by name; this catches renames in tools/artgen.
func TestSpriteMetaHasWhatTheGameUses(t *testing.T) {
	m, err := LoadSpriteMeta(assets.FS)
	if err != nil {
		t.Fatal(err)
	}
	need := map[string][]string{
		"player.png": {"death"},
		"enemies.png": {"crawler.walk_right", "crawler.attack_right", "crawler.death",
			"spitter.idle", "spitter.attack", "spitter.death", "drone.idle", "drone.attack", "drone.death"},
		"fx.png": {"bullet", "laser", "hit_spark", "acid_glob", "acid_splash", "explosion", "muzzle_flash", "crosshair"},
	}
	for _, f := range []string{"down", "down_right", "right", "up_right", "up"} {
		need["player.png"] = append(need["player.png"], "idle_"+f, "walk_"+f, "hurt_"+f)
	}
	for sheet, names := range need {
		for _, n := range names {
			if _, ok := m.Sheets[sheet][n]; !ok {
				t.Errorf("%s: missing animation %q", sheet, n)
			}
		}
	}
	for _, w := range []string{"rifle", "shotgun", "pistol"} {
		if _, ok := m.Weapons[w]; !ok {
			t.Errorf("missing weapon %q", w)
		}
	}
	if len(m.Player.Facings) != 8 || len(m.Player.HandAnchor) != 8 {
		t.Errorf("want 8 facings and 8 hand anchors, got %d and %d", len(m.Player.Facings), len(m.Player.HandAnchor))
	}
}

func TestTileMetaHasAutotilePieces(t *testing.T) {
	m, err := LoadTileMeta(assets.FS)
	if err != nil {
		t.Fatal(err)
	}
	idx, err := m.TileIndex()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{"floor_plate_a", "wall_face", "door_open", "crate", "barrel_explosive", "decal_scorch",
		"wall_cap_00", "wall_cap_15", "wall_inner_NE", "wall_inner_SW"} {
		if _, ok := idx[n]; !ok {
			t.Errorf("missing tile %q", n)
		}
	}
}
