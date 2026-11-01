package game

import (
	"math"
	"slices"
	"strings"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/dbackowski/bulkhead-zero/internal/gfx"
	"github.com/hajimehoshi/ebiten/v2"
)

const (
	playerSpeed  = 85.0
	playerHW     = 4.0 // collision half-size around the feet
	playerHH     = 3.0
	playerMaxHP  = 8
	hurtAnimTime = 0.16
	invulnTime   = 0.45
)

// Slot order for keys 1-6. The first three are there from the start; each
// cleared wave unlocks the next. Code that needs a particular gun looks it up
// with slotOf rather than by number.
var weaponOrder = []string{"pistol", "rifle", "shotgun", "smg", "plasma", "launcher"}

const startWeapon = "rifle" // the gun in hand when a deck starts

// slotOf is the slot holding the named weapon, or -1.
func (g *Game) slotOf(name string) int {
	return slices.IndexFunc(g.weapons, func(w *gfx.Weapon) bool { return w.Name == name })
}

// Player is the marine. Pos is the feet position.
type Player struct {
	Pos       geom.Vec
	Vel       geom.Vec
	HP        int
	Aim       float64
	Facing    string
	Moving    bool
	AnimT     float64
	Slot      int
	FireCD    float64
	SinceShot float64
	HurtT     float64
	Invuln    float64
	Dead      bool
	DeadT     float64
	Mag       []int   // rounds loaded, per weapon slot
	Spare     []int   // spare rounds, per slot (unused for unlimited weapons)
	ReloadT   float64 // time left on the current reload; 0 = not reloading
}

func newPlayer(pos geom.Vec, weapons []*gfx.Weapon) *Player {
	p := &Player{Pos: pos, HP: playerMaxHP, Facing: "right", HurtT: 99, SinceShot: 99}
	for _, w := range weapons {
		p.Mag = append(p.Mag, w.Mag)
		p.Spare = append(p.Spare, w.Ammo)
	}
	return p
}

// canReload reports whether slot i has a partly empty magazine and rounds to fill it.
func (g *Game) canReload(i int) bool {
	w, p := g.weapons[i], g.player
	return p.Mag[i] < w.Mag && (w.Ammo == 0 || p.Spare[i] > 0)
}

func (g *Game) startReload() {
	p := g.player
	if p.ReloadT > 0 || !g.canReload(p.Slot) {
		return
	}
	p.ReloadT = g.weapons[p.Slot].ReloadMS / 1000
	g.sound("reload", p.Pos)
}

func (g *Game) finishReload() {
	p := g.player
	w := g.weapons[p.Slot]
	n := w.Mag - p.Mag[p.Slot]
	if w.Ammo > 0 {
		n = min(n, p.Spare[p.Slot])
		p.Spare[p.Slot] -= n
	}
	p.Mag[p.Slot] += n
}

// facingFor maps an aim angle to one of the 8 facings (45° sectors).
func facingFor(facings []string, aim float64) string {
	i := int(math.Round(aim/(math.Pi/4))) % 8
	if i < 0 {
		i += 8
	}
	return facings[i]
}

// bodyFrame returns the current body image, whether to mirror it, and the bob offset.
func (g *Game) bodyFrame() (*ebiten.Image, bool, float64) {
	p := g.player
	if p.Dead {
		return g.lib.A("player.death").Frame(p.DeadT), false, 0
	}
	base, flip := p.Facing, false
	if strings.HasSuffix(base, "left") {
		base, flip = strings.Replace(base, "left", "right", 1), true
	}
	if p.HurtT < hurtAnimTime {
		return g.lib.A("player.hurt_" + base).Frame(p.HurtT), flip, 0
	}
	name := "player.idle_" + base
	if p.Moving {
		name = "player.walk_" + base
	}
	a := g.lib.A(name)
	i := a.Index(p.AnimT)
	return a.Frames[i], flip, a.BobAt(i)
}

// handPos is where the gun pivot sits in world space.
func (g *Game) handPos() geom.Vec {
	p := g.player
	_, _, bob := g.bodyFrame()
	a := g.lib.HandAnchor[p.Facing]
	h := p.Pos.Sub(geom.V(gfx.FeetX, gfx.FeetY)).Add(a).Add(geom.V(0, bob))
	return geom.V(math.Round(h.X), math.Round(h.Y))
}

// muzzlePos is the world position of the current weapon's muzzle.
func (g *Game) muzzlePos(hand geom.Vec) geom.Vec {
	w := g.weapons[g.player.Slot]
	off := w.Muzzle.Sub(w.Pivot)
	if math.Cos(g.player.Aim) < 0 {
		off.Y = -off.Y // the gun is flipped when aiming left
	}
	return hand.Add(off.Rotate(g.player.Aim))
}

