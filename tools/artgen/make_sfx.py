#!/usr/bin/env python3
"""Sound effects for Bulkhead Zero, synthesised from noise and sine waves.

Writes 16-bit mono 44.1 kHz WAVs named <sound>_<variant>.wav into the
current directory (build.py moves them to assets/sounds/). The game picks a
random variant each time, which stands in for pitch variation. Standard
library only; seeded, so the output is identical on every run.

Guns are layered the way recorded gunshots are built up in games: a bright
crack, a noise body whose filter closes quickly, a low punch, soft clipping
for weight, and a short metal-room reverb tail.
"""
import math
import random
import struct
import wave

RATE = 44100
VARIANTS = 3


def biquad(xs, kind, freq, q=0.707):
    """RBJ-cookbook filter ('lp', 'hp' or 'bp'). freq may be a function of time
    in seconds, for sweeps."""
    out = []
    x1 = x2 = y1 = y2 = 0.0
    for i, x in enumerate(xs):
        f = freq(i / RATE) if callable(freq) else freq
        w = 2 * math.pi * min(f, RATE * 0.45) / RATE
        cw, alpha = math.cos(w), math.sin(w) / (2 * q)
        if kind == "lp":
            b0, b1, b2 = (1 - cw) / 2, 1 - cw, (1 - cw) / 2
        elif kind == "hp":
            b0, b1, b2 = (1 + cw) / 2, -(1 + cw), (1 + cw) / 2
        else:  # band-pass, 0 dB at the centre
            b0, b1, b2 = alpha, 0.0, -alpha
        a0, a1, a2 = 1 + alpha, -2 * cw, 1 - alpha
        y = (b0 * x + b1 * x1 + b2 * x2 - a1 * y1 - a2 * y2) / a0
        x2, x1, y2, y1 = x1, x, y1, y
        out.append(y)
    return out


def reverb(xs, wet, size=1.0, decay=0.78):
    """Small Schroeder reverb: four damped combs into two all-passes. Only the
    sound above 350 Hz is sent in, so low thumps don't ring on as a hum."""
    def comb(src, ms):
        d = int(ms * size * RATE / 1000)
        buf, out, lp = [0.0] * d, [], 0.0
        for i, x in enumerate(src):
            y = buf[i % d]
            lp += 0.4 * (y - lp)  # damping: the tail gets darker as it decays
            buf[i % d] = x + lp * decay
            out.append(y)
        return out

    def allpass(src, ms, g=0.6):
        d = int(ms * RATE / 1000)
        buf, out = [0.0] * d, []
        for i, x in enumerate(src):
            b = buf[i % d]
            y = -g * x + b
            buf[i % d] = x + g * y
            out.append(y)
        return out

    send = biquad(xs, "hp", 350)
    combs = [comb(send, ms) for ms in (23.3, 28.9, 33.7, 39.1)]
    w = [sum(c[i] for c in combs) / 4 for i in range(len(xs))]
    w = allpass(allpass(w, 5.0), 1.7)
    return [x + wet * y for x, y in zip(xs, w)]


def sweep(f0, f1, tau):
    """A frequency that starts at f0 and settles exponentially to f1."""
    return lambda t: f1 + (f0 - f1) * math.exp(-t / tau)


def tone(n, freq, decay, rng=None):
    """Sine with an exponential decay; freq may be a sweep."""
    out, phase = [], (rng.random() * 2 * math.pi if rng else 0.0)
    for i in range(n):
        t = i / RATE
        phase += 2 * math.pi * (freq(t) if callable(freq) else freq) / RATE
        out.append(math.sin(phase) * math.exp(-t / decay))
    return out


def env(xs, decay, delay=0.0):
    return [x * (math.exp(-(i / RATE - delay) / decay) if i / RATE >= delay else 0.0) for i, x in enumerate(xs)]


def delayed(xs, at):
    """Shift a sound to start at `at` seconds, keeping the length."""
    k = int(at * RATE)
    return [0.0] * k + xs[:len(xs) - k]


def drive(xs, amount):
    """Soft clipping: adds weight and loudness to the transient."""
    k = math.tanh(amount)
    return [math.tanh(x * amount) / k for x in xs]


