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

// sfx plays the generated sound effects (assets/sounds, see tools/artgen/make_sfx.py)
// and music loops (assets/music, make_music.py).
type sfx struct {
	ctx   *audio.Context
	clips map[string][][]byte // name -> decoded PCM of each variant
	last  map[string]float64  // game time each sound last started
	live  []*audio.Player     // held so playing sounds aren't garbage collected
	music map[string]*track
	duckT float64 // music stays ducked while > 0 (a stinger is playing)
	duck  float64 // current music level from ducking, 0..1
	muted bool
}

// track is one music loop and its current fade level (0..1).
type track struct {
	p   *audio.Player
	vol float64
}

// musicTracks lists every music loop (assets/music/music_<name>.wav): each
// deck has its own calm, combat and boss loop, named with its music suffix.
func musicTracks() []string {
	var names []string
	for _, l := range levels {
		for _, base := range []string{"calm", "combat", "boss"} {
			names = append(names, base+l.music)
		}
	}
	return names
}

const (
	musicVolume  = 0.45 // under the sound effects
	musicFade    = 1.5  // seconds for a crossfade
	combatLinger = 5.0  // seconds combat music outlasts the last hunter: longer than a wave break, so it carries through
	duckLevel    = 0.3  // music level under a stinger
	duckTime     = 2.0  // seconds the music stays ducked
)

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
	s := &sfx{ctx: ctx, clips: clips, last: map[string]float64{}, music: map[string]*track{}, duck: 1}
	for _, name := range musicTracks() {
		b, err := fs.ReadFile(fsys, "music/music_"+name+".wav")
		if err != nil {
			return nil, err
		}
		st, err := wav.DecodeWithSampleRate(sampleRate, bytes.NewReader(b))
		if err != nil {
			return nil, err
		}
		p, err := ctx.NewPlayer(audio.NewInfiniteLoop(st, st.Length()))
		if err != nil {
			return nil, err
		}
		s.music[name] = &track{p: p}
	}
	return s, nil
}

// stinger plays a short musical cue (stinger_<name> in assets/sounds, from
// make_music.py) at full volume and ducks the music under it.
func (g *Game) stinger(name string) {
	g.sound("stinger_"+name, g.player.Pos)
	if g.sfx != nil {
		g.sfx.duckT = duckTime
	}
}

// musicTrack picks the current deck's loop for the moment: the boss theme
// while a boss is alive, combat while enemies are hunting the player (and
// for a few seconds after), calm otherwise.
func (g *Game) musicTrack() string {
	mood := "calm"
	switch {
	case g.state == stateDead || g.state == stateClear:
	case g.boss != nil && g.boss.State != stDead:
		mood = "boss"
	case g.fightT > 0:
		mood = "combat"
	}
	return mood + g.deck().music
}

// updateFight keeps fightT topped up while a wave is coming in or any enemy
// is awake and hunting.
func (g *Game) updateFight() {
	hunted := len(g.queue) > 0
	for _, e := range g.enemies {
		hunted = hunted || e.Aggro && e.State != stDead
	}
	if hunted {
		g.fightT = combatLinger
	} else {
		g.fightT -= dt
	}
}

// updateMusic crossfades toward the wanted loop. A loop that fades in starts
// from its beginning; it is quieter while paused and silent when muted.
func (g *Game) updateMusic() {
	s := g.sfx
	if s == nil {
		return
	}
	s.duckT -= dt
	if s.duckT > 0 {
		s.duck = math.Max(s.duck-dt*3, duckLevel)
	} else {
		s.duck = math.Min(s.duck+dt, 1)
	}
	want, gain := g.musicTrack(), musicVolume*s.duck
	if s.muted {
		gain = 0
	} else if g.state == statePaused {
		gain *= 0.4
	}
	for name, tr := range s.music {
		step := dt / musicFade
		if name == want {
			tr.vol = math.Min(tr.vol+step, 1)
		} else {
			tr.vol = math.Max(tr.vol-step, 0)
		}
		tr.p.SetVolume(tr.vol * gain)
		switch {
		case tr.vol > 0 && !tr.p.IsPlaying():
			_ = tr.p.Rewind()
			tr.p.Play()
		case tr.vol == 0 && tr.p.IsPlaying():
			tr.p.Pause()
		}
	}
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
