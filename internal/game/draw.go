package game

import (
	"image/color"
	"math"
	"sort"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/dbackowski/bulkhead-zero/internal/gfx"
	"github.com/dbackowski/bulkhead-zero/internal/world"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var (
	voidColor = color.RGBA{18, 20, 28, 255}
	globMark  = color.RGBA{120, 220, 60, 255}
)

// renderLevel pre-draws floors, walls and decals into one big image.
func renderLevel(lib *gfx.Library, l *world.Level) *ebiten.Image {
	w, h := l.PixelSize()
	img := ebiten.NewImage(w, h)
	img.Fill(voidColor)
	at := func(t *ebiten.Image, x, y int) {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(x*world.T), float64(y*world.T))
		img.DrawImage(t, op)
	}
	for y := 0; y < l.H; y++ {
		for x := 0; x < l.W; x++ {
			if !l.IsWallCell(x, y) {
				at(lib.T(l.FloorTile(x, y)), x, y)
				continue
			}
			if l.DoorAt(x, y) != nil { // doors move: drawn each frame
				at(lib.T("floor_plate_a"), x, y)
				continue
			}
			tiles := l.WallTiles(x, y)
			if !isCap(tiles[0]) { // faces and doors sit on a floor tile
				at(lib.T("floor_plate_a"), x, y)
			}
			for _, t := range tiles {
				at(lib.T(t), x, y)
			}
		}
	}
	for _, d := range l.Decals {
		at(lib.T(d.Name), d.TX, d.TY)
	}
	return img
}

func isCap(name string) bool { return len(name) > 8 && name[:8] == "wall_cap" }

type drawable struct {
	y    float64
	draw func(dst *ebiten.Image, cam geom.Vec)
}

func (g *Game) drawWorld(dst *ebiten.Image) {
	cam := g.camOffset()
	dst.Fill(voidColor)
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Translate(-cam.X, -cam.Y)
	dst.DrawImage(g.static, op)
	for _, d := range g.lvl.Doors {
		op := &ebiten.DrawImageOptions{}
		op.GeoM.Translate(float64(d.TX*world.T)-cam.X, float64(d.TY*world.T)-cam.Y)
		dst.DrawImage(g.lib.T(doorTile(d)), op)
	}
	for _, s := range g.scorches {
		gfx.DrawCentered(dst, g.lib.T("decal_scorch"), s.X-cam.X, s.Y-cam.Y, 0)
	}
	// acid globs mark where they will land, growing as they fall
	for _, s := range g.shots {
		if s.Kind == shotGlob {
			k := math.Min(s.T/s.Dur, 1)
			fillEllipse(dst, s.To.Sub(cam), 4+11*k, (4+11*k)*0.55, globMark, 0.25+0.35*k)
		}
	}

	g.drawPickups(dst, cam)

	// corpses lie under everything that is still alive
	var list []drawable
	for _, e := range g.enemies {
		e := e
		y := e.Pos.Y
		if e.State == stDead {
			y -= 1000
		}
		list = append(list, drawable{y, func(d *ebiten.Image, c geom.Vec) { g.drawEnemy(d, e, c) }})
	}
	for _, p := range g.lvl.Props {
		if !p.Alive {
			continue
		}
		p := p
		list = append(list, drawable{p.Feet().Y, func(d *ebiten.Image, c geom.Vec) {
			gfx.DrawAtFeet(d, g.lib.T(p.Name), p.Feet().X-c.X, p.Feet().Y-c.Y, gfx.SpriteOpts{})
		}})
	}
	py := g.player.Pos.Y
	if g.player.Dead {
		py -= 999
	}
	list = append(list, drawable{py, g.drawPlayer})
	sort.SliceStable(list, func(i, j int) bool { return list[i].y < list[j].y })
	for _, d := range list {
		d.draw(dst, cam)
	}

	// projectiles and effects on top
	for _, b := range g.bullets {
		v := b.Ground.Sub(geom.V(0, b.H)).Sub(cam)
		gfx.DrawCentered(dst, g.lib.A(b.Anim).Frame(g.t), v.X, v.Y, b.Rot)
	}
	for _, s := range g.shots {
		v := s.Ground.Sub(geom.V(0, s.H)).Sub(cam)
		switch s.Kind {
		case shotLaser:
			gfx.DrawCentered(dst, g.lib.A("fx.laser").Frame(g.t), v.X, v.Y, s.Vel.Angle())
		case shotSpine:
			gfx.DrawCentered(dst, g.lib.A("fx.spine").Frame(g.t), v.X, v.Y, s.Vel.Angle())
		case shotGlob:
			gfx.DrawCentered(dst, g.lib.A("fx.acid_glob").Frame(s.T), v.X, v.Y, 0)
		}
	}
	for _, f := range g.fx {
		if f.T < 0 {
			continue // delayed (the boss's death blasts)
		}
		gfx.DrawCentered(dst, f.Anim.Frame(f.T), f.Pos.X-cam.X, f.Pos.Y-cam.Y, f.Rot)
	}
}