def mix(*layers):
    return [sum(v) for v in zip(*layers)]


def scale(xs, k):
    return [x * k for x in xs]


def noise(rng, n):
    return [rng.uniform(-1, 1) for _ in range(n)]


def finish(xs, fade=0.015):
    """Fade-in of 1 ms and a short fade-out so nothing pops."""
    n, fi, fo = len(xs), int(0.001 * RATE), int(fade * RATE)
    return [x * min(1.0, i / fi, (n - i) / fo) for i, x in enumerate(xs)]


def gun(rng, length, crack, body, body_decay, punch, punch_decay, punch_amp, amount, wet, room):
    n = int(length * RATE)
    nz = noise(rng, n)
    layers = [
        scale(env(biquad(nz, "hp", 2500), 0.003), crack),         # crack
        env(biquad(nz, "lp", body, q=0.9), body_decay),            # noise body, filter closing
        scale(tone(n, punch, punch_decay), punch_amp),             # low punch
    ]
    dry = drive(mix(*layers), amount)
    return finish(reverb(dry, wet, size=room))


def shotgun(rng, v):
    s = gun(rng, 0.62, 0.9, sweep(5200 - 300 * v, 500, 0.03), 0.07,
            sweep(160 - 8 * v, 65, 0.015), 0.03, 0.6, 3.0, 0.3, 1.25)
    # the pump: two short clacks once the boom has died down
    n = len(s)
    for at, f in ((0.36, 2300), (0.44, 1700)):
        c = env(biquad(noise(rng, n), "bp", f, q=3), 0.006, delay=at)
        s = [x + 1.6 * y for x, y in zip(s, c)]
    return finish(s)


def hit_flesh(rng, v):
    """A wet thwack: low thump plus a squelchy, wobbling noise burst."""
    n = int(0.16 * RATE)
    thump = scale(tone(n, sweep(240 + 20 * v, 100, 0.015), 0.022), 0.5)
    wet = env(biquad(noise(rng, n), "lp", sweep(1800, 350, 0.025), q=1.5), 0.05)
    wobble = [1 - 0.5 * math.sin(2 * math.pi * (32 + 4 * v) * i / RATE) ** 2 for i in range(n)]
    slap = scale(env(biquad(noise(rng, n), "bp", 1400, q=1.0), 0.004), 0.7)
    return finish(drive(mix(thump, [w * s for w, s in zip(wet, wobble)], slap), 1.6))


def hit_metal(rng, v):
    """A clank: inharmonic partials like a struck metal plate."""
    n = int(0.22 * RATE)
    k = 1 + 0.05 * (v - 1)
    partials = [tone(n, f * k, d, rng) for f, d in ((870, 0.035), (1460, 0.025), (2390, 0.018), (3710, 0.01))]
    ring = scale(mix(*partials), 0.3)
    clang = env(biquad(noise(rng, n), "bp", 1900 * k, q=2.0), 0.02)
    tick = scale(env(biquad(noise(rng, n), "hp", 3000), 0.003), 0.8)
    return finish(reverb(mix(ring, clang, tick), 0.15, size=0.6))


def hit_wall(rng, v):
    """A dull impact on a bulkhead: mid knock plus a puff of grit."""
    n = int(0.12 * RATE)
    knock = scale(tone(n, sweep(520 + 40 * v, 230, 0.01), 0.012), 0.7)
    grit = env(biquad(noise(rng, n), "bp", 2200, q=0.8), 0.012)
    return finish(reverb(drive(mix(knock, grit), 1.5), 0.12, size=0.6))


def fm(n, freq, mod, depth, decay):
    """Frequency-modulated tone: a fast wobble on the pitch gives a raspy,
    creature-like voice. freq may be a sweep."""
    out, phase = [], 0.0
    for i in range(n):
        t = i / RATE
        f = (freq(t) if callable(freq) else freq) * (1 + depth * math.sin(2 * math.pi * mod * t))
        phase += 2 * math.pi * f / RATE
        out.append(math.sin(phase) * math.exp(-t / decay))
    return out


