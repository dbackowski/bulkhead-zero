package game

import (
	"fmt"
	"strings"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/dbackowski/bulkhead-zero/internal/world"
)

// Wave 1 is the enemies placed in the map. These come after it, in through
// the outer doors, one letter per enemy (map letters: C crawler, R drone,
// E bomber, G brute; spitters don't walk, so they stay map-only). The boss
// follows the last wave.
var stationWaves = []string{
	"CCCCCCEE",
	"CCCCRRGEE",
	"CCCCCRRRGGEEE",
}

const (
	waveBreak = 4.0  // seconds between waves
	spawnGap  = 0.45 // between enemies of one wave
	waveHeal  = 3    // HP restored after each wave
	doorRange = 40.0 // px from a door's centre that opens it
	doorTime  = 0.25 // seconds to slide fully open or shut
)

func (g *Game) waveCount() int { return 1 + len(stationWaves) }

// updateWaves trickles in the current wave, then after a short break starts
// the next one, then the boss. The deck is clear once the boss is dead.
func (g *Game) updateWaves() {
	if g.player.Dead {
		return
	}
	if len(g.queue) > 0 {
		if g.spawnT -= dt; g.spawnT <= 0 {
			g.spawnAtDoor(g.queue[0])
			g.queue = g.queue[1:]
			g.spawnT = spawnGap
		}
		return
	}
	if g.boss != nil {
		if g.boss.State == stDead && g.boss.DeadT > 2.5 {
			g.state, g.stateT = stateClear, 0
		}
		return
	}
	if g.aliveFoes() > 0 {
		return
	}
	if g.nextIn <= 0 { // the wave was just cleared
		g.waveCleared()
		g.nextIn = waveBreak
	}
	if g.nextIn -= dt; g.nextIn <= 0 {
		g.startWave()
	}
}

func (g *Game) waveCleared() {
	p := g.player
	p.HP = min(p.HP+waveHeal, playerMaxHP)
	msg := fmt.Sprintf("WAVE %d CLEARED  +%d HP", g.wave, waveHeal)
	if g.wave == g.waveCount() { // full health for the boss
		p.HP = playerMaxHP
		msg = fmt.Sprintf("WAVE %d CLEARED  HEALTH RESTORED", g.wave)
	}
	if g.unlocked < len(g.weapons) {
		g.unlocked++
		msg += fmt.Sprintf("\nNEW WEAPON: %s [%d]", strings.ToUpper(g.weapons[g.unlocked-1].Name), g.unlocked)
	}
	g.say(msg, waveBreak)
}

func (g *Game) startWave() {
	g.wave++
	if g.wave <= g.waveCount() {
		g.queue = []byte(stationWaves[g.wave-2])
		g.totalFoes = len(g.queue)
		g.say(fmt.Sprintf("WAVE %d", g.wave), 2)
		return
	}
	g.boss = g.spawnHunter('Q', g.lvl.Boss)
	g.boss.CD = 2
	g.totalFoes = 1
	g.shake(5, 0.6)
	g.sound("boss_roar", g.boss.Pos)
	g.say("WARNING\nBROOD MOTHER", 3)
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
