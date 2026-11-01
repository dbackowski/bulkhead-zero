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
	bomber // runs at the player and bursts
	brute  // slow and tough, charges
	brood  // boss of deck 1, the Brood Mother
	warden // boss of deck 2, a security mech
)

// Map letters for each kind (see world/level.go), also used by the wave
// lists; the bosses' letters are only used by the level list.
var kindFor = map[byte]enemyKind{'C': crawler, 'S': spitter, 'R': drone, 'E': bomber, 'G': brute, 'M': brood, 'W': warden}

func (e *Enemy) isBoss() bool { return e.Kind == brood || e.Kind == warden }

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
	crawler: {hp: 4, radius: 9, aggro: 170, speed: 70, cooldown: 0.55, attackLen: 0.36, fireAt: 0.18},
	spitter: {hp: 6, radius: 10, aggro: 210, speed: 0, cooldown: 2.0, attackLen: 0.45, fireAt: 0.18},
	drone:   {hp: 7, radius: 9, aggro: 210, speed: 42, cooldown: 2.0, attackLen: 0.36, fireAt: 0.18},
	// bomber: attackLen is the fuse once it is next to the player
	bomber: {hp: 2, radius: 8, aggro: 170, speed: 62, attackLen: 0.5},
	// brute: winds up until fireAt, then charges until attackLen
	brute:  {hp: 20, radius: 11, aggro: 190, speed: 30, cooldown: 2.4, attackLen: 1.15, fireAt: 0.55},
	brood:  {hp: 120, radius: 17, aggro: 999, speed: 24, cooldown: 1.6, attackLen: 0.7, fireAt: 0.35},
	warden: {hp: 80, radius: 16, aggro: 999, speed: 22, cooldown: 2.4, attackLen: 0.8, fireAt: 0.3},
}

// Enemy shots: how far ahead they aim (seconds of player movement) and laser speed (px/s).
const (
	globLead   = 0.7
	laserLead  = 0.25
	laserSpeed = 260

	bomberBlast = 40.0  // px
	bruteCharge = 200.0 // px/s
	spineSpeed  = 150.0 // the Brood Mother's spines are slow enough to weave through
	mortarBlast = 30.0  // px, the Warden's shells
	fanSpeed    = 190.0 // the Warden's laser fan: slower than a drone's shot
	fanGap      = 0.24  // radians between its bolts
)

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
	Dir      geom.Vec // brute: charge direction
	Pattern  int      // boss: which attack comes next
}

func newEnemy(s world.Spawn, rng *rand.Rand) *Enemy {
	k := kindFor[s.Kind]
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
	case bomber:
		g.updateBomber(e, st, d, los)
	case brute:
		g.updateBrute(e, st, d, los)
	case brood, warden:
		g.updateBoss(e, st, d, los)
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
		g.walk(e, st.speed, d, los)
		if d < 18 && e.CD <= 0 {
			e.setState(stAttack)
		}
	}
}

// walk moves a ground enemy toward the player, keeping it from stacking on others.
func (g *Game) walk(e *Enemy, speed, d float64, los bool) {
	dir := g.chaseDir(e, los, d)
	var sep geom.Vec
	for _, o := range g.enemies {
		if o == e || o.State == stDead || o.Kind == drone {
			continue
		}
		if od := o.Pos.Dist(e.Pos); od < 14 && od > 0 {
			sep = sep.Add(e.Pos.Sub(o.Pos).Scale((14 - od) / od))
		}
	}
	v := dir.Scale(speed).Add(sep.Scale(6))
	if d > 13 {
		e.Pos = g.lvl.MoveAndSlide(e.Pos, v.Scale(dt), 4, 3, false)
	}
}

func (g *Game) updateBomber(e *Enemy, st enemyStats, d float64, los bool) {
	e.FaceLeft = g.player.Pos.X < e.Pos.X
	switch e.State {
	case stAttack:
		if e.T >= st.attackLen {
			g.kill(e)
		}
	default:
		g.walk(e, st.speed, d, los)
		if d < 24 {
			e.setState(stAttack)
		}
	}
}