def bubbles(rng, n, count, lo, hi, spread):
    """Short sine blips that rise in pitch, like bubbles popping in liquid."""
    out = [0.0] * n
    for _ in range(count):
        start, f0 = int(rng.uniform(0, spread) * RATE), rng.uniform(lo, hi)
        for j, x in enumerate(tone(int(0.04 * RATE), sweep(f0, f0 * 1.8, 0.015), 0.012)):
            if start + j < n:
                out[start + j] += x * math.exp(-start / RATE / (spread * 0.6))
    return out


def boom(rng, n, cutoff_tau, decay, punch_amp, amount):
    """The core of an explosion: darkening noise roar plus a deep thump."""
    roar = env(biquad(noise(rng, n), "lp", sweep(3500, 180, cutoff_tau), q=0.8), decay)
    punch = scale(tone(n, sweep(150, 55, 0.04), decay * 0.3), punch_amp)
    return drive(mix(roar, punch), amount)


def explosion(rng, v):
    n = int(1.4 * RATE)
    s = boom(rng, n, 0.12 + 0.02 * v, 0.28, 0.55, 3.0)
    # debris: sparse crackles that thin out over the first half second
    crackle = [0.0] * n
    for _ in range(40):
        at = rng.expovariate(1 / 0.18)
        i = int(at * RATE)
        for j in range(int(0.004 * RATE)):
            if i + j < n:
                crackle[i + j] += rng.uniform(-1, 1) * 0.5 * math.exp(-j / (0.001 * RATE))
    return finish(reverb(mix(s, biquad(crackle, "hp", 1500)), 0.35, size=1.4, decay=0.82), fade=0.1)


def voice(rng, n, pitch, formants, breath, amp):
    """A vocal sound by source-filter synthesis, like a speech synthesiser:
    a sawtooth at voice pitch (with jitter, as real vocal folds never hold
    steady) plus breath noise, shaped by band-pass formants that make the
    vowel. pitch and amp are functions of time; formants is a list of
    (frequency, bandwidth, gain) where frequency may also be a function."""
    src, phase, wob = [], 0.0, 0.0
    for i in range(n):
        t = i / RATE
        wob += 0.05 * (rng.uniform(-1, 1) - wob)  # slow random pitch drift
        phase += pitch(t) * (1 + 0.04 * wob) / RATE
        saw = 1 - 2 * (phase % 1.0)
        a = amp(t)
        src.append(a * (saw + breath * rng.uniform(-1, 1)))
    out = [0.0] * n
    for f, bw, gain in formants:
        q = (f(0) if callable(f) else f) / bw
        for i, y in enumerate(biquad(src, "bp", f, q=q)):
            out[i] += gain * y
    return out


# Formants (Hz) of the vowels a pained shout moves through.
VOWELS = {"ah": (750, 1150, 2500), "uh": (620, 1200, 2450), "eh": (560, 1750, 2500)}


def hurt(rng, v):
    """The marine cries out in pain: a short "ugh!" whose pitch jumps up on the
    hit and falls away, gliding from an open vowel to a closed one, heard
    through the helmet radio, over a small impact thud."""
    n = int(0.42 * RATE)
    f0 = (118, 135, 104)[v]
    start, end = VOWELS[("ah", "eh", "ah")[v]], VOWELS["uh"]
    pitch = lambda t: f0 * (0.9 + 0.55 * math.exp(-((t - 0.035) / 0.03) ** 2) + 0.45 * math.exp(-t / 0.12)) * (1 - 0.35 * t)
    amp = lambda t: min(1.0, t / 0.012) * (0.25 + 0.75 * math.exp(-t / 0.09)) * math.exp(-t / 0.25)
    glide = lambda a, b: (lambda t: b + (a - b) * math.exp(-t / 0.1))
    formants = [(glide(start[0], end[0]), 90, 1.0), (glide(start[1], end[1]), 110, 0.6), (glide(start[2], end[2]), 160, 0.3)]
    cry = voice(rng, n, pitch, formants, 0.35, amp)
    cry = biquad(biquad(drive(scale(cry, 3.0), 1.8), "hp", 300), "lp", 3400)  # helmet radio
    thud = scale(tone(n, sweep(170, 80, 0.02), 0.03), 0.35)
    return finish(mix(cry, thud), fade=0.05)


