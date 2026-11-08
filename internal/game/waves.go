package game

import (
	"fmt"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/dbackowski/bulkhead-zero/internal/world"
)

// level is one deck of the station. Wave 1 is the enemies placed in the map;
// waves lists the ones after it, which come in through the outer doors, one
// letter per enemy (map letters: C crawler, R drone, E bomber, G brute,
// Y splitter, U shield-bearer; spitters and nests don't walk, so they stay
// map-only). The boss appears at the map's
// Q after the last wave; beating it moves on to the next deck.
type level struct {
	name     string
	rows     []string
	waves    []string
	boss     byte   // M Brood Mother, W Warden, O the Maw, K the Overmind
	unlocked int    // guns held at the latest when the deck starts (what the guaranteed drops give by then)
	music    string // suffix of its music loops: calm<music>, combat<music>, boss<music>
}

var levels = []level{
	{"DECK 1: HABITAT", world.Station, []string{"CCCCCCEE", "CCCYRRGEE", "CCCCCRRRGGEEE", "CCYYYRRGEE"}, 'M', 2, ""},
	{"DECK 2: REACTOR", world.Reactor, []string{"CCCRREE", "CCURRGEE", "CCCCRRRGGEE", "CCUURRYGEE"}, 'W', 4, "_reactor"},
	{"DECK 3: HYDROPONICS", world.Hydroponics, []string{"CCCCRREEG", "CCCCCRRGEE", "CCCCCRRRGGEEE", "CCYYUURRE"}, 'O', 6, "_hydroponics"},
	{"DECK 4: COMMAND", world.Command, []string{"CCYURRE", "CCCYURRGEE", "CCYYURRGEE", "CCYYUURRGEE"}, 'K', 6, "_command"},
}

var bossName = map[enemyKind]string{brood: "BROOD MOTHER", warden: "THE WARDEN", maw: "THE MAW", overmind: "THE OVERMIND"}

func (g *Game) deck() level { return levels[g.level] }

const (
	waveBreak = 4.0  // seconds between waves
	spawnGap  = 0.45 // between enemies of one wave
	waveHeal  = 3    // HP restored after each wave
	gunWave   = 2    // clearing this wave drops the next gun the player lacks (so does each boss)
	heavyWave = 4    // clearing this wave drops a heavy weapon for the last wave and the boss
	doorRange = 40.0 // px from a door's centre that opens it
	doorTime  = 0.25 // seconds to slide fully open or shut
)

func (g *Game) waveCount() int { return 1 + len(g.deck().waves) }

// updateWaves trickles in the current wave, then after a short break starts
// the next one, then the boss. Once the boss is dead the next deck starts, or
// after the last deck the station is clear.
func (g *Game) updateWaves() error {
	if g.player.Dead {
		return nil
	}
	if len(g.queue) > 0 {
		if g.spawnT -= dt; g.spawnT <= 0 {
			g.spawnAtDoor(g.queue[0])
			g.queue = g.queue[1:]
			g.spawnT = spawnGap
		}
		return nil
	}
	if g.boss != nil {
		switch {
		case g.boss.State != stDead || g.boss.DeadT < 2.5:
		case g.level+1 < len(levels):
			g.level++
			g.carried = g.unlocked + g.gunsOnFloor() // a gun left lying around comes along
			return g.reset()
		default:
			g.state, g.stateT = stateClear, 0
		}
		return nil
	}
	if g.aliveFoes() > 0 {
		return nil
	}
	if g.nextIn <= 0 { // the wave was just cleared
		g.waveCleared()
		g.nextIn = waveBreak
	}
	if g.nextIn -= dt; g.nextIn <= 0 {
		g.startWave()
	}
	return nil
}

func (g *Game) waveCleared() {
	p := g.player
	p.HP = min(p.HP+waveHeal, playerMaxHP)
	msg := fmt.Sprintf("WAVE %d CLEARED  +%d HP", g.wave, waveHeal)
	if g.wave == g.waveCount() { // full health for the boss
		p.HP = playerMaxHP
		msg = fmt.Sprintf("WAVE %d CLEARED  HEALTH RESTORED", g.wave)
	}
	if g.wave == gunWave && g.dropGun(g.lastKill) {
		msg += "\nA WEAPON DROPPED"
	}
	if g.wave == heavyWave && g.dropHeavy(g.lastKill) {
		msg += "\nA HEAVY WEAPON DROPPED"
	}
	g.say(msg, waveBreak)
	g.stinger("clear")
}

func (g *Game) startWave() {
	g.wave++
	if g.wave <= g.waveCount() {
		g.queue = []byte(g.deck().waves[g.wave-2])
		g.totalFoes = len(g.queue)
		g.say(fmt.Sprintf("WAVE %d", g.wave), 2)
		g.stinger("wave")
		return
	}
	g.boss = g.spawnHunter(g.deck().boss, g.lvl.Boss)
	g.boss.CD = 2
	g.totalFoes = 1
	g.shake(5, 0.6)
	g.sound(map[enemyKind]string{brood: "boss_roar", warden: "warden_alarm", maw: "burrow", overmind: "boss_roar"}[g.boss.Kind], g.boss.Pos)
	g.say("WARNING\n"+bossName[g.boss.Kind], 3)
	g.stinger("boss")
}

// spawnHunter adds an enemy that is already hunting the player.
func (g *Game) spawnHunter(kind byte, pos geom.Vec) *Enemy {
	e := newEnemy(world.Spawn{Kind: kind, Pos: pos}, g.rng)
	e.Aggro = true
	e.setState(stChase)
	g.enemies = append(g.enemies, e)
	return e
}

// spawnAtDoor puts an enemy inside a random outer door away from the player;
// the door slides open because the enemy is next to it.
func (g *Game) spawnAtDoor(kind byte) {
	var far []*world.Door
	var farthest *world.Door
	for _, d := range g.lvl.Doors {
		if d.Inner {
			continue
		}
		dist := d.Center().Dist(g.player.Pos)
		if dist > 180 {
			far = append(far, d)
		}
		if farthest == nil || dist > farthest.Center().Dist(g.player.Pos) {
			farthest = d
		}
	}
	d := farthest
	if len(far) > 0 {
		d = far[g.rng.Intn(len(far))]
	}
	if d == nil {
		return // a map without outer doors gets no waves
	}
	g.spawnHunter(kind, geom.V(float64(d.TX*world.T+12), float64(d.TY*world.T+16)))
}

// updateDoors slides doors open while someone is next to them. Outer doors
// only open for enemies: there is nothing behind them for the player.
func (g *Game) updateDoors() {
	for _, d := range g.lvl.Doors {
		c := d.Center()
		want := d.Inner && !g.player.Dead && g.player.Pos.Dist(c) < doorRange
		for _, e := range g.enemies {
			if want {
				break
			}
			want = e.State != stDead && e.Pos.Dist(c) < doorRange
		}
		open := d.Open - dt/doorTime
		if want {
			open = d.Open + dt/doorTime
		}
		open = clamp(open, 0, 1)
		if (d.Open == 0 && open > 0) || (d.Open == 1 && open < 1) {
			g.sound("door", c)
		}
		g.lvl.SetDoorOpen(d, open)
	}
}

func (g *Game) say(msg string, secs float64) { g.banner, g.bannerT = msg, secs }

// doorTile is the door frame to draw for how far open it is.
func doorTile(d *world.Door) string {
	switch {
	case d.Open >= world.DoorOpenAt:
		return "door_open"
	case d.Open > 0.15:
		return "door_half"
	}
	return "door_closed"
}
