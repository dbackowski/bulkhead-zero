package game

import (
	"math"
	"slices"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/dbackowski/bulkhead-zero/internal/gfx"
	"github.com/dbackowski/bulkhead-zero/internal/world"
)

// Bullet is a player projectile. Ground is its position on the floor plane;
// it is drawn H pixels above that, which is what makes it look like it flies
// at hand height while collisions stay simple.
type Bullet struct {
	Ground geom.Vec
	Vel    geom.Vec
	H      float64
	Life   float64
	Dmg    int
	Rot    float64
	Anim   string  // fx animation
	Pierce int     // enemies it may pass through
	Blast  float64 // explodes with this radius instead of hitting
	Flame  bool    // flamethrower fire: no sparks, not stopped by shields, slows down
	Age    float64
	hit    []*Enemy // already pierced
}

type shotKind int

const (
	shotLaser shotKind = iota
	shotGlob
	shotSpine  // the Brood Mother's; flies straight like a laser
	shotMortar // the Warden's; arcs like a glob and explodes
)

// Shot is an enemy projectile: a straight laser, or an acid glob lobbed in an arc.
type Shot struct {
	Kind     shotKind
	Ground   geom.Vec
	Vel      geom.Vec // laser
	From, To geom.Vec // glob
	T, Dur   float64  // glob flight time
	H        float64
	Life     float64
	dead     bool
}

// FX is a one-shot animation in world space (positions are visual centres).
type FX struct {
	Anim *gfx.Anim
	Pos  geom.Vec
	Rot  float64
	T    float64
}

type pendingBlast struct {
	prop *world.Prop
	in   float64
}

const subSteps = 4 // bullets move ~7 px per tick: check collisions in smaller steps

func (g *Game) spawnFX(anim string, pos geom.Vec, rot float64) {
	g.fx = append(g.fx, &FX{Anim: g.lib.A(anim), Pos: pos, Rot: rot})
}

func (g *Game) updateBullets() {
	keep := g.bullets[:0]
	for _, b := range g.bullets {
		if g.stepBullet(b) {
			keep = append(keep, b)
		}
	}
	g.bullets = keep
}

// stepBullet advances one bullet and reports whether it is still flying.
func (g *Game) stepBullet(b *Bullet) bool {
	b.Life -= dt
	b.Age += dt
	if b.Flame {
		b.Vel = b.Vel.Scale(1 - 1.8*dt)
	}
	if b.Life <= 0 {
		if b.Blast > 0 {
			g.blast(b.Ground, b.Blast, b.Dmg)
		}
		return false
	}
	step := b.Vel.Scale(dt / subSteps)
	for i := 0; i < subSteps; i++ {
		b.Ground = b.Ground.Add(step)
		visual := b.Ground.Sub(geom.V(0, b.H))
		for _, e := range g.enemies {
			if e.State == stDead || e.hidden() || slices.Contains(b.hit, e) {
				continue
			}
			if b.Ground.Dist(e.Pos) < stats[e.Kind].radius {
				if b.Blast > 0 {
					g.blast(b.Ground, b.Blast, b.Dmg)
					return false
				}
				if e.blocks(b.Vel) && !b.Flame {
					g.spawnFX("fx.hit_spark", visual, 0)
					g.sound("shield_block", b.Ground)
					return false
				}
				g.damageEnemy(e, b.Dmg, b.Ground.Sub(b.Vel))
				switch {
				case b.Flame:
					e.ignite()
				case e.Kind == drone:
					g.spawnFX("fx.hit_spark", visual, 0)
					g.sound("hit_metal", b.Ground)
				default:
					g.spawnFX("fx.hit_spark", visual, 0)
					g.sound("hit_flesh", b.Ground)
				}
				if len(b.hit) >= b.Pierce {
					return false
				}
				b.hit = append(b.hit, e)
			}
		}
		if b.Blast > 0 && (g.lvl.WallAt(b.Ground) || g.lvl.PropAt(b.Ground) != nil) {
			g.blast(b.Ground.Sub(step), b.Blast, b.Dmg) // just short of what it hit
			return false
		}
		if b.Flame && (g.lvl.WallAt(b.Ground) || g.lvl.PropAt(b.Ground) != nil) {
			if pr := g.lvl.PropAt(b.Ground); pr != nil {
				g.damageProp(pr, b.Dmg) // crates burn, barrels go off
			}
			return false
		}
		if pr := g.lvl.PropAt(b.Ground); pr != nil {
			hit := "hit_wall"
			if pr.Explosive {
				hit = "hit_metal"
			}
			g.spawnFX("fx.hit_spark", visual, 0)
			g.sound(hit, b.Ground)
			g.damageProp(pr, b.Dmg)
			return false
		}
		if g.lvl.WallAt(b.Ground) {
			g.spawnFX("fx.hit_spark", visual, 0)
			g.sound("hit_wall", b.Ground)
			return false
		}
	}
	return true
}