def player_death(rng, v):
    """A longer cry that breaks and sinks, then the armoured body hitting the
    deck: a heavy thud with a clank of plates."""
    n = int(1.1 * RATE)
    f0 = (112, 126, 100)[v]
    start, mid = VOWELS[("ah", "eh", "ah")[v]], VOWELS["uh"]
    pitch = lambda t: f0 * (0.75 + 0.6 * math.exp(-((t - 0.05) / 0.05) ** 2) + 0.35 * math.exp(-t / 0.3)) * (1 - 0.4 * min(t, 0.6))
    amp = lambda t: min(1.0, t / 0.015) * (0.4 + 0.6 * math.exp(-t / 0.15)) * math.exp(-t / 0.22) * (t < 0.6)
    glide = lambda a, b: (lambda t: b + (a - b) * math.exp(-t / 0.18))
    formants = [(glide(start[0], mid[0]), 90, 1.0), (glide(start[1], mid[1]), 110, 0.6), (glide(start[2], mid[2]), 160, 0.3)]
    cry = voice(rng, n, pitch, formants, 0.45, amp)
    cry = biquad(biquad(drive(scale(cry, 3.0), 1.8), "hp", 300), "lp", 3400)  # helmet radio
    fall = 0.62 + 0.03 * v
    thud = delayed(tone(n, sweep(170, 80, 0.025), 0.05), fall)
    clank = delayed(env(biquad(noise(rng, n), "bp", 1700, q=2.5), 0.03), fall + 0.01)
    body = drive(mix(scale(thud, 0.45), scale(clank, 1.4)), 1.8)
    return finish(reverb(mix(cry, body), 0.18, size=1.0), fade=0.08)


def laser(rng, v):
    """The drone's shot: a bright, falling sci-fi "pew"."""
    n = int(0.22 * RATE)
    f = sweep(2100 + 150 * v, 380, 0.045)
    zap = drive(fm(n, f, 90, 0.08, 0.07), 3.0)  # square-ish, with a little buzz
    zap = biquad(zap, "lp", 6000)
    return finish(reverb(scale(zap, 0.8), 0.18, size=0.7))


def spit(rng, v):
    """The spitter hawks up a glob: a throaty rasp that rises, then a wet pop."""
    n = int(0.3 * RATE)
    rasp = env(biquad(noise(rng, n), "bp", sweep(500, 1400 + 100 * v, 0.06), q=3.0), 0.07)
    rasp = [x * (0.6 + 0.4 * math.sin(2 * math.pi * 55 * i / RATE)) for i, x in enumerate(rasp)]
    pop = scale(delayed(tone(n, sweep(300, 700, 0.01), 0.02), 0.09), 0.6)
    return finish(drive(mix(scale(rasp, 1.5), pop, scale(bubbles(rng, n, 5, 400, 900, 0.12), 0.3)), 1.5))


def acid_splash(rng, v):
    """The glob lands: a wet splat, then acid sizzling and popping."""
    n = int(0.7 * RATE)
    splat = env(biquad(noise(rng, n), "lp", sweep(2500, 400, 0.02), q=1.0), 0.05)
    hiss = biquad(noise(rng, n), "hp", 3500)
    crackle = [1.0 if rng.random() < 0.08 else 0.3 for _ in range(n)]  # fizzing, uneven
    sizzle = [h * c * min(1.0, i / (0.03 * RATE)) * math.exp(-i / RATE / 0.25) for i, (h, c) in enumerate(zip(hiss, crackle))]
    pops = scale(bubbles(rng, n, 14, 500, 1300, 0.45), 0.35)
    return finish(mix(splat, scale(sizzle, 0.45), pops), fade=0.08)