func (g *Game) updateBrute(e *Enemy, st enemyStats, d float64, los bool) {
	p := g.player
	switch {
	case e.State != stAttack:
		e.FaceLeft = p.Pos.X < e.Pos.X
		g.walk(e, st.speed, d, los)
		if e.CD <= 0 && los && d < 140 {
			e.setState(stAttack)
		}
	case e.T < st.fireAt: // wind-up: keep turning toward the player
		e.FaceLeft = p.Pos.X < e.Pos.X
		e.Dir = p.Pos.Add(p.Vel.Scale(0.2)).Sub(e.Pos).Norm()
	case e.T < st.attackLen: // charge in a straight line
		before := e.Pos
		e.Pos = g.lvl.MoveAndSlide(e.Pos, e.Dir.Scale(bruteCharge*dt), 4, 3, false)
		if !e.Fired && e.Pos.Dist(p.Pos) < 16 {
			e.Fired = true
			g.hurtPlayer(2, e.Pos)
		}
		if e.Pos.Dist(before) < bruteCharge*dt*0.5 { // ran into something
			e.T = st.attackLen
			g.shake(1.5, 0.1)
		}
	default:
		e.setState(stChase)
		e.CD = st.cooldown
	}
}

// updateBoss runs either boss. The Brood Mother crawls after the player and
// cycles a ring of spines, an acid barrage and summoning her brood. The
// Warden keeps its distance and cycles a fan of lasers, mortar shells and
// deploying drones. Below half health both are faster and every attack is bigger.
func (g *Game) updateBoss(e *Enemy, st enemyStats, d float64, los bool) {
	enraged := e.HP*2 < st.hp
	if d < 22 {
		g.hurtPlayer(2, e.Pos) // touching her hurts
	}
	switch e.State {
	case stAttack:
		if !e.Fired && e.T >= st.fireAt {
			e.Fired = true
			if e.Kind == warden {
				g.wardenAttack(e, enraged)
			} else {
				g.broodAttack(e, enraged)
			}
		}
		if e.T >= st.attackLen {
			e.setState(stChase)
			e.CD = st.cooldown
			if enraged {
				e.CD *= 0.65
			}
		}
	default:
		speed := st.speed
		if enraged {
			speed *= 1.4
		}
		dir := g.chaseDir(e, los, d)
		switch {
		case e.Kind == warden && los && d < 60:
			dir = e.Pos.Sub(g.player.Pos).Norm() // back off
		case e.Kind == warden && los && d < 100, e.Kind == brood && d <= 40:
			dir = geom.Vec{} // close enough
		}
		e.Pos = g.lvl.MoveAndSlide(e.Pos, dir.Scale(speed*dt), 8, 5, false)
		if e.CD <= 0 && los {
			e.setState(stAttack)
		}
	}
}

func (g *Game) wardenAttack(e *Enemy, enraged bool) {
	p := g.player
	from := e.Pos.Sub(geom.V(0, 4))
	pattern := e.Pattern % 3
	e.Pattern++
	if pattern == 2 && g.aliveFoes() > 6 {
		pattern = 0 // enough drones out already
	}
	switch pattern {
	case 0: // a fan of lasers at the player
		n := 3
		if enraged {
			n = 5
		}
		aim := p.Pos.Sub(from).Angle()
		for i := 0; i < n; i++ {
			a := aim + (float64(i)-float64(n-1)/2)*fanGap
			g.shots = append(g.shots, &Shot{Kind: shotLaser, Ground: from, Vel: geom.FromAngle(a).Scale(fanSpeed), H: 22, Life: 2.5})
		}
		g.sound("laser", from)
	case 1: // mortar shells that burst where they land (marked on the floor)
		n := 2
		if enraged {
			n = 4
		}
		for i := 0; i < n; i++ {
			to := p.Pos.Add(p.Vel.Scale(globLead)).Add(geom.FromAngle(g.rng.Float64() * 2 * math.Pi).Scale(float64(i) * 26))
			g.shots = append(g.shots, &Shot{Kind: shotMortar, From: from, To: to, Ground: from,
				Dur: 0.8 + from.Dist(to)/450 + float64(i)*0.12, H: 22})
		}
		g.sound("launcher", from)
	case 2: // deploy drones
		n := 2
		if enraged {
			n = 3
		}
		for i := 0; i < n; i++ {
			pos := e.Pos.Add(geom.V(float64(i*2-n+1)*14, -6))
			if g.lvl.RectBlocked(pos, 4, 3, true) {
				pos = e.Pos
			}
			g.spawnHunter('R', pos)
		}
		g.sound("warden_alarm", from)
	}
}

