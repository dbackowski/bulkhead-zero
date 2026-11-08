package game

import (
	"math"
	"math/rand"
	"testing"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/dbackowski/bulkhead-zero/internal/world"
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
	for i := 0; i < 60*1200 && g.state == statePlaying; i++ { // every deck in a row: allow ~20 minutes
		if err := g.Update(); err != nil {
			t.Fatal(err)
		}
	}
	t.Logf("after %.1fs on deck %d: state=%d wave=%d killed=%d hp=%d", g.t, g.level+1, g.state, g.wave, g.kills, g.player.HP)
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

// Shooting a crate wears it down until it breaks: the tile is free to walk
// through and a wreck is left on the floor. Consoles can't be broken.
func TestCrateBreaks(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	var crate, console *world.Prop
	for _, p := range g.lvl.Props {
		switch {
		case p.Name == "crate" && crate == nil:
			crate = p
		case p.Name == "console" && console == nil:
			console = p
		}
	}
	g.damageProp(crate, crate.HP-1)
	if !crate.Alive {
		t.Fatal("the crate broke before its HP ran out")
	}
	g.damageProp(crate, 1)
	if crate.Alive || g.lvl.RectBlocked(crate.Center(), 4, 3, false) || len(g.wrecks) != 1 {
		t.Fatalf("broken crate: alive=%v, still blocking=%v, wrecks=%d",
			crate.Alive, g.lvl.RectBlocked(crate.Center(), 4, 3, false), len(g.wrecks))
	}
	g.damageProp(console, 99)
	if !console.Alive {
		t.Fatal("a console broke")
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

// Killing everything walks through every wave of every deck (dropping guns
// and healing between waves) to its boss; beating a boss starts the
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
		for g.aliveFoes() > 0 { // again for what a splitter leaves behind
			for _, e := range g.enemies {
				if e.State != stDead {
					g.kill(e)
				}
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
			want := deck.unlocked
			if w >= gunWave {
				want++
			}
			if got := g.unlocked + g.gunsOnFloor(); got < min(want, g.guns) {
				t.Fatalf("deck %d after wave %d: %d weapons held or dropped, want %d", lvl+1, w, got, want)
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
	if p.Spare[r] != g.weapons[r].Ammo/ammoBox {
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

// Guns drop in order, never time out, can only be picked up in order, come
// along to the next deck if left lying around, and survive a retry but not a
// fresh start.
func TestGunDrops(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	p := g.player
	if g.unlocked != 2 {
		t.Fatalf("deck 1 starts with %d guns, want pistol and rifle", g.unlocked)
	}
	far := p.Pos.Add(geom.V(200, 0))
	g.dropGun(far)
	g.dropGun(far)
	if g.pickups[0].Slot != 2 || g.pickups[1].Slot != 3 {
		t.Fatalf("dropped slots %d and %d, want 2 and 3", g.pickups[0].Slot, g.pickups[1].Slot)
	}
	g.pickups[1].Pos = p.Pos
	for i := 0; i < int(pickupLife*60)+2; i++ {
		g.updatePickups()
	}
	if len(g.pickups) != 2 || g.unlocked != 2 {
		t.Fatalf("%d guns on the floor and %d held: the later one was taken first or one timed out", len(g.pickups), g.unlocked)
	}
	g.pickups[0].Pos = p.Pos
	g.updatePickups()
	g.updatePickups()
	if g.unlocked != 4 || len(g.pickups) != 0 {
		t.Fatalf("%d guns held, %d on the floor, want 4 and 0", g.unlocked, len(g.pickups))
	}

	g.unlocked = 5
	g.dropGun(far)
	g.carried = g.unlocked + g.gunsOnFloor()
	g.level = 1
	if err := g.reset(); err != nil {
		t.Fatal(err)
	}
	if g.unlocked != 6 {
		t.Fatalf("deck 2 starts with %d guns, want the 6 found", g.unlocked)
	}
	if g.reset(); g.unlocked != 6 {
		t.Fatalf("a retry starts with %d guns, want 6", g.unlocked)
	}
	g.state = stateClear
	if err := g.step(Input{Restart: true}); err != nil {
		t.Fatal(err)
	}
	if g.level != 0 || g.unlocked != 2 {
		t.Fatalf("a fresh start is on deck %d with %d guns", g.level+1, g.unlocked)
	}
}

// A shield-bearer's shield stops bullets from the front but not from behind,
// a splitter leaves small crawlers, and a nest keeps at most nestBrood of
// its own crawlers alive.
func TestNewEnemies(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	g.enemies = nil
	p := g.player.Pos
	s := g.spawnHunter('U', p.Add(geom.V(60, 0)))
	s.Face = math.Pi // shield toward the player, on its left
	shoot := func(from geom.Vec) {
		g.bullets = []*Bullet{{Ground: from, Vel: s.Pos.Sub(from).Norm().Scale(400), Life: 1, Dmg: 1, Anim: "fx.bullet"}}
		for i := 0; i < 30 && len(g.bullets) > 0; i++ {
			g.updateBullets()
		}
	}
	if shoot(s.Pos.Add(geom.V(-30, 0))); s.HP != stats[shielder].hp {
		t.Fatalf("a shot at the shield did damage: HP %d", s.HP)
	}
	if shoot(s.Pos.Add(geom.V(30, 0))); s.HP != stats[shielder].hp-1 {
		t.Fatalf("a shot in the back did no damage: HP %d", s.HP)
	}

	g.enemies = nil
	g.kill(g.spawnHunter('Y', p.Add(geom.V(60, 0))))
	if n := g.aliveFoes(); n != splitBrood {
		t.Fatalf("a dead splitter left %d crawlers, want %d", n, splitBrood)
	}

	g.enemies = nil
	n := g.spawnHunter('N', p.Add(geom.V(60, 0)))
	for i := 0; i < 60*20; i++ {
		g.player.Invuln = 1
		g.updateEnemy(n)
	}
	if got := g.aliveFoes() - 1; got != nestBrood {
		t.Fatalf("the nest has %d crawlers out, want %d", got, nestBrood)
	}
}

// A heavy weapon goes straight into the hands, can't be reloaded or refilled
// by ammo, the minigun only fires once spun up, an empty one is thrown away
// for the gun held before, and none comes along to the next deck.
func TestHeavyWeapons(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	g.enemies = nil
	p := g.player
	rifle, minigun := g.slotOf("rifle"), g.slotOf("minigun")
	g.pickups = []*Pickup{{Kind: pickHeavy, Slot: minigun, Pos: p.Pos}}
	g.updatePickups()
	if p.Heavy != minigun || p.Slot != minigun || g.key(minigun) != 7 {
		t.Fatalf("picked up the minigun: heavy %d, in hand %d", p.Heavy, p.Slot)
	}
	fire := Input{SelectSlot: -1, Fire: true, Mouse: geom.V(ScreenW/2+60, ScreenH/2)}
	for i := 0; i < 15; i++ { // a quarter second: still spinning up
		if err := g.step(fire); err != nil {
			t.Fatal(err)
		}
	}
	full := g.weapons[minigun].Mag
	if p.Mag[minigun] != full {
		t.Fatalf("the minigun fired before it spun up (%d of %d left)", p.Mag[minigun], full)
	}
	for i := 0; i < 30; i++ {
		if err := g.step(fire); err != nil {
			t.Fatal(err)
		}
	}
	left := p.Mag[minigun]
	if left == full {
		t.Fatal("the minigun never fired")
	}
	g.collect(&Pickup{Kind: pickAmmo})
	if g.startReload(); p.Mag[minigun] != left || p.ReloadT != 0 {
		t.Fatal("a heavy weapon was refilled or reloaded")
	}
	for i := 0; i < 60*20 && p.Heavy >= 0; i++ {
		if err := g.step(fire); err != nil {
			t.Fatal(err)
		}
	}
	if p.Heavy != -1 || p.Slot != rifle {
		t.Fatalf("after running dry: heavy %d, in hand %d, want none and the rifle", p.Heavy, p.Slot)
	}

	g.pickups = []*Pickup{{Kind: pickHeavy, Slot: g.slotOf("flamethrower"), Pos: p.Pos}}
	g.updatePickups()
	g.level = 1
	if err := g.reset(); err != nil {
		t.Fatal(err)
	}
	if g.player.Heavy != -1 {
		t.Fatal("a heavy weapon came along to the next deck")
	}
	if !g.dropHeavy(g.player.Pos) || g.dropHeavy(g.player.Pos) {
		t.Fatal("dropHeavy should leave one heavy weapon on the floor at a time")
	}
}

// Flames set an enemy on fire: it keeps taking damage for burnTime after the
// last hit, then stops.
func TestBurning(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	g.enemies = nil
	e := g.spawnHunter('G', g.player.Pos.Add(geom.V(200, 0)))
	from := e.Pos.Sub(geom.V(20, 0))
	g.bullets = []*Bullet{{Ground: from, Vel: geom.V(190, 0), Life: 0.38, Dmg: 1, Anim: "fx.flame", Pierce: 99, Flame: true}}
	for i := 0; i < 30 && len(g.bullets) > 0; i++ {
		g.updateBullets()
	}
	hit := e.HP
	if hit != stats[brute].hp-1 || e.Burn <= 0 {
		t.Fatalf("after one puff: HP %d, burning %.2f s", e.HP, e.Burn)
	}
	for i := 0; i < int(burnTime*60)+30; i++ {
		g.updateBurn(e)
	}
	ticks := int(math.Floor(burnTime / burnTick))
	if burnt := hit - e.HP; burnt != ticks || e.Burn > 0 {
		t.Fatalf("burning did %d damage and %.2f s are left, want %d and none", burnt, e.Burn, ticks)
	}
}

// The Overmind takes no damage while a node stands; with them all down its
// shield drops for overmindExpose seconds, then the nodes grow back.
func TestOvermindShield(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	g.level = 3
	if err := g.reset(); err != nil {
		t.Fatal(err)
	}
	g.enemies = nil
	b := g.spawnHunter('K', g.lvl.Boss)
	g.boss = b
	g.player.Pos = g.lvl.Start
	step := func(secs float64) {
		for i := 0; i < int(secs*60); i++ {
			g.player.Invuln = 1
			g.updateEnemy(b)
		}
	}
	nodes := func() (n []*Enemy) {
		for _, e := range g.enemies {
			if e.Parent == b && e.State != stDead {
				n = append(n, e)
			}
		}
		return n
	}
	step(0.1)
	if len(nodes()) != overmindNodes || !b.Shielded {
		t.Fatalf("at the start: %d nodes, shielded %v", len(nodes()), b.Shielded)
	}
	g.damageEnemy(b, 5, g.player.Pos)
	if b.HP != stats[overmind].hp {
		t.Fatal("the shield let damage through")
	}
	for _, n := range nodes() {
		g.kill(n)
	}
	step(0.1)
	if b.Shielded || b.Exposed <= 0 {
		t.Fatal("killing every node should drop the shield")
	}
	g.damageEnemy(b, 5, g.player.Pos)
	if b.HP != stats[overmind].hp-5 {
		t.Fatal("no damage with the shield down")
	}
	step(overmindExpose + 0.2)
	if len(nodes()) != overmindNodes || !b.Shielded {
		t.Fatalf("after the exposure: %d nodes, shielded %v", len(nodes()), b.Shielded)
	}
}
