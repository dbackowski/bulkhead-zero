#!/usr/bin/env python3
"""Sound effects for Bulkhead Zero, synthesised from noise and sine waves.

Writes 16-bit mono 44.1 kHz WAVs named <sound>_<variant>.wav into the
current directory (build.py moves them to assets/sounds/). The game picks a
random variant each time, which stands in for pitch variation. Standard
library only; seeded, so the output is identical on every run.
"""
import math
import random
import struct
import wave

RATE = 44100
VARIANTS = 3


def lowpass(xs, a):
    """One-pole low-pass: a near 0 is dark, a near 1 lets everything through."""
    y, out = 0.0, []
    for x in xs:
        y += a * (x - y)
        out.append(y)
    return out


def gun(rng, length, tau, cutoff, f0, f1, thump_tau, thump_amp):
    """A shot: a filtered noise burst, a bright click and a falling low thump."""
    n = int(length * RATE)
    noise = lowpass([rng.uniform(-1, 1) for _ in range(n)], cutoff)
    out, phase = [], 0.0
    for i in range(n):
        t = i / RATE
        f = f1 + (f0 - f1) * math.exp(-t / 0.03)  # pitch drops fast, then settles
        phase += 2 * math.pi * f / RATE
        s = noise[i] * 2.2 * math.exp(-t / tau)
        s += thump_amp * math.sin(phase) * math.exp(-t / thump_tau)
        if t < 0.002:
            s += rng.uniform(-1, 1)  # the crack at the very start
        out.append(s * min(1.0, i / 20))  # 0.5 ms fade-in, no pop
    return out


def hit(rng, pitch):
    """A short ricochet tick: high-passed noise plus a quick ping."""
    n = int(0.07 * RATE)
    noise = [rng.uniform(-1, 1) for _ in range(n)]
    low = lowpass(noise, 0.15)
    out = []
    for i in range(n):
        t = i / RATE
        s = (noise[i] - low[i]) * math.exp(-t / 0.012)
        s += 0.4 * math.sin(2 * math.pi * 1900 * pitch * t) * math.exp(-t / 0.02)
        out.append(s)
    return out


SOUNDS = {
    # name: (generator(rng, variant) -> samples, peak level)
    "rifle": (lambda rng, v: gun(rng, 0.18, 0.035, 0.35 + 0.05 * v, 120 + 15 * v, 60, 0.04, 0.8), 0.70),
    "pistol": (lambda rng, v: gun(rng, 0.14, 0.025, 0.50 + 0.05 * v, 180 + 20 * v, 90, 0.03, 0.6), 0.60),
    "shotgun": (lambda rng, v: gun(rng, 0.45, 0.090, 0.20 + 0.03 * v, 90 + 10 * v, 40, 0.09, 1.0), 0.90),
    "hit": (lambda rng, v: hit(rng, 0.9 + 0.1 * v), 0.35),
}


def write(name, samples, peak):
    top = max(abs(s) for s in samples) or 1.0
    k = peak / top
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