func (g *Game) broodAttack(e *Enemy, enraged bool) {
	p := g.player
	from := e.Pos.Sub(geom.V(0, 4))
	pattern := e.Pattern % 3
	e.Pattern++
	if pattern == 2 && g.aliveFoes() > 7 {
		pattern = 0 // enough brood out already
	}
	switch pattern {
	case 0: // ring of spines
		n := 12
		if enraged {
			n = 18
		}
		off := g.rng.Float64()
		for i := 0; i < n; i++ {
			a := (float64(i) + off) / float64(n) * 2 * math.Pi
			g.shots = append(g.shots, &Shot{Kind: shotSpine, Ground: from, Vel: geom.FromAngle(a).Scale(spineSpeed), H: 16, Life: 3})
		}
		g.sound("spine", from)
	case 1: // acid barrage on and around the player
		n := 3
		if enraged {
			n = 5
		}
		for i := 0; i < n; i++ {
			to := p.Pos.Add(p.Vel.Scale(globLead)).Add(geom.FromAngle(g.rng.Float64() * 2 * math.Pi).Scale(float64(i) * 20))
			g.shots = append(g.shots, &Shot{Kind: shotGlob, From: from, To: to, Ground: from,
				Dur: 0.5 + from.Dist(to)/420 + float64(i)*0.08, H: 12})
		}
		g.sound("spit", from)
	case 2: // brood
		kinds := "CCC"
		if enraged {
			kinds = "CCEC"
		}
		for i := range kinds {
			pos := e.Pos.Add(geom.FromAngle(float64(i)*2*math.Pi/float64(len(kinds)) + g.rng.Float64()).Scale(24))
			if g.lvl.RectBlocked(pos, 4, 3, false) {
				pos = e.Pos
			}
			g.spawnHunter(kinds[i], pos)
		}
		g.sound("boss_roar", from)
	}
}

func (g *Game) updateSpitter(e *Enemy, st enemyStats, d float64, los bool) {
	p := g.player
	switch e.State {
	case stAttack:
		if !e.Fired && e.T >= st.fireAt {
			e.Fired = true
			target := p.Pos.Add(p.Vel.Scale(globLead))
			if target.Dist(e.Pos) > 240 {
				target = e.Pos.Add(target.Sub(e.Pos).Norm().Scale(240))
			}
			from := e.Pos.Sub(geom.V(0, 2))
			g.sound("spit", from)
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
			aim := p.Pos.Add(p.Vel.Scale(laserLead)).Sub(from).Norm()
			g.shots = append(g.shots, &Shot{Kind: shotLaser, Ground: from, Vel: aim.Scale(laserSpeed), H: 14, Life: 2})
			g.sound("laser", from)
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
	if e.Kind == crawler || e.Kind == bomber {
		push := e.Pos.Sub(from).Norm().Scale(2.5)
		e.Pos = g.lvl.MoveAndSlide(e.Pos, push, 4, 3, false)
	}
	if e.HP <= 0 {
		g.kill(e)
	}
}

func (g *Game) kill(e *Enemy) {
	e.setState(stDead)
	e.DeadT = 0
	g.kills++
	g.sound(deathSound[e.Kind], e.Pos)
	g.maybeDrop(e)
	switch e.Kind {
	case drone:
		g.spawnFX("fx.explosion", e.Pos.Sub(geom.V(0, 12)), 0)
		g.shake(2, 0.15)
	case bomber:
		g.blast(e.Pos, bomberBlast, 3)
	case brood, warden:
		g.say(bossName[e.Kind]+" DOWN", 2.5)
		for i := 0; i < 6; i++ {
			off := geom.FromAngle(float64(i) * 1.1).Scale(float64(6 + i*3))
			g.fx = append(g.fx, &FX{Anim: g.lib.A("fx.explosion"), Pos: e.Pos.Sub(geom.V(0, 18)).Add(off), T: -0.12 * float64(i)})
		}
		g.shake(6, 0.8)
		g.shots = nil
		for _, o := range g.enemies { // its minions die with it
			if o.State != stDead {
				o.setState(stDead)
				o.DeadT = 0
			}
		}
	}
}

var deathSound = map[enemyKind]string{crawler: "crawler_death", spitter: "spitter_death", drone: "drone_death",
	bomber: "explosion", brute: "brute_death", brood: "boss_death", warden: "warden_death"}

// height is how far above the feet an enemy's body centre is (for aiming).
func (e *Enemy) height() float64 {
	switch e.Kind {
	case drone:
		return 12
	case brood:
		return 20
	case warden:
		return 22
	}
	return 7
}

func clamp(v, lo, hi float64) float64 { return math.Max(lo, math.Min(hi, v)) }