var enemyAnim = map[enemyKind]string{crawler: "crawler", spitter: "spitter", drone: "drone",
	bomber: "bomber", brute: "brute", boss: "boss"}

func (g *Game) drawEnemy(dst *ebiten.Image, e *Enemy, cam geom.Vec) {
	// side-on enemies only have *_right frames, mirrored for left
	side := e.Kind == crawler || e.Kind == bomber || e.Kind == brute
	name, t := enemyAnim[e.Kind], e.AnimT
	switch {
	case e.State == stDead:
		name, t = name+".death", e.DeadT
	case e.State == stAttack && side:
		name += ".attack_right"
	case e.State == stAttack:
		name += ".attack"
	case side:
		name += ".walk_right"
	default:
		name += ".idle"
	}
	a := g.lib.A(name)
	img := a.Frame(t)
	if e.Kind == brute && e.State == stAttack { // frames 0-1 wind-up, 2-3 charge
		i := int(e.T*10) % 2
		if e.T >= stats[brute].fireAt {
			i += 2
		}
		img = a.Frames[i]
	}
	o := gfx.SpriteOpts{Flip: side && e.FaceLeft}
	if e.Flash > 0 {
		o.Flash = 0.85
	}
	gfx.DrawAtFeet(dst, img, e.Pos.X-cam.X, e.Pos.Y-cam.Y, o)
}

func (g *Game) drawPlayer(dst *ebiten.Image, cam geom.Vec) {
	p := g.player
	body, flip, _ := g.bodyFrame()
	o := gfx.SpriteOpts{Flip: flip}
	if p.Invuln > 0 && p.HurtT >= hurtAnimTime && int(p.Invuln/0.08)%2 == 0 {
		o.Alpha = 0.45
	}
	if p.Dead {
		gfx.DrawAtFeet(dst, body, p.Pos.X-cam.X, p.Pos.Y-cam.Y, o)
		return
	}
	behind := g.lib.GunBehind[p.Facing]
	if behind {
		g.drawGun(dst, cam, o.Alpha)
	}
	gfx.DrawAtFeet(dst, body, p.Pos.X-cam.X, p.Pos.Y-cam.Y, o)
	if !behind {
		g.drawGun(dst, cam, o.Alpha)
	}
}

// drawGun rotates the weapon sprite around the hand toward the aim:
// pivot to origin -> flip vertically when aiming left -> rotate -> move to hand.
func (g *Game) drawGun(dst *ebiten.Image, cam geom.Vec, alpha float64) {
	p := g.player
	w := g.weapons[p.Slot]
	hand := g.handPos()
	if p.SinceShot < 0.06 {
		hand = hand.Sub(geom.FromAngle(p.Aim).Scale(w.RecoilPx))
	}
	var m ebiten.GeoM
	m.Translate(-w.Pivot.X-0.5, -w.Pivot.Y-0.5)
	if math.Cos(p.Aim) < 0 {
		m.Scale(1, -1)
	}
	m.Rotate(p.Aim)
	m.Translate(hand.X-cam.X+0.5, hand.Y-cam.Y+0.5)
	gfx.DrawWithGeoM(dst, w.Img, m, gfx.SpriteOpts{Alpha: alpha})
}

// fillEllipse draws a flat ground ellipse (the 3/4 view squashes circles vertically).
func fillEllipse(dst *ebiten.Image, c geom.Vec, rx, ry float64, clr color.Color, alpha float64) {
	var path vector.Path
	for i := 0; i < 16; i++ {
		a := float64(i) * math.Pi / 8
		x, y := float32(c.X+rx*math.Cos(a)), float32(c.Y+ry*math.Sin(a))
		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}
	path.Close()
	op := &vector.DrawPathOptions{}
	op.ColorScale.ScaleWithColor(clr)
	op.ColorScale.ScaleAlpha(float32(alpha))
	vector.FillPath(dst, &path, nil, op)
}