func (g *Game) updateShots() {
	p := g.player
	keep := g.shots[:0]
	for _, s := range g.shots {
		switch s.Kind {
		case shotLaser, shotSpine:
			s.Life -= dt
			step := s.Vel.Scale(dt / subSteps)
			for i := 0; i < subSteps && !s.dead; i++ {
				s.Ground = s.Ground.Add(step)
				visual := s.Ground.Sub(geom.V(0, s.H))
				switch {
				case !p.Dead && s.Ground.Dist(p.Pos) < 8:
					g.hurtPlayer(1, s.Ground.Sub(s.Vel))
					g.spawnFX("fx.hit_spark", visual, 0)
					s.dead = true
				case g.lvl.WallAt(s.Ground) || g.lvl.PropAt(s.Ground) != nil:
					g.spawnFX("fx.hit_spark", visual, 0)
					s.dead = true
				}
			}
			if s.Life <= 0 {
				s.dead = true
			}
		case shotGlob, shotMortar:
			s.T += dt
			k := math.Min(s.T/s.Dur, 1)
			s.Ground = s.From.Lerp(s.To, k)
			s.H = 12*(1-k) + math.Sin(k*math.Pi)*34
			if k >= 1 && s.Kind == shotMortar {
				g.blast(s.To, mortarBlast, 3)
				s.dead = true
			} else if k >= 1 {
				g.spawnFX("fx.acid_splash", s.To, 0)
				g.sound("acid_splash", s.To)
				if !p.Dead && s.To.Dist(p.Pos) < 15 {
					g.hurtPlayer(2, s.To)
				}
				s.dead = true
			}
		}
		if !s.dead {
			keep = append(keep, s)
		}
	}
	g.shots = keep
}

// crateDrop is the chance a broken crate leaves a pickup.
const crateDrop = 0.35

// damageProp wears down a barrel or crate: at 0 HP barrels explode and
// crates break. Props without HP shrug it off.
func (g *Game) damageProp(pr *world.Prop, dmg int) {
	if !pr.Alive || pr.HP <= 0 {
		return
	}
	if pr.HP -= dmg; pr.HP > 0 {
		return
	}
	if pr.Explosive {
		g.explode(pr)
		return
	}
	g.lvl.RemoveProp(pr)
	g.wrecks = append(g.wrecks, geom.V(float64(pr.TX*world.T), float64(pr.TY*world.T)))
	g.spawnFX("fx.debris", pr.Center(), 0)
	g.sound("crate_break", pr.Center())
	if g.rng.Float64() < crateDrop {
		g.dropAt(pr.Feet())
	}
}

// explode blows up an explosive barrel.
func (g *Game) explode(pr *world.Prop) {
	if !pr.Alive {
		return
	}
	g.lvl.RemoveProp(pr)
	g.blast(pr.Feet(), 44, 4)
}

// blast is an explosion on the ground at c (barrels, bombers, grenades): it
// deals dmg to enemies within r, 2 to the player a little closer in, leaves a
// scorch mark, sets off barrels nearby and smashes crates.
func (g *Game) blast(c geom.Vec, r float64, dmg int) {
	v := c.Sub(geom.V(0, 9))
	g.spawnFX("fx.explosion", v, 0)
	g.spawnFX("fx.explosion", v.Add(geom.V(5, -6)), 0)
	g.scorches = append(g.scorches, v)
	g.shake(4, 0.25)
	g.sound("explosion", c)
	for _, e := range g.enemies {
		if e.State != stDead && e.Pos.Dist(c) < r {
			g.damageEnemy(e, dmg, c)
		}
	}
	if g.player.Pos.Dist(c) < r-6 {
		g.hurtPlayer(2, c)
	}
	for _, o := range g.lvl.Props {
		switch {
		case !o.Alive || o.Feet().Dist(c) >= r+4:
		case o.Explosive:
			g.blasts = append(g.blasts, pendingBlast{prop: o, in: 0.15})
		default:
			g.damageProp(o, dmg)
		}
	}
}

func (g *Game) updateBlasts() {
	var due []*world.Prop
	keep := g.blasts[:0]
	for _, b := range g.blasts {
		b.in -= dt
		if b.in <= 0 {
			due = append(due, b.prop)
			continue
		}
		keep = append(keep, b)
	}
	g.blasts = keep
	for _, pr := range due { // explode after the loop: explode() appends to g.blasts
		g.explode(pr)
	}
}

func (g *Game) updateFX() {
	keep := g.fx[:0]
	for _, f := range g.fx {
		f.T += dt
		if f.T < f.Anim.Duration() {
			keep = append(keep, f)
		}
	}
	g.fx = keep
}
