#!/usr/bin/env python3
"""Music loops for Bulkhead Zero, synthesised like the sound effects.

Writes music_<name>.wav (16-bit mono, 22.05 kHz to keep the files small)
into the current directory; build.py moves them to assets/music/. Each
track is a seamless loop: it is rendered with a tail, and the tail (echo,
reverb, ringing notes) is folded back onto the start.

    calm    exploring and between waves: pads, a slow bell arpeggio, a pulse
    combat  during a wave: driving bass, drums, a 16th-note arpeggio
    boss    the boss fight: faster, darker progression, heavier drums, stabs

Standard library only (reuses make_sfx's synth helpers); seeded, so the
output is identical on every run.
"""
import math
import random
import struct
import wave

from make_sfx import RATE, biquad, env, noise, reverb, scale, sweep, tone

OUT_RATE = RATE // 2


def hz(midi):
    return 440.0 * 2 ** ((midi - 69) / 12)


def add(buf, at, sound, gain=1.0):
    """Mix sound into buf starting at sample at."""
    for i, x in enumerate(sound):
        j = at + i
        if j >= len(buf):
            break
        buf[j] += x * gain


def saw(n, f, detune=0.0, rng=None):
    out, ph = [], (rng.random() if rng else 0.0)
    step = f * (1 + detune) / RATE
    for _ in range(n):
        ph = (ph + step) % 1.0
        out.append(1 - 2 * ph)
    return out


def square(n, f, duty=0.5):
    step, ph, out = f / RATE, 0.0, []
    for _ in range(n):
        ph = (ph + step) % 1.0
        out.append(1.0 if ph < duty else -1.0)
    return out


def adsr(xs, a, d, s, r):
    """Attack/decay to sustain level s, release over the last r seconds."""
    n, out = len(xs), []
    na, nd, nr = max(1, int(a * RATE)), max(1, int(d * RATE)), max(1, int(r * RATE))
    for i, x in enumerate(xs):
        if i < na:
            k = i / na
        elif i < na + nd:
            k = 1 - (1 - s) * (i - na) / nd
        else:
            k = s
        if i > n - nr:
            k *= (n - i) / nr
        out.append(x * k)
    return out


# ---- instruments ------------------------------------------------------------
def kick(rng, amp=1.0):
    n = int(0.3 * RATE)
    body = tone(n, sweep(160, 45, 0.03), 0.14)
    click = env(biquad(noise(rng, n), "hp", 3000), 0.003)
    return [amp * (b + 0.3 * c) for b, c in zip(body, click)]


def snare(rng, amp=1.0):
    n = int(0.25 * RATE)
    rattle = env(biquad(noise(rng, n), "bp", 1900, q=0.8), 0.07)
    body = tone(n, sweep(240, 180, 0.02), 0.05)
    return [amp * (0.9 * r + 0.5 * b) for r, b in zip(rattle, body)]


def hat(rng, amp=1.0, length=0.04):
    n = int(0.12 * RATE)
    return scale(env(biquad(noise(rng, n), "hp", 7500), length), amp)


def bass(note, secs, cutoff=900, amp=1.0):
    n = int(secs * RATE)
    f = hz(note)
    osc = [0.7 * a + 0.5 * b for a, b in zip(saw(n, f), square(n, f / 2))]
    osc = biquad(osc, "lp", sweep(cutoff * 2.5, cutoff * 0.6, 0.06), q=1.4)
    return scale(adsr(osc, 0.004, 0.08, 0.7, 0.03), amp)


def pad(rng, notes, secs, cutoff=1400, amp=1.0):
    n = int(secs * RATE)
    mixed = [0.0] * n
    for m in notes:
        for det in (-0.004, 0.004):
            for i, x in enumerate(saw(n, hz(m), det, rng)):
                mixed[i] += x
    mixed = biquad(mixed, "lp", cutoff, q=0.8)
    return scale(adsr(mixed, 0.5, 0.4, 0.8, 0.6), amp / (2 * len(notes)))


