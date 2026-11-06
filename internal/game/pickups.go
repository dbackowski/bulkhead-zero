package game

import (
	"fmt"
	"math"
	"slices"
	"strings"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/dbackowski/bulkhead-zero/internal/gfx"
	"github.com/dbackowski/bulkhead-zero/internal/world"
	"github.com/hajimehoshi/ebiten/v2"
)

type pickupKind int

const (
	pickHealth pickupKind = iota // +pickupHP health
	pickAmmo                     // ammoBox of a full load of spare rounds for every unlocked gun
	pickWeapon                   // unlocks the gun in Slot; never times out
	pickHeavy                    // the heavy weapon in Slot, fully loaded; lasts until the deck ends
)

// Pickup lies on the floor at Pos (feet) until collected or it times out.
type Pickup struct {
	Kind pickupKind
	Slot int // the gun, for pickWeapon
	Pos  geom.Vec
	T    float64
}

const (
	pickupHP       = 2
	pickupLife     = 25.0 // seconds on the floor; it blinks for the last 4
	pickupRange    = 14.0 // px from the player's feet
	healthChance   = 0.4  // share of drops that are health rather than ammo
	ammoBox        = 3    // an ammo box gives 1/ammoBox of each gun's full spare load
	bruteGunChance = 0.1  // a brute drops the next gun instead of health or ammo
)

// Chance that each kind drops a heavy weapon instead of health or ammo (only
// while none is lying around). Clearing heavyWave always drops one.
var heavyChance = map[enemyKind]float64{brute: 0.06, shielder: 0.04, splitter: 0.02, nest: 0.1}

// Chance that each kind drops something when it dies. Bombers burst and the
// boss's brood dies with her, so neither drops anything; nor do crawlers from
// a nest or a splitter (a nest would be an endless supply).
var dropChance = map[enemyKind]float64{crawler: 0.12, spitter: 0.4, drone: 0.4, brute: 1,
	splitter: 0.3, shielder: 0.5, nest: 1}

func (g *Game) maybeDrop(e *Enemy) {
	gunLeft := g.unlocked+g.gunsOnFloor() < g.guns
	if gunLeft && (e == g.boss || e.Kind == brute && g.rng.Float64() < bruteGunChance) && g.dropGun(e.Pos) {
		return
	}
	if e.Parent == nil && g.rng.Float64() < heavyChance[e.Kind] && g.dropHeavy(e.Pos) {
		return
	}
	if e.Parent == nil && g.rng.Float64() < dropChance[e.Kind] {
		g.dropAt(e.Pos)
	}
}

// dropAt leaves a health or ammo pickup at (or next to) pos.
func (g *Game) dropAt(at geom.Vec) {
	if g.lvl.DoorAt(int(at.X/world.T), int(at.Y/world.T)) != nil {
		return // it would be shut inside the door
	}
	kind := pickAmmo
	if g.rng.Float64() < healthChance {
		kind = pickHealth
	}
	if pos, ok := g.reachableNear(at); ok {
		g.pickups = append(g.pickups, &Pickup{Kind: kind, Pos: pos})
	}
}

// dropGun leaves the next gun the player lacks at (or next to) at. Guns
// unlock in weaponOrder, so the ones on the floor are the next few slots and
// can only be picked up in that order. It reports false when every gun is
// already held or lying somewhere.
func (g *Game) dropGun(at geom.Vec) bool {
	slot := g.unlocked + g.gunsOnFloor()
	if slot >= g.guns {
		return false
	}
	pos, ok := g.reachableNear(at)
	if !ok {
		pos = g.player.Pos // never lose a gun behind a wall
	}
	g.pickups = append(g.pickups, &Pickup{Kind: pickWeapon, Slot: slot, Pos: pos})
	return true
}

// dropHeavy leaves a random heavy weapon at (or next to) at, unless one is
// lying around already.
func (g *Game) dropHeavy(at geom.Vec) bool {
	if g.guns == len(g.weapons) || slices.ContainsFunc(g.pickups, func(pk *Pickup) bool { return pk.Kind == pickHeavy }) {
		return false
	}
	pos, ok := g.reachableNear(at)
	if !ok {
		pos = g.player.Pos
	}
	slot := g.guns + g.rng.Intn(len(g.weapons)-g.guns)
	g.pickups = append(g.pickups, &Pickup{Kind: pickHeavy, Slot: slot, Pos: pos})
	return true
}

func (g *Game) gunsOnFloor() int {
	n := 0
	for _, pk := range g.pickups {
		if pk.Kind == pickWeapon {
			n++
		}
	}
	return n
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
		if pk.T > pickupLife && pk.Kind != pickWeapon && pk.Kind != pickHeavy {
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
	switch pk.Kind {
	case pickHealth:
		return p.HP < playerMaxHP
	case pickWeapon:
		return pk.Slot == g.unlocked
	case pickHeavy: // anything but the same one, still full
		return p.Heavy != pk.Slot || p.Mag[pk.Slot] < g.weapons[pk.Slot].Mag
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
	switch pk.Kind {
	case pickHealth:
		p.HP = min(p.HP+pickupHP, playerMaxHP)
		return true
	case pickWeapon:
		g.unlocked++
		g.say(fmt.Sprintf("NEW WEAPON: %s [%d]", strings.ToUpper(g.weapons[pk.Slot].Name), pk.Slot+1), 2.5)
		return true
	case pickHeavy: // replaces the one carried and goes straight into the hands
		p.Heavy, p.Mag[pk.Slot] = pk.Slot, g.weapons[pk.Slot].Mag
		p.Slot, p.ReloadT, p.Spin = pk.Slot, 0, 0
		g.say(fmt.Sprintf("%s [%d]\nNO RELOADS", strings.ToUpper(g.weapons[pk.Slot].Name), g.key(pk.Slot)), 2.5)
		return true
	}
	for i, w := range g.weapons[:g.unlocked] {
		if w.Ammo > 0 {
			p.Spare[i] = min(p.Spare[i]+w.Ammo/ammoBox, w.Ammo)
		}
	}
	return true
}

func (g *Game) drawPickup(dst *ebiten.Image, pk *Pickup, cam geom.Vec) {
	bob := math.Round(math.Sin(pk.T*4) * 1)
	if pk.Kind == pickWeapon || pk.Kind == pickHeavy { // the gun itself, glinting now and then
		flash := math.Max(0, math.Sin(pk.T*5)) * 0.6
		gfx.DrawAtFeet(dst, g.weapons[pk.Slot].Img, pk.Pos.X-cam.X, pk.Pos.Y-cam.Y-2+bob, gfx.SpriteOpts{Flash: flash})
		return
	}
	if pk.T > pickupLife-4 && int(pk.T*10)%2 == 0 {
		return // blinking: about to vanish
	}
	name := "fx.pickup_ammo"
	if pk.Kind == pickHealth {
		name = "fx.pickup_health"
	}
	gfx.DrawAtFeet(dst, g.lib.A(name).Frame(pk.T), pk.Pos.X-cam.X, pk.Pos.Y-cam.Y+bob, gfx.SpriteOpts{})
}
