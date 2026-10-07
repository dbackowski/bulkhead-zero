package game

import (
	"math"
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

// Slot order for keys 1/2/3.
var weaponOrder = []string{"rifle", "shotgun", "pistol"}

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
}

func newPlayer(pos geom.Vec) *Player {
	return &Player{Pos: pos, HP: playerMaxHP, Facing: "right", HurtT: 99, SinceShot: 99}
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
	if in.SelectSlot >= 0 && in.SelectSlot < len(g.weapons) {
		p.Slot = in.SelectSlot
	}
	if in.Cycle != 0 {
		p.Slot = (p.Slot + in.Cycle + len(g.weapons)) % len(g.weapons)
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
	if trigger && p.FireCD <= 0 {
		g.fire(w)
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
		life := 0.9
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
		})
	}
	g.spawnFX("fx.muzzle_flash", muzzle, 0)
	switch w.Name {
	case "shotgun":
		g.shake(2.5, 0.12)
	case "pistol":
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
	}
}
