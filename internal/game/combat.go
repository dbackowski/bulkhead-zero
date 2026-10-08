package game

import (
	"math"

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
}

type shotKind int

const (
	shotLaser shotKind = iota
	shotGlob
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
	if b.Life <= 0 {
		return false
	}
	step := b.Vel.Scale(dt / subSteps)
	for i := 0; i < subSteps; i++ {
		b.Ground = b.Ground.Add(step)
		visual := b.Ground.Sub(geom.V(0, b.H))
		for _, e := range g.enemies {
			if e.State == stDead {
				continue
			}
			if b.Ground.Dist(e.Pos) < stats[e.Kind].radius {
				g.damageEnemy(e, b.Dmg, b.Ground.Sub(b.Vel))
				g.spawnFX("fx.hit_spark", visual, 0)
				g.sound("hit", b.Ground)
				return false
			}
		}
		if pr := g.lvl.PropAt(b.Ground); pr != nil {
			if pr.Explosive {
				pr.HP -= b.Dmg
				if pr.HP <= 0 {
					g.explode(pr)
				}
			}
			g.spawnFX("fx.hit_spark", visual, 0)
			g.sound("hit", b.Ground)
			return false
		}
		if g.lvl.WallAt(b.Ground) {
			g.spawnFX("fx.hit_spark", visual, 0)
			g.sound("hit", b.Ground)
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
		case shotLaser:
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
		case shotGlob:
			s.T += dt
			k := math.Min(s.T/s.Dur, 1)
			s.Ground = s.From.Lerp(s.To, k)
			s.H = 12*(1-k) + math.Sin(k*math.Pi)*34
			if k >= 1 {
				g.spawnFX("fx.acid_splash", s.To, 0)
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

// explode blows up an explosive barrel: area damage, scorch mark, chain reactions.
func (g *Game) explode(pr *world.Prop) {
	if !pr.Alive {
		return
	}
	g.lvl.RemoveProp(pr)
	c := pr.Feet()
	g.spawnFX("fx.explosion", pr.Center(), 0)
	g.spawnFX("fx.explosion", pr.Center().Add(geom.V(5, -6)), 0)
	g.scorches = append(g.scorches, pr.Center())
	g.shake(4, 0.25)
	for _, e := range g.enemies {
		if e.State != stDead && e.Pos.Dist(c) < 44 {
			g.damageEnemy(e, 4, c)
		}
	}
	if g.player.Pos.Dist(c) < 38 {
		g.hurtPlayer(2, c)
	}
	for _, o := range g.lvl.Props {
		if o.Alive && o.Explosive && o != pr && o.Feet().Dist(c) < 48 {
			g.blasts = append(g.blasts, pendingBlast{prop: o, in: 0.15})
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
