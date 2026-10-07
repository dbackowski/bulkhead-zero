package game

import (
	"math"
	"math/rand"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/dbackowski/bulkhead-zero/internal/world"
)

type enemyKind int

const (
	crawler enemyKind = iota
	spitter
	drone
)

type enemyState int

const (
	stIdle enemyState = iota
	stChase
	stAttack
	stDead
)

// Per-kind tuning. Starting values from the art previews; tune freely.
type enemyStats struct {
	hp        int
	radius    float64 // ground hit radius for bullets
	aggro     float64 // wake-up range (needs line of sight)
	speed     float64
	cooldown  float64
	attackLen float64 // seconds the attack animation runs
	fireAt    float64 // when in the attack the hit/projectile happens
}

var stats = map[enemyKind]enemyStats{
	crawler: {hp: 3, radius: 9, aggro: 170, speed: 52, cooldown: 0.55, attackLen: 0.36, fireAt: 0.18},
	spitter: {hp: 6, radius: 10, aggro: 210, speed: 0, cooldown: 2.3, attackLen: 0.45, fireAt: 0.18},
	drone:   {hp: 5, radius: 9, aggro: 210, speed: 42, cooldown: 2.4, attackLen: 0.36, fireAt: 0.18},
}

// Enemy is any hostile. Pos is the feet (or, for the drone, its shadow).
type Enemy struct {
	Kind     enemyKind
	Pos      geom.Vec
	HP       int
	State    enemyState
	T        float64 // time in current state
	AnimT    float64
	CD       float64
	Flash    float64
	FaceLeft bool
	Aggro    bool
	Fired    bool
	Strafe   float64
	DeadT    float64
}

func newEnemy(s world.Spawn, rng *rand.Rand) *Enemy {
	k := map[byte]enemyKind{'C': crawler, 'S': spitter, 'R': drone}[s.Kind]
	return &Enemy{
		Kind:   k,
		Pos:    s.Pos,
		HP:     stats[k].hp,
		CD:     0.6 + rng.Float64()*1.2, // stagger first attacks
		AnimT:  rng.Float64() * 2,
		Strafe: []float64{-1, 1}[rng.Intn(2)],
	}
}

func (e *Enemy) setState(s enemyState) {
	e.State, e.T, e.Fired = s, 0, false
	if s == stAttack {
		e.AnimT = 0
	}
}

// eye is roughly where the enemy "sees" from, for line-of-sight checks.
func (e *Enemy) eye() geom.Vec { return e.Pos.Sub(geom.V(0, 6)) }

// alert wakes an enemy and its neighbours.
func (g *Game) alert(e *Enemy) {
	if e.Aggro {
		return
	}
	e.Aggro = true
	if e.State == stIdle {
		e.setState(stChase)
	}
	for _, o := range g.enemies {
		if o != e && !o.Aggro && o.State != stDead && o.Pos.Dist(e.Pos) < 110 && g.lvl.LineOfSight(o.eye(), e.eye()) {
			g.alert(o)
		}
	}
}

func (g *Game) updateEnemy(e *Enemy) {
	if e.Flash > 0 {
		e.Flash -= dt
	}
	if e.State == stDead {
		e.DeadT += dt
		return
	}
	st := stats[e.Kind]
	e.T += dt
	e.AnimT += dt
	e.CD -= dt
	p := g.player
	d := e.Pos.Dist(p.Pos)
	los := g.lvl.LineOfSight(e.eye(), p.Pos.Sub(geom.V(0, 6)))

	if !e.Aggro {
		if !p.Dead && d < st.aggro && los {
			g.alert(e)
		}
		return
	}
	if p.Dead || g.state != statePlaying {
		if e.State == stAttack && e.T > st.attackLen {
			e.setState(stIdle)
		}
		return
	}

	switch e.Kind {
	case crawler:
		g.updateCrawler(e, st, d, los)
	case spitter:
		g.updateSpitter(e, st, d, los)
	case drone:
		g.updateDrone(e, st, d, los)
	}
}

// chaseDir points straight at the player when visible and close, otherwise
// follows the flow field around walls.
func (g *Game) chaseDir(e *Enemy, los bool, d float64) geom.Vec {
	p := g.player.Pos
	if los && d < 72 {
		return p.Sub(e.Pos).Norm()
	}
	if next, ok := g.lvl.NextStep(g.flow, e.Pos); ok {
		return next.Sub(e.Pos).Norm()
	}
	return p.Sub(e.Pos).Norm()
}