def pluck(note, secs=0.16, cutoff=3000, amp=1.0, duty=0.25):
    n = int(secs * RATE)
    osc = biquad(square(n, hz(note), duty), "lp", sweep(cutoff * 2, cutoff * 0.5, 0.05))
    return scale(adsr(osc, 0.002, 0.05, 0.4, 0.05), amp)


def bell(note, amp=1.0):
    n = int(1.6 * RATE)
    f = hz(note)
    return scale([a + 0.4 * b for a, b in zip(tone(n, f, 0.6), tone(n, f * 2.76, 0.25))], amp)


def echo(xs, secs, feedback, mix):
    d = int(secs * RATE)
    out = list(xs)
    for i in range(d, len(out)):
        out[i] += out[i - d] * feedback
    return [x + (y - x) * mix for x, y in zip(xs, out)]


# ---- arrangement ------------------------------------------------------------
# Chords as MIDI notes; the first note is the root.
Am, F, C, G, Dm, E = (57, 60, 64), (53, 57, 60), (48, 52, 55), (55, 59, 62), (50, 53, 57), (52, 56, 59)
Bb, Gm, A = (46, 50, 53), (55, 58, 62), (57, 61, 64)


class Track:
    def __init__(self, bpm, bars, tail=3.0):
        self.beat = 60 / bpm
        self.bars = bars
        self.loop = int(bars * 4 * self.beat * RATE)
        n = self.loop + int(tail * RATE)
        self.drums, self.low, self.pads, self.lead = ([0.0] * n for _ in range(4))

    def at(self, bar, beat):
        return int((bar * 4 + beat) * self.beat * RATE)

    def mixdown(self, levels, wet):
        """levels: target RMS of drums, bass, pads and lead, which sets the balance
        whatever the parts' raw loudness. The sum is then gently limited."""
        buses = (self.drums, biquad(self.low, "lp", 2500), reverb(self.pads, wet, size=1.6, decay=0.85),
                 reverb(self.lead, wet * 0.6, size=1.2))
        out = [0.0] * len(self.drums)
        for bus, level in zip(buses, levels):
            k = level / (rms(bus) or 1.0)
            for i, x in enumerate(bus):
                out[i] += x * k
        loop = out[:self.loop]
        for i, x in enumerate(out[self.loop:]):  # fold the tail onto the start
            loop[i] += x
        peak = max(abs(x) for x in loop) or 1.0
        return [math.tanh(1.2 * x / peak) for x in loop]  # only the loudest hits get rounded off


def rms(xs):
    return math.sqrt(sum(x * x for x in xs) / len(xs))


def calm(rng):
    t = Track(bpm=76, bars=8)
    prog = [Am, F, C, E, Am, F, Dm, E]
    for bar, ch in enumerate(prog):
        add(t.pads, t.at(bar, 0), pad(rng, ch + (ch[0] + 12,), 4 * t.beat + 0.3, cutoff=1100))
        add(t.low, t.at(bar, 0), bass(ch[0] - 12, 4 * t.beat - 0.05, cutoff=300, amp=0.6))
        for step, k in enumerate((0, 2, 1, 2)):  # slow bell arpeggio, one note per beat
            add(t.lead, t.at(bar, step), bell(ch[k] + 12, amp=0.35 if step else 0.45))
        for beat in range(4):  # soft heartbeat pulse
            add(t.drums, t.at(bar, beat), kick(rng, 0.35 if beat % 2 == 0 else 0.15))
            add(t.drums, t.at(bar, beat + 0.5), hat(rng, 0.08))
    return t.mixdown((0.04, 0.09, 0.10, 0.08), wet=0.45)