func (g *Game) updatePlayer(in Input) {
	p := g.player
	p.HurtT += dt
	p.SinceShot += dt
	if p.Invuln > 0 {
		p.Invuln -= dt
	}
	if p.FireCD > 0 {
		p.FireCD -= dt
	}
	if p.Dead {
		p.DeadT += dt
		return
	}
	if g.state != statePlaying {
		return
	}

	// weapons
	slot := p.Slot
	if in.SelectSlot >= 0 && in.SelectSlot < g.unlocked {
		slot = in.SelectSlot
	}
	if in.Cycle != 0 {
		slot = (slot + in.Cycle + g.unlocked) % g.unlocked
	}
	if slot != p.Slot {
		p.Slot, p.ReloadT = slot, 0 // switching drops the reload
	}
	if p.ReloadT > 0 {
		if p.ReloadT -= dt; p.ReloadT <= 0 {
			p.ReloadT = 0
			g.finishReload()
		}
	}
	if in.Reload || p.Mag[p.Slot] == 0 { // an empty gun reloads by itself, even after switching back to it
		g.startReload()
	}

	// movement
	move := in.Move.Norm()
	p.Moving = move.Len() > 0
	p.Vel = move.Scale(playerSpeed)
	if p.Moving {
		p.AnimT += dt
		p.Pos = g.lvl.MoveAndSlide(p.Pos, p.Vel.Scale(dt), playerHW, playerHH, false)
	} else {
		p.AnimT += dt * 0.8
	}

	// aim: from the hand to the cursor, then pick the body facing
	target := g.mouseWorld(in.Mouse)
	hand := g.handPos()
	if target.Dist(hand) > 2 {
		p.Aim = target.Sub(hand).Angle()
	}
	p.Facing = facingFor(g.lib.Facings, p.Aim)

	// fire
	w := g.weapons[p.Slot]
	trigger := in.Fire
	if w.SemiAuto {
		trigger = in.FirePressed
	}
	switch {
	case !trigger || p.FireCD > 0 || p.ReloadT > 0:
	case p.Mag[p.Slot] > 0:
		g.fire(w)
		p.Mag[p.Slot]--
	default: // empty and nothing to reload with
		g.sound("dry_fire", p.Pos)
		p.FireCD = 0.3
	}
}

func (g *Game) fire(w *gfx.Weapon) {
	p := g.player
	p.FireCD = w.FireMS / 1000
	p.SinceShot = 0
	hand := g.handPos()
	muzzle := g.muzzlePos(hand)
	height := p.Pos.Y - hand.Y // bullets fly at hand height above the ground
	spread := w.SpreadDeg * math.Pi / 180
	for i := 0; i < w.Pellets; i++ {
		a := p.Aim
		speed := w.BulletSpeed
		life := 0.9 // the launcher's grenade bursts at the end of this
		if w.Pellets > 1 {
			a += -spread + 2*spread*float64(i)/float64(w.Pellets-1) + (g.rng.Float64()*2-1)*0.04
			speed *= 0.9 + g.rng.Float64()*0.2
			life = 0.32
		} else {
			a += (g.rng.Float64()*2 - 1) * spread
		}
		g.bullets = append(g.bullets, &Bullet{
			Ground: muzzle.Add(geom.V(0, height)),
			Vel:    geom.FromAngle(a).Scale(speed),
			H:      height,
			Life:   life,
			Dmg:    w.Damage,
			Rot:    a,
			Anim:   w.Projectile,
			Pierce: w.Pierce,
			Blast:  w.Blast,
		})
	}
	g.spawnFX("fx.muzzle_flash", muzzle, 0)
	g.sound(w.Name, muzzle)
	switch w.Name {
	case "shotgun", "launcher":
		g.shake(2.5, 0.12)
	case "pistol", "plasma":
		g.shake(1, 0.06)
	}
}

func (g *Game) hurtPlayer(dmg int, from geom.Vec) {
	p := g.player
	if p.Dead || p.Invuln > 0 || g.state != statePlaying {
		return
	}
	p.HP -= dmg
	p.HurtT = 0
	p.Invuln = invulnTime
	g.shake(3, 0.2)
	push := p.Pos.Sub(from).Norm().Scale(6)
	p.Pos = g.lvl.MoveAndSlide(p.Pos, push, playerHW, playerHH, false)
	if p.HP <= 0 {
		p.HP = 0
		p.Dead = true
		p.DeadT = 0
		g.sound("player_death", p.Pos)
		return
	}
	g.sound("hurt", p.Pos)
}