func (g *Game) updateCrawler(e *Enemy, st enemyStats, d float64, los bool) {
	p := g.player
	e.FaceLeft = p.Pos.X < e.Pos.X
	switch e.State {
	case stAttack:
		if e.T < 0.27 {
			lunge := p.Pos.Sub(e.Pos).Norm().Scale(45 * dt)
			e.Pos = g.lvl.MoveAndSlide(e.Pos, lunge, 4, 3, false)
		}
		if !e.Fired && e.T >= st.fireAt {
			e.Fired = true
			if e.Pos.Dist(p.Pos) < 21 {
				g.hurtPlayer(1, e.Pos)
			}
		}
		if e.T >= st.attackLen {
			e.setState(stChase)
			e.CD = st.cooldown
		}
	default:
		dir := g.chaseDir(e, los, d)
		// keep crawlers from stacking on one pixel
		var sep geom.Vec
		for _, o := range g.enemies {
			if o == e || o.State == stDead || o.Kind == drone {
				continue
			}
			if od := o.Pos.Dist(e.Pos); od < 14 && od > 0 {
				sep = sep.Add(e.Pos.Sub(o.Pos).Scale((14 - od) / od))
			}
		}
		v := dir.Scale(st.speed).Add(sep.Scale(6))
		if d > 13 {
			e.Pos = g.lvl.MoveAndSlide(e.Pos, v.Scale(dt), 4, 3, false)
		}
		if d < 18 && e.CD <= 0 {
			e.setState(stAttack)
		}
	}
}

func (g *Game) updateSpitter(e *Enemy, st enemyStats, d float64, los bool) {
	p := g.player
	switch e.State {
	case stAttack:
		if !e.Fired && e.T >= st.fireAt {
			e.Fired = true
			target := p.Pos.Add(p.Vel.Scale(0.35)) // lead the target a little
			if target.Dist(e.Pos) > 240 {
				target = e.Pos.Add(target.Sub(e.Pos).Norm().Scale(240))
			}
			from := e.Pos.Sub(geom.V(0, 2))
			g.shots = append(g.shots, &Shot{
				Kind: shotGlob, From: from, To: target, Ground: from,
				Dur: 0.45 + from.Dist(target)/420, H: 12,
			})
		}
		if e.T >= st.attackLen {
			e.setState(stChase)
		}
	default:
		if e.CD <= 0 && los && d < 240 {
			e.setState(stAttack)
			e.CD = st.cooldown + g.rng.Float64()*0.6
		}
	}
}

func (g *Game) updateDrone(e *Enemy, st enemyStats, d float64, los bool) {
	p := g.player
	// movement happens in every state: hover at a comfortable range
	var dir geom.Vec
	switch {
	case !los:
		dir = g.chaseDir(e, false, d)
	case d > 140:
		dir = p.Pos.Sub(e.Pos).Norm()
	case d < 85:
		dir = e.Pos.Sub(p.Pos).Norm()
	default:
		to := p.Pos.Sub(e.Pos).Norm()
		dir = geom.V(-to.Y, to.X).Scale(e.Strafe)
	}
	speed := st.speed
	if e.State == stAttack {
		speed *= 0.3
	}
	before := e.Pos
	e.Pos = g.lvl.MoveAndSlide(e.Pos, dir.Scale(speed*dt), 4, 3, true)
	if e.Pos.Dist(before) < speed*dt*0.3 || g.rng.Float64() < dt/2.5 {
		e.Strafe = -e.Strafe
	}

	switch e.State {
	case stAttack:
		if !e.Fired && e.T >= st.fireAt {
			e.Fired = true
			from := e.Pos.Add(geom.V(0, 2))
			aim := p.Pos.Add(p.Vel.Scale(0.12)).Sub(from).Norm()
			g.shots = append(g.shots, &Shot{Kind: shotLaser, Ground: from, Vel: aim.Scale(215), H: 14, Life: 2})
		}
		if e.T >= st.attackLen {
			e.setState(stChase)
		}
	default:
		if e.CD <= 0 && los && d < 230 {
			e.setState(stAttack)
			e.CD = st.cooldown + g.rng.Float64()*0.6
		}
	}
}

func (g *Game) damageEnemy(e *Enemy, dmg int, from geom.Vec) {
	if e.State == stDead {
		return
	}
	e.HP -= dmg
	e.Flash = 0.07
	g.alert(e)
	if e.Kind == crawler {
		push := e.Pos.Sub(from).Norm().Scale(2.5)
		e.Pos = g.lvl.MoveAndSlide(e.Pos, push, 4, 3, false)
	}
	if e.HP <= 0 {
		e.setState(stDead)
		e.DeadT = 0
		if e.Kind == drone {
			g.spawnFX("fx.explosion", e.Pos.Sub(geom.V(0, 12)), 0)
			g.shake(2, 0.15)
		}
	}
}

// flying enemies draw a bit higher and sort by their shadow.
func (e *Enemy) height() float64 {
	if e.Kind == drone {
		return 12
	}
	return 7
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
