package game

import (
	"math"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/dbackowski/bulkhead-zero/internal/gfx"
	"github.com/dbackowski/bulkhead-zero/internal/world"
	"github.com/hajimehoshi/ebiten/v2"
)

type pickupKind int

const (
	pickHealth pickupKind = iota // +pickupHP health
	pickAmmo                     // half a full load of spare rounds for every unlocked gun
)

// Pickup lies on the floor at Pos (feet) until collected or it times out.
type Pickup struct {
	Kind pickupKind
	Pos  geom.Vec
	T    float64
}

const (
	pickupHP     = 2
	pickupLife   = 25.0 // seconds on the floor; it blinks for the last 4
	pickupRange  = 14.0 // px from the player's feet
	healthChance = 0.4  // share of drops that are health rather than ammo
)

// Chance that each kind drops something when it dies. Bombers burst and the
// boss's brood dies with her, so neither drops anything.
var dropChance = map[enemyKind]float64{crawler: 0.12, spitter: 0.4, drone: 0.4, brute: 1}

func (g *Game) maybeDrop(e *Enemy) {
	if g.rng.Float64() >= dropChance[e.Kind] {
		return
	}
	if g.lvl.DoorAt(int(e.Pos.X/world.T), int(e.Pos.Y/world.T)) != nil {
		return // it would be shut inside the door
	}
	kind := pickAmmo
	if g.rng.Float64() < healthChance {
		kind = pickHealth
	}
	if pos, ok := g.reachableNear(e.Pos); ok {
		g.pickups = append(g.pickups, &Pickup{Kind: kind, Pos: pos})
	}
}

// reachableNear finds a spot at or next to pos where the player can stand (a
// drone can die above a crate or against a wall), or reports false.
func (g *Game) reachableNear(pos geom.Vec) (geom.Vec, bool) {
	for _, r := range []float64{0, 12, 24} {
		for i := 0; i < 8; i++ {
			p := pos.Add(geom.FromAngle(float64(i) * math.Pi / 4).Scale(r))
			if !g.lvl.RectBlocked(p, playerHW, playerHH, false) && g.lvl.DoorAt(int(p.X/world.T), int(p.Y/world.T)) == nil {
				return p, true
			}
			if r == 0 {
				break
			}
		}
	}
	return pos, false
}

func (g *Game) updatePickups() {
	p := g.player
	keep := g.pickups[:0]
	for _, pk := range g.pickups {
		pk.T += dt
		if pk.T > pickupLife {
			continue
		}
		if !p.Dead && pk.Pos.Dist(p.Pos) < pickupRange && g.collect(pk) {
			g.sound("pickup", pk.Pos)
			continue
		}
		keep = append(keep, pk)
	}
	g.pickups = keep
}

// wants reports whether the player has a use for a pickup; one they don't
// stays on the floor.
func (g *Game) wants(pk *Pickup) bool {
	p := g.player
	if pk.Kind == pickHealth {
		return p.HP < playerMaxHP
	}
	for i, w := range g.weapons[:g.unlocked] {
		if w.Ammo > 0 && p.Spare[i] < w.Ammo {
			return true
		}
	}
	return false
}

// collect applies a pickup, or reports false when the player has no use for it.
func (g *Game) collect(pk *Pickup) bool {
	if !g.wants(pk) {
		return false
	}
	p := g.player
	if pk.Kind == pickHealth {
		p.HP = min(p.HP+pickupHP, playerMaxHP)
		return true
	}
	for i, w := range g.weapons[:g.unlocked] {
		if w.Ammo > 0 {
			p.Spare[i] = min(p.Spare[i]+w.Ammo/2, w.Ammo)
		}
	}
	return true
}

func (g *Game) drawPickup(dst *ebiten.Image, pk *Pickup, cam geom.Vec) {
	if pk.T > pickupLife-4 && int(pk.T*10)%2 == 0 {
		return // blinking: about to vanish
	}
	name := "fx.pickup_ammo"
	if pk.Kind == pickHealth {
		name = "fx.pickup_health"
	}
	bob := math.Round(math.Sin(pk.T*4) * 1)
	gfx.DrawAtFeet(dst, g.lib.A(name).Frame(pk.T), pk.Pos.X-cam.X, pk.Pos.Y-cam.Y+bob, gfx.SpriteOpts{})
}