def combat(rng):
    t = Track(bpm=128, bars=16)
    prog = [Am, Am, F, G, Am, Am, F, E] * 2
    for bar, ch in enumerate(prog):
        second_half = bar >= 8
        add(t.pads, t.at(bar, 0), pad(rng, ch, 4 * t.beat, cutoff=1600, amp=0.6))
        for e in range(8):  # driving 8th-note bass, octave jump on the off-beats
            note = ch[0] - 12 + (12 if e % 2 else 0)
            add(t.low, t.at(bar, e / 2), bass(note, t.beat / 2 - 0.01, cutoff=700))
        for s in range(16):  # 16th arpeggio up and down the chord
            k = (0, 1, 2, 1)[s % 4] if not second_half else (0, 2, 1, 2)[s % 4]
            octave = 12 if (s // 4) % 2 else 24
            add(t.lead, t.at(bar, s / 4), pluck(ch[k] + octave - 12, cutoff=2400, amp=0.35))
        for beat in range(4):
            add(t.drums, t.at(bar, beat), kick(rng, 0.9))
            if beat in (1, 3):
                add(t.drums, t.at(bar, beat), snare(rng, 0.7))
            for h in (0.5,) if not second_half else (0.25, 0.5, 0.75):
                add(t.drums, t.at(bar, beat + h), hat(rng, 0.25))
        if bar % 4 == 3:  # fill into the next phrase
            for s in (3.25, 3.5, 3.75):
                add(t.drums, t.at(bar, s), snare(rng, 0.45))
    t.lead = echo(t.lead, t.beat * 0.75, 0.3, 0.5)
    return t.mixdown((0.15, 0.12, 0.06, 0.08), wet=0.25)


def boss(rng):
    t = Track(bpm=144, bars=16)
    prog = [Dm, Dm, Bb, A, Dm, Gm, Bb, A] * 2
    for bar, ch in enumerate(prog):
        add(t.pads, t.at(bar, 0), pad(rng, ch + (ch[0] - 12,), 4 * t.beat, cutoff=1800, amp=0.7))
        for s in range(16):  # relentless 16th bass
            note = ch[0] - 24 + (7 if s % 8 == 6 else 0)
            add(t.low, t.at(bar, s / 4), bass(note, t.beat / 4 - 0.005, cutoff=600, amp=0.9))
        for beat in range(4):
            add(t.drums, t.at(bar, beat), kick(rng, 1.0))
            add(t.drums, t.at(bar, beat + 0.5), kick(rng, 0.5) if bar % 2 else hat(rng, 0.3))
            if beat in (1, 3):
                add(t.drums, t.at(bar, beat), snare(rng, 0.85))
            for h in (0.25, 0.5, 0.75):
                add(t.drums, t.at(bar, beat + h), hat(rng, 0.22, length=0.02))
        for beat, k in ((0, 0), (1.5, 2), (3, 1)):  # brass-like stabs
            add(t.lead, t.at(bar, beat), pluck(ch[k] + 12, secs=0.22, cutoff=1800, amp=0.45, duty=0.5))
            add(t.lead, t.at(bar, beat), pluck(ch[k] + 24, secs=0.22, cutoff=2600, amp=0.2, duty=0.3))
        if bar % 8 == 7:
            for s in (2, 2.5, 3, 3.25, 3.5, 3.75):
                add(t.drums, t.at(bar, s), snare(rng, 0.6))
    t.lead = echo(t.lead, t.beat * 0.5, 0.25, 0.4)
    return t.mixdown((0.17, 0.12, 0.06, 0.08), wet=0.3)


TRACKS = {"calm": calm, "combat": combat, "boss": boss}


def write(name, samples):
    smooth = biquad(biquad(samples, "lp", 9000), "lp", 9000)  # no aliasing when halving the rate
    half = smooth[::2]
    k = 0.85 / (max(abs(s) for s in half) or 1.0)
    with wave.open(name, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(OUT_RATE)
        w.writeframes(b"".join(struct.pack("<h", int(s * k * 32767)) for s in half))


def main():
    for i, (name, make) in enumerate(TRACKS.items()):
        write(f"music_{name}.wav", make(random.Random(1000 + i)))
    print(f"music: {len(TRACKS)} loops")


if __name__ == "__main__":
    main()
