package gfx

import (
	"bytes"
	"fmt"
	"image"
	_ "image/png" // register the PNG decoder
	"io/fs"
	"path"
	"strings"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/hajimehoshi/ebiten/v2"
)

// Anim is a sequence of frames with timing.
type Anim struct {
	Name    string
	Frames  []*ebiten.Image
	FrameMS float64
	Loop    bool
	Bob     []int
}

// Index returns the frame index t seconds into the animation.
func (a *Anim) Index(t float64) int {
	if t < 0 {
		t = 0
	}
	i := int(t * 1000 / a.FrameMS)
	n := len(a.Frames)
	if a.Loop {
		return i % n
	}
	if i >= n {
		return n - 1
	}
	return i
}

// Frame returns the frame t seconds into the animation.
func (a *Anim) Frame(t float64) *ebiten.Image { return a.Frames[a.Index(t)] }

// Duration is the length of one play-through in seconds.
func (a *Anim) Duration() float64 { return float64(len(a.Frames)) * a.FrameMS / 1000 }

// BobAt returns the vertical body offset for frame i (player idle/walk only).
func (a *Anim) BobAt(i int) float64 {
	if len(a.Bob) == 0 {
		return 0
	}
	return float64(a.Bob[i%len(a.Bob)])
}

// Weapon is a rotatable gun sprite plus its stats.
type Weapon struct {
	Name        string
	Img         *ebiten.Image
	Pivot       geom.Vec
	Muzzle      geom.Vec
	Height      float64 // sprite height, for flipping the pivot
	FireMS      float64
	SpreadDeg   float64
	Pellets     int
	RecoilPx    float64
	BulletSpeed float64
}

// Library holds every loaded image and animation.
type Library struct {
	FrameSize  int
	TileSize   int
	Anims      map[string]*Anim // "player.walk_right", "crawler.death", "fx.bullet", ...
	Tiles      map[string]*ebiten.Image
	Weapons    map[string]*Weapon
	HandAnchor map[string]geom.Vec
	GunBehind  map[string]bool
	Facings    []string
}

// A returns an animation by name and panics if it doesn't exist (a typo is a bug).
func (l *Library) A(name string) *Anim {
	a, ok := l.Anims[name]
	if !ok {
		panic("gfx: unknown animation " + name)
	}
	return a
}

// T returns a tile image by name and panics if it doesn't exist.
func (l *Library) T(name string) *ebiten.Image {
	t, ok := l.Tiles[name]
	if !ok {
		panic("gfx: unknown tile " + name)
	}
	return t
}

func loadImage(fsys fs.FS, p string) (*ebiten.Image, error) {
	b, err := fs.ReadFile(fsys, p)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", p, err)
	}
	return ebiten.NewImageFromImage(img), nil
}

func sub(img *ebiten.Image, x, y, w, h int) *ebiten.Image {
	return img.SubImage(image.Rect(x, y, x+w, y+h)).(*ebiten.Image)
}

// sheetPrefix maps a sheet file to the animation name prefix.
var sheetPrefix = map[string]string{
	"player.png":  "player.",
	"enemies.png": "", // rows are already named "crawler.walk_right" etc.
	"fx.png":      "fx.",
}

// Load reads every sheet and its metadata from fsys (see assets.FS).
func Load(fsys fs.FS) (*Library, error) {
	sm, err := LoadSpriteMeta(fsys)
	if err != nil {
		return nil, err
	}
	tm, err := LoadTileMeta(fsys)
	if err != nil {
		return nil, err
	}
	lib := &Library{
		FrameSize:  sm.FrameSize,
		TileSize:   tm.TileSize,
		Anims:      map[string]*Anim{},
		Tiles:      map[string]*ebiten.Image{},
		Weapons:    map[string]*Weapon{},
		HandAnchor: map[string]geom.Vec{},
		GunBehind:  map[string]bool{},
		Facings:    sm.Player.Facings,
	}
	fsz := sm.FrameSize

	for sheet, anims := range sm.Sheets {
		img, err := loadImage(fsys, path.Join("sprites", sheet))
		if err != nil {
			return nil, err
		}
		prefix, ok := sheetPrefix[sheet]
		if !ok {
			prefix = strings.TrimSuffix(sheet, ".png") + "."
		}
		for name, m := range anims {
			a := &Anim{Name: prefix + name, FrameMS: float64(m.FrameMS), Loop: m.Loop, Bob: m.Bob}
			if a.FrameMS <= 0 {
				a.FrameMS = 100
			}
			for i := 0; i < m.Frames; i++ {
				a.Frames = append(a.Frames, sub(img, i*fsz, m.Row*fsz, fsz, fsz))
			}
			lib.Anims[a.Name] = a
		}
	}

	wimg, err := loadImage(fsys, "sprites/weapons.png")
	if err != nil {
		return nil, err
	}
	for name, w := range sm.Weapons {
		lib.Weapons[name] = &Weapon{
			Name:        name,
			Img:         sub(wimg, w.Rect[0], w.Rect[1], w.Rect[2], w.Rect[3]),
			Pivot:       geom.V(float64(w.Pivot[0]), float64(w.Pivot[1])),
			Muzzle:      geom.V(float64(w.Muzzle[0]), float64(w.Muzzle[1])),
			Height:      float64(w.Rect[3]),
			FireMS:      float64(w.FireMS),
			SpreadDeg:   w.SpreadDeg,
			Pellets:     max(w.Pellets, 1),
			RecoilPx:    w.RecoilPx,
			BulletSpeed: w.BulletSpeed,
		}
	}
	for f, a := range sm.Player.HandAnchor {
		lib.HandAnchor[f] = geom.V(float64(a[0]), float64(a[1]))
	}
	for _, f := range sm.Player.GunBehindBody {
		lib.GunBehind[f] = true
	}

	timg, err := loadImage(fsys, "tiles/tileset.png")
	if err != nil {
		return nil, err
	}
	idx, err := tm.TileIndex()
	if err != nil {
		return nil, err
	}
	ts := tm.TileSize
	for name, i := range idx {
		lib.Tiles[name] = sub(timg, (i%tm.Columns)*ts, (i/tm.Columns)*ts, ts, ts)
	}
	return lib, nil
}
