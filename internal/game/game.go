// Package game is the playable prototype: one station level, the marine,
// three enemy types, weapons, camera and HUD.
package game

import (
	"math"
	"math/rand"
	"os"

	"github.com/dbackowski/bulkhead-zero/assets"
	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/dbackowski/bulkhead-zero/internal/gfx"
	"github.com/dbackowski/bulkhead-zero/internal/world"
	"github.com/hajimehoshi/ebiten/v2"
)

// Logical resolution; Ebitengine scales it up to the window with nearest filtering.
const (
	ScreenW = 480
	ScreenH = 270
	dt      = 1.0 / 60
)

type state int

const (
	statePlaying state = iota
	statePaused
	stateDead
	stateClear
)

// Game implements ebiten.Game.
type Game struct {
	lib     *gfx.Library
	lvl     *world.Level
	static  *ebiten.Image // pre-rendered floors, walls, decals
	canvas  *ebiten.Image
	rng     *rand.Rand
	weapons []*gfx.Weapon

	player     *Player
	enemies    []*Enemy
	bullets    []*Bullet
	shots      []*Shot
	fx         []*FX
	scorches   []geom.Vec
	blasts     []pendingBlast
	flow       []int
	flowTimer  float64
	cam        geom.Vec
	shakeAmp   float64
	shakeT     float64
	t          float64
	stateT     float64
	state      state
	totalFoes  int
	autopilot  *autopilot
	recorder   *recorder
	fullscreen bool
}

// New loads assets and starts a fresh round.
func New() (*Game, error) {
	lib, err := gfx.Load(assets.FS)
	if err != nil {
		return nil, err
	}
	g := &Game{lib: lib, rng: rand.New(rand.NewSource(rand.Int63()))}
	for _, name := range weaponOrder {
		w, ok := lib.Weapons[name]
		if !ok {
			continue
		}
		g.weapons = append(g.weapons, w)
	}
	g.canvas = ebiten.NewImage(ScreenW, ScreenH)
	switch os.Getenv("BZ_AUTOPILOT") {
	case "":
	case "human":
		g.autopilot = newHumanAutopilot()
	default:
		g.autopilot = newAutopilot()
	}
	if dir := os.Getenv("BZ_RECORD"); dir != "" {
		g.recorder = newRecorder(dir)
	}
	if err := g.reset(); err != nil {
		return nil, err
	}
	ebiten.SetCursorMode(ebiten.CursorModeHidden)
	return g, nil
}

func (g *Game) reset() error {
	lvl, err := world.Parse(world.Station)
	if err != nil {
		return err
	}
	g.lvl = lvl
	g.static = renderLevel(g.lib, lvl)
	g.player = newPlayer(lvl.Start)
	g.enemies = g.enemies[:0]
	for _, s := range lvl.Spawns {
		g.enemies = append(g.enemies, newEnemy(s, g.rng))
	}
	g.totalFoes = len(g.enemies)
	g.bullets, g.shots, g.fx, g.scorches, g.blasts = nil, nil, nil, nil, nil
	g.state, g.stateT, g.t = statePlaying, 0, 0
	g.flow = lvl.DistanceField(g.player.Pos)
	g.cam = g.cameraTarget(geom.V(ScreenW/2, ScreenH/2))
	return nil
}

// Layout fixes the logical screen size.
func (g *Game) Layout(_, _ int) (int, int) { return ScreenW, ScreenH }

// Update runs one 1/60 s tick.
func (g *Game) Update() error {
	if g.Finished() {
		return ebiten.Termination
	}
	in := readInput(g)
	if g.autopilot != nil {
		in = g.autopilot.input(g, in)
	}
	if in.Fullscreen {
		g.fullscreen = !g.fullscreen
		ebiten.SetFullscreen(g.fullscreen)
	}
	if in.Restart {
		return g.reset()
	}
	if in.Pause && (g.state == statePlaying || g.state == statePaused) {
		if g.state == statePlaying {
			g.state = statePaused
			ebiten.SetCursorMode(ebiten.CursorModeVisible)
		} else {
			g.state = statePlaying
			ebiten.SetCursorMode(ebiten.CursorModeHidden)
		}
	}
	if g.state == statePaused {
		return nil
	}
	g.t += dt
	g.stateT += dt

	g.updatePlayer(in)
	g.flowTimer -= dt
	if g.flowTimer <= 0 {
		g.flow = g.lvl.DistanceField(g.player.Pos)
		g.flowTimer = 0.25
	}
	for _, e := range g.enemies {
		g.updateEnemy(e)
	}
	g.updateBullets()
	g.updateShots()
	g.updateBlasts()
	g.updateFX()
	g.updateCamera(in.Mouse)

	switch {
	case g.state == statePlaying && g.player.Dead && g.player.DeadT > 1.2:
		g.state, g.stateT = stateDead, 0
	case g.state == statePlaying && g.aliveFoes() == 0:
		g.state, g.stateT = stateClear, 0
	}
	if g.recorder != nil {
		g.recorder.tick(g)
	}
	return nil
}

func (g *Game) aliveFoes() int {
	n := 0
	for _, e := range g.enemies {
		if e.State != stDead {
			n++
		}
	}
	return n
}

// cameraTarget centres on the player and leans a little toward the cursor.
func (g *Game) cameraTarget(mouse geom.Vec) geom.Vec {
	look := mouse.Sub(geom.V(ScreenW/2, ScreenH/2)).Scale(0.22)
	return g.player.Pos.Add(look).Sub(geom.V(ScreenW/2, ScreenH/2-8))
}

func (g *Game) updateCamera(mouse geom.Vec) {
	k := 1 - math.Exp(-dt*9)
	g.cam = g.cam.Lerp(g.cameraTarget(mouse), k)
	mw, mh := g.lvl.PixelSize()
	g.cam.X = math.Max(0, math.Min(g.cam.X, float64(mw-ScreenW)))
	g.cam.Y = math.Max(0, math.Min(g.cam.Y, float64(mh-ScreenH)))
	if g.shakeT > 0 {
		g.shakeT -= dt
	}
}

// camOffset is the integer camera position used for drawing (with shake).
func (g *Game) camOffset() geom.Vec {
	c := geom.V(math.Round(g.cam.X), math.Round(g.cam.Y))
	if g.shakeT > 0 {
		a := g.shakeAmp * (g.shakeT / 0.2)
		c = c.Add(geom.V(math.Round((g.rng.Float64()*2-1)*a), math.Round((g.rng.Float64()*2-1)*a)))
	}
	return c
}

// mouseWorld converts a logical-screen cursor position to world space.
func (g *Game) mouseWorld(mouse geom.Vec) geom.Vec {
	return mouse.Add(geom.V(math.Round(g.cam.X), math.Round(g.cam.Y)))
}

func (g *Game) shake(amp, dur float64) {
	if amp >= g.shakeAmp || g.shakeT <= 0 {
		g.shakeAmp = amp
	}
	g.shakeT = math.Max(g.shakeT, dur)
}

// Draw renders the frame.
func (g *Game) Draw(screen *ebiten.Image) {
	g.drawWorld(g.canvas)
	g.drawHUD(g.canvas)
	screen.DrawImage(g.canvas, nil)
	if g.recorder != nil {
		g.recorder.capture(g.canvas)
	}
}
