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

// Difficulty guard-rail: the handicapped autopilot (shaky aim, slow reactions)
// should usually clear the deck, but not without getting hurt. Run with -v to
// see the numbers when tuning stats, HP or invulnerability.
func TestDifficulty(t *testing.T) {
	const rounds = 20
	clears, hpLeft := 0, 0
	for seed := int64(1); seed <= rounds; seed++ {
		g, err := New()
		if err != nil {
			t.Fatal(err)
		}
		g.rng = rand.New(rand.NewSource(seed))
		if err := g.reset(); err != nil {
			t.Fatal(err)
		}
		g.autopilot = newHumanAutopilot()
		for i := 0; i < 60*420 && g.state == statePlaying && !g.player.Dead; i++ {
			if err := g.Update(); err != nil {
				t.Fatal(err)
			}
		}
		if g.state == stateClear {
			clears++
			hpLeft += g.player.HP
		}
		t.Logf("seed %2d: state=%d wave=%d boss HP=%d time=%.0fs hp=%d", seed, g.state, g.wave, bossHealth(g), g.t, g.player.HP)
	}
	t.Logf("cleared %d/%d, avg HP left %.1f/%d", clears, rounds, float64(hpLeft)/float64(max(clears, 1)), playerMaxHP)
	if clears < rounds/2 {
		t.Errorf("too hard: cleared only %d/%d", clears, rounds)
	}
	if hpLeft > clears*playerMaxHP*3/4 {
		t.Errorf("too easy: avg HP left %.1f of %d", float64(hpLeft)/float64(clears), playerMaxHP)
	}
}

func bossHealth(g *Game) int {
	if g.boss == nil {
		return -1
	}
	return g.boss.HP
}

// Killing everything walks through every wave (unlocking weapons and healing
// between them) to the boss, and killing the boss clears the deck.
func TestWavesThenBoss(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	g.player.Invuln = 1e9
	step := func(secs float64) {
		for i := 0; i < int(secs*60); i++ {
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
	for w := 1; w <= g.waveCount(); w++ {
		if g.wave != w {
			t.Fatalf("expected wave %d, got %d", w, g.wave)
		}
		killAll()
		step(waveBreak + 0.1)
		if want := min(3+w, len(g.weapons)); g.unlocked != want {
			t.Fatalf("after wave %d: %d weapons unlocked, want %d", w, g.unlocked, want)
		}
		step(spawnGap * 20) // let the next wave come in through the doors
		if w < g.waveCount() && g.aliveFoes() != len(stationWaves[w-1]) {
			t.Fatalf("wave %d: %d enemies in, want %d", w+1, g.aliveFoes(), len(stationWaves[w-1]))
		}
	}
	if g.boss == nil || g.boss.State == stDead {
		t.Fatal("the boss should be out after the last wave")
	}
	killAll()
	step(3)
	if g.state != stateClear {
		t.Fatalf("expected the deck to be clear after the boss, state %d", g.state)
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
	p, w := g.player, g.weapons[0]
	hold := Input{SelectSlot: -1, Fire: true}
	for i := 0; i < 600 && p.ReloadT == 0; i++ {
		g.updatePlayer(hold)
	}
	if p.Mag[0] != 0 || p.ReloadT == 0 {
		t.Fatalf("expected an empty, reloading rifle: mag %d, reload %.2f", p.Mag[0], p.ReloadT)
	}
	for i := 0; i < int(w.ReloadMS/1000*60)+2; i++ {
		g.updatePlayer(Input{SelectSlot: -1})
	}
	if p.Mag[0] != w.Mag || p.Spare[0] != w.Ammo-w.Mag {
		t.Fatalf("after reload: mag %d spare %d, want %d and %d", p.Mag[0], p.Spare[0], w.Mag, w.Ammo-w.Mag)
	}
	p.Mag[0], p.Spare[0] = 0, 0
	g.updatePlayer(Input{SelectSlot: -1, Reload: true})
	if p.ReloadT != 0 || g.canReload(0) {
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
	p.Spare[0] = 0
	g.pickups = []*Pickup{{Kind: pickAmmo, Pos: p.Pos}}
	g.updatePickups()
	if p.Spare[0] != g.weapons[0].Ammo/2 {
		t.Fatalf("ammo pickup gave the rifle %d spare rounds", p.Spare[0])
	}
	g.pickups = []*Pickup{{Kind: pickAmmo, Pos: p.Pos.Add(geom.V(200, 0))}}
	for i := 0; i < int(pickupLife*60)+2; i++ {
		g.updatePickups()
	}
	if len(g.pickups) != 0 {
		t.Fatal("pickup never timed out")
	}
}
