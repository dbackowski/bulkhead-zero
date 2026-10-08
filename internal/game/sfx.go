package game

import (
	"bytes"
	"io"
	"io/fs"
	"math"
	"math/rand/v2"
	"path"
	"strings"

	"github.com/dbackowski/bulkhead-zero/internal/geom"
	"github.com/hajimehoshi/ebiten/v2/audio"
	"github.com/hajimehoshi/ebiten/v2/audio/wav"
)

const sampleRate = 44100

// sfx plays the generated sound effects (assets/sounds, see tools/artgen/make_sfx.py).
type sfx struct {
	ctx   *audio.Context
	clips map[string][][]byte // name -> decoded PCM of each variant
	last  map[string]float64  // game time each sound last started
	live  []*audio.Player     // held so playing sounds aren't garbage collected
	muted bool
}

// loadClips decodes sounds/<name>_<variant>.wav into PCM, grouped by name.
func loadClips(fsys fs.FS) (map[string][][]byte, error) {
	paths, err := fs.Glob(fsys, "sounds/*.wav")
	if err != nil {
		return nil, err
	}
	clips := map[string][][]byte{}
	for _, p := range paths {
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return nil, err
		}
		s, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		pcm, err := io.ReadAll(s)
		if err != nil {
			return nil, err
		}
		name := strings.TrimSuffix(path.Base(p), ".wav")
		if i := strings.LastIndexByte(name, '_'); i > 0 {
			name = name[:i]
		}
		clips[name] = append(clips[name], pcm)
	}
	return clips, nil
}

func newSFX(fsys fs.FS) (*sfx, error) {
	clips, err := loadClips(fsys)
	if err != nil {
		return nil, err
	}
	ctx := audio.CurrentContext() // only one context may exist per process
	if ctx == nil {
		ctx = audio.NewContext(sampleRate)
	}
	return &sfx{ctx: ctx, clips: clips, last: map[string]float64{}}, nil
}

// sound plays a random variant of name, quieter the farther at is from the
// player. Repeats within 40 ms are dropped so bursts (shotgun pellets hitting
// a wall together) don't turn into noise.
func (g *Game) sound(name string, at geom.Vec) {
	s := g.sfx
	if s == nil || s.muted {
		return
	}
	if t, ok := s.last[name]; ok && g.t-t < 0.04 {
		return
	}
	v := s.clips[name]
	if len(v) == 0 {
		return
	}
	s.last[name] = g.t
	keep := s.live[:0]
	for _, p := range s.live {
		if p.IsPlaying() {
			keep = append(keep, p)
		}
	}
	p := s.ctx.NewPlayerFromBytes(v[rand.IntN(len(v))])
	p.SetVolume(math.Max(0.15, 1-at.Dist(g.player.Pos)/400))
	p.Play()
	s.live = append(keep, p)
}
