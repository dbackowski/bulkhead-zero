package game

import (
	"bytes"
	"io/fs"
	"testing"

	"github.com/dbackowski/bulkhead-zero/assets"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

// The game plays sounds by name; this catches renames in tools/artgen/make_sfx.py.
func TestSoundsExist(t *testing.T) {
	clips, err := loadClips(assets.FS)
	if err != nil {
		t.Fatal(err)
	}
	names := append([]string{"hit_flesh", "hit_metal", "hit_wall", "hurt", "explosion",
		"player_death", "laser", "spit", "acid_splash", "door", "boss_roar", "spine", "pickup", "reload", "dry_fire", "warden_alarm", "burrow",
		"stinger_wave", "stinger_clear", "stinger_boss", "stinger_death", "stinger_victory"}, weaponOrder...)
	for _, n := range deathSound {
		names = append(names, n)
	}
	for _, name := range names {
		if len(clips[name]) == 0 || len(clips[name][0]) == 0 {
			t.Errorf("missing sound %q", name)
		}
	}
}

func TestMusicExists(t *testing.T) {
	for _, name := range musicTracks {
		b, err := fs.ReadFile(assets.FS, "music/music_"+name+".wav")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(b)); err != nil {
			t.Errorf("music %q: %v", name, err)
		}
	}
}

// Calm while nothing hunts the player, combat once something does (lingering
// a little), the boss theme while a boss lives.
func TestMusicTrack(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	step := func(secs float64) {
		for i := 0; i < int(secs*60); i++ {
			g.player.Invuln = 1
			if err := g.Update(); err != nil {
				t.Fatal(err)
			}
		}
	}
	if got := g.musicTrack(); got != "calm" {
		t.Fatalf("at the start: %s", got)
	}
	e := g.enemies[0]
	g.alert(e)
	step(0.1)
	if got := g.musicTrack(); got != "combat" {
		t.Fatalf("with a hunter: %s", got)
	}
	for _, e := range g.enemies {
		if e.Aggro && e.State != stDead {
			g.kill(e)
		}
	}
	step(combatLinger - 1)
	if got := g.musicTrack(); got != "combat" {
		t.Fatalf("just after the fight: %s", got)
	}
	step(2)
	if got := g.musicTrack(); got != "calm" {
		t.Fatalf("well after the fight: %s", got)
	}
	g.boss = g.spawnHunter('M', g.lvl.Boss)
	if got := g.musicTrack(); got != "boss" {
		t.Fatalf("with a boss: %s", got)
	}
}