def crawler_death(rng, v):
    """A falling, raspy screech over a wet splat."""
    n = int(0.42 * RATE)
    screech = scale(fm(n, sweep(1300 + 80 * v, 450, 0.22), 47 + 5 * v, 0.35, 0.16), 0.6)
    screech = biquad(screech, "bp", 1200, q=0.6)
    splat = env(biquad(noise(rng, n), "lp", sweep(2000, 300, 0.03), q=1.2), 0.07)
    return finish(drive(mix(scale(screech, 1.6), splat), 1.8))


def spitter_death(rng, v):
    """Lower and longer than the crawler: a groan that sinks into a gurgle."""
    n = int(0.8 * RATE)
    groan = biquad(fm(n, sweep(520 + 40 * v, 120, 0.25), 23 + 3 * v, 0.5, 0.3), "lp", 1400)
    gurgle = scale(bubbles(rng, n, 26, 220, 700, 0.6), 0.5)
    splat = env(biquad(noise(rng, n), "lp", sweep(1500, 200, 0.05), q=1.2), 0.1)
    return finish(drive(mix(scale(groan, 1.2), gurgle, splat), 1.6), fade=0.06)


def drone_death(rng, v):
    """Electronics failing (a stuttering, falling whine) and a small blast."""
    n = int(0.9 * RATE)
    whine = tone(n, sweep(1600 + 100 * v, 90, 0.18), 0.25)
    whine = [x * (1.0 if int(i / RATE * 34) % 2 == 0 else 0.25) for i, x in enumerate(whine)]
    whine = biquad(drive(whine, 3.0), "lp", 4000)
    blast = boom(rng, n, 0.06, 0.12, 0.4, 2.4)
    return finish(reverb(mix(scale(whine, 0.35), blast), 0.25, size=1.1), fade=0.08)


# name: (generator(rng, variant) -> samples, loudness). Loudness is the RMS of
# the loudest 50 ms above 200 Hz, i.e. roughly how loud it sounds on laptop
# speakers, so these numbers compare directly; peaks are capped at 0.9.
SOUNDS = {
    "rifle": (lambda rng, v: gun(rng, 0.38, 0.8, sweep(7000 - 400 * v, 1100, 0.018), 0.03,
                                 sweep(200 - 10 * v, 70, 0.01), 0.02, 0.6, 2.6, 0.22, 1.0), 0.20),
    "pistol": (lambda rng, v: gun(rng, 0.32, 1.0, sweep(9000 - 500 * v, 1800, 0.012), 0.022,
                                  sweep(260 - 15 * v, 100, 0.008), 0.015, 0.45, 2.2, 0.2, 0.9), 0.18),
    "shotgun": (shotgun, 0.26),
    "hit_flesh": (hit_flesh, 0.12),
    "hit_metal": (hit_metal, 0.09),
    "hit_wall": (hit_wall, 0.07),
    "hurt": (hurt, 0.22),
    "crawler_death": (crawler_death, 0.14),
    "spitter_death": (spitter_death, 0.16),
    "drone_death": (drone_death, 0.20),
    "explosion": (explosion, 0.30),
    "player_death": (player_death, 0.24),
    "laser": (laser, 0.15),
    "spit": (spit, 0.12),
    "acid_splash": (acid_splash, 0.13),
}


def loudness(samples):
    audible = biquad(samples, "hp", 200)
    w = int(0.05 * RATE)
    sq = [x * x for x in audible] + [0.0] * w
    run, best = sum(sq[:w]), 0.0
    for i in range(len(audible)):
        best = max(best, run)
        run += sq[i + w] - sq[i]
    return math.sqrt(best / w)


def write(name, samples, target):
    k = min(target / (loudness(samples) or 1.0), 0.9 / (max(abs(s) for s in samples) or 1.0))
    with wave.open(name, "wb") as w:
        w.setnchannels(1)
        w.setsampwidth(2)
        w.setframerate(RATE)
        w.writeframes(b"".join(struct.pack("<h", int(s * k * 32767)) for s in samples))


def main():
    for i, (name, (make, peak)) in enumerate(SOUNDS.items()):
        for v in range(VARIANTS):
            rng = random.Random(i * 100 + v)
            write(f"{name}_{v}.wav", make(rng, v), peak)
    print(f"sounds: {len(SOUNDS)} x {VARIANTS} variants")


if __name__ == "__main__":
    main()
