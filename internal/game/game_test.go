package game

import (
	"math/rand"
	"testing"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
)

// Plays the level with the autopilot, without a window, and checks the round
// flows: enemies die, the deck can be cleared or lost, and restart works.
// On Linux this needs a display (xvfb-run go test ./...); macOS is fine as is.
func TestAutopilotRound(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	g.autopilot = newAutopilot()
	start := g.aliveFoes()
	for i := 0; i < 60*400 && g.state == statePlaying; i++ { // waves and boss: allow ~7 minutes
		if err := g.Update(); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("after %.1fs: state=%d wave=%d killed=%d hp=%d", g.t, g.state, g.wave, g.kills, g.player.HP)
	if g.kills < start/2 {
		t.Errorf("autopilot only killed %d enemies", g.kills)
	}
	if g.state == statePlaying {
		t.Error("round never ended")
	}
}

func TestDeathAndRestart(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for g.player.HP > 0 {
		g.player.Invuln = 0
		g.hurtPlayer(1, g.player.Pos)
	}
	for i := 0; i < 120 && g.state == statePlaying; i++ {
		if err := g.Update(); err != nil {
			t.Fatal(err)
		}
	}
	if g.state != stateDead {
		t.Fatalf("expected the dead state, got %d", g.state)
	}
	if err := g.reset(); err != nil {
		t.Fatal(err)
	}
	if g.player.HP != playerMaxHP || g.aliveFoes() != g.totalFoes || g.state != statePlaying {
		t.Fatal("restart did not restore the round")
	}
}

func TestBarrelChainReaction(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range g.lvl.Props {
		if p.Explosive {
			g.explode(p)
			break
		}
	}
	for i := 0; i < 30; i++ {
		g.updateBlasts()
	}
	n := 0
	for _, p := range g.lvl.Props {
		if p.Explosive && !p.Alive {
			n++
		}
	}
	if n < 1 || len(g.scorches) != n {
		t.Fatalf("exploded %d barrels, %d scorch marks", n, len(g.scorches))
	}
}

// Difficulty guard-rail: on every deck the handicapped autopilot (shaky aim,
// slow reactions) should usually beat the boss, but not without getting hurt.
// Run with -v to see the numbers when tuning.
func TestDifficulty(t *testing.T) {
	for lvl := range levels {
		t.Run(levels[lvl].name, func(t *testing.T) { checkDifficulty(t, lvl) })
	}
}

func checkDifficulty(t *testing.T, lvl int) {
	const rounds = 20
	clears, hpLeft := 0, 0
	for seed := int64(1); seed <= rounds; seed++ {
		g, err := New()
		if err != nil {
			t.Fatal(err)
		}
		g.rng = rand.New(rand.NewSource(seed))
		g.level = lvl
		if err := g.reset(); err != nil {
			t.Fatal(err)
		}
		g.autopilot = newHumanAutopilot()
		bossHP := -1
		for i := 0; i < 60*420 && g.level == lvl && g.state == statePlaying && !g.player.Dead; i++ {
			hp := g.player.HP
			if err := g.Update(); err != nil {
				t.Fatal(err)
			}
			if g.level != lvl || g.state == stateClear { // beat the boss: the next deck just started
				clears++
				hpLeft += hp
			} else if g.boss != nil {
				bossHP = g.boss.HP
			}
		}
		t.Logf("seed %2d: wave=%d boss HP=%d time=%.0fs dead=%v", seed, g.wave, bossHP, g.t, g.player.Dead)
	}
	t.Logf("cleared %d/%d, avg HP left %.1f/%d", clears, rounds, float64(hpLeft)/float64(max(clears, 1)), playerMaxHP)
	if clears < rounds/2 {
		t.Errorf("too hard: cleared only %d/%d", clears, rounds)
	}
	if hpLeft > clears*playerMaxHP*3/4 {
		t.Errorf("too easy: avg HP left %.1f of %d", float64(hpLeft)/float64(clears), playerMaxHP)
	}
}

// Killing everything walks through every wave of every deck (unlocking
// weapons and healing between waves) to its boss; beating a boss starts the
// next deck, and beating the last one clears the station.
func TestWavesThenBoss(t *testing.T) {
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
	killAll := func() {
		for _, e := range g.enemies {
			if e.State != stDead {
				g.kill(e)
			}
		}
	}
	for lvl, deck := range levels {
		if g.level != lvl || g.state != statePlaying {
			t.Fatalf("expected to be playing deck %d, at %d in state %d", lvl+1, g.level+1, g.state)
		}
		for w := 1; w <= g.waveCount(); w++ {
			if g.wave != w {
				t.Fatalf("deck %d: expected wave %d, got %d", lvl+1, w, g.wave)
			}
			killAll()
			step(waveBreak + 0.1)
			if want := min(deck.unlocked+w, len(g.weapons)); g.unlocked != want {
				t.Fatalf("deck %d after wave %d: %d weapons unlocked, want %d", lvl+1, w, g.unlocked, want)
			}
			step(spawnGap * 20) // let the next wave come in through the doors
			if w < g.waveCount() && g.aliveFoes() != len(deck.waves[w-1]) {
				t.Fatalf("deck %d wave %d: %d enemies in, want %d", lvl+1, w+1, g.aliveFoes(), len(deck.waves[w-1]))
			}
		}
		if g.boss == nil || g.boss.State == stDead || g.boss.Kind != kindFor[deck.boss] {
			t.Fatalf("deck %d: its boss should be out after the last wave", lvl+1)
		}
		killAll()
		step(3)
	}
	if g.state != stateClear {
		t.Fatalf("expected the station to be clear after the last boss, state %d", g.state)
	}
}

// Inner doors slide open when the player walks up and shut after they leave.
func TestDoorOpensForPlayer(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	g.enemies = nil
	d := g.lvl.DoorAt(6, 14)
	g.player.Pos = d.Center().Add(geom.V(0, 26))
	for i := 0; i < 30; i++ {
		g.updateDoors()
	}
	if d.Open < 1 || g.lvl.WallAt(d.Center()) {
		t.Fatalf("door should be open, at %.2f", d.Open)
	}
	g.player.Pos = g.lvl.Start
	for i := 0; i < 30; i++ {
		g.updateDoors()
	}
	if d.Open > 0 || !g.lvl.WallAt(d.Center()) {
		t.Fatalf("door should be shut, at %.2f", d.Open)
	}
}

// Emptying the rifle starts a reload that refills the magazine from the spare
// rounds; with no spare rounds left the gun stays empty.
func TestReload(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	g.enemies = nil
	r := g.slotOf("rifle")
	p, w := g.player, g.weapons[r]
	hold := Input{SelectSlot: -1, Fire: true}
	for i := 0; i < 600 && p.ReloadT == 0; i++ {
		g.updatePlayer(hold)
	}
	if p.Mag[r] != 0 || p.ReloadT == 0 {
		t.Fatalf("expected an empty, reloading rifle: mag %d, reload %.2f", p.Mag[r], p.ReloadT)
	}
	for i := 0; i < int(w.ReloadMS/1000*60)+2; i++ {
		g.updatePlayer(Input{SelectSlot: -1})
	}
	if p.Mag[r] != w.Mag || p.Spare[r] != w.Ammo-w.Mag {
		t.Fatalf("after reload: mag %d spare %d, want %d and %d", p.Mag[r], p.Spare[r], w.Mag, w.Ammo-w.Mag)
	}
	p.Mag[r], p.Spare[r] = 0, 0
	g.updatePlayer(Input{SelectSlot: -1, Reload: true})
	if p.ReloadT != 0 || g.canReload(r) {
		t.Fatal("reloaded with no spare rounds")
	}
}

func TestPickups(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	p := g.player
	g.pickups = []*Pickup{{Kind: pickHealth, Pos: p.Pos}}
	g.updatePickups()
	if len(g.pickups) != 1 {
		t.Fatal("health was taken at full HP")
	}
	p.HP = 3
	g.updatePickups()
	if len(g.pickups) != 0 || p.HP != 3+pickupHP {
		t.Fatalf("health pickup: %d left, HP %d", len(g.pickups), p.HP)
	}
	r := g.slotOf("rifle")
	p.Spare[r] = 0
	g.pickups = []*Pickup{{Kind: pickAmmo, Pos: p.Pos}}
	g.updatePickups()
	if p.Spare[r] != g.weapons[r].Ammo/2 {
		t.Fatalf("ammo pickup gave the rifle %d spare rounds", p.Spare[r])
	}
	g.pickups = []*Pickup{{Kind: pickAmmo, Pos: p.Pos.Add(geom.V(200, 0))}}
	for i := 0; i < int(pickupLife*60)+2; i++ {
		g.updatePickups()
	}
	if len(g.pickups) != 0 {
		t.Fatal("pickup never timed out")
	}
}

// The bot shouldn't spend time pushing against walls, doors or props: count
// the seconds it wants to move but stays within 6 px for 2 s, over a few
// games of each deck.
func TestAutopilotDoesNotGetStuck(t *testing.T) {
	stuck, played := 0, 0.0
	for lvl := range levels {
		for seed := int64(1); seed <= 5; seed++ {
			g, err := New()
			if err != nil {
				t.Fatal(err)
			}
			g.rng = rand.New(rand.NewSource(seed))
			g.level = lvl
			if err := g.reset(); err != nil {
				t.Fatal(err)
			}
			g.autopilot = newHumanAutopilot()
			var hist []geom.Vec
			for i := 0; i < 60*240 && g.level == lvl && g.state == statePlaying && !g.player.Dead; i++ {
				in := g.autopilot.input(g, Input{})
				if err := g.step(in); err != nil {
					t.Fatal(err)
				}
				played += tick
				if hist = append(hist, g.player.Pos); len(hist) > 120 {
					hist = hist[1:]
				}
				if i%60 == 0 && len(hist) == 120 && in.Move.Len() > 0.1 && hist[0].Dist(g.player.Pos) < 6 {
					stuck++
				}
			}
		}
	}
	t.Logf("stuck %d s of %.0f s played", stuck, played)
	if float64(stuck) > played*0.02 {
		t.Errorf("the bot was stuck %d s of %.0f s", stuck, played)
	}
}
