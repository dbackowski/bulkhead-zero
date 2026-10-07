"""Animated previews built from the generated tiles and sprites.

  scene.gif       - a small simulated fight with free 360-degree mouse-style aiming (2x)
  animations.gif  - every animation looping side by side, labelled (3x)
  aiming.png      - the 8 body facings, each with the rifle aimed that way (5x)

The player-compositing code here (draw_player / gun_transform) is the same
logic the Go game needs, so it doubles as a reference implementation.
"""
import math
import random
from PIL import Image, ImageDraw, ImageOps

import make_tiles as mt
import make_sprites as ms

F = 24
TICK = 50  # ms per GIF frame
DEBUG = False


def frame_at(frames, frame_ms, t, loop=True):
    i = int(t // frame_ms)
    return frames[i % len(frames)] if loop else frames[min(i, len(frames) - 1)]


def to_gif(frames, path, scale, duration):
    out = []
    for f in frames:
        f = f.convert("RGB").resize((f.width * scale, f.height * scale), Image.NEAREST)
        out.append(f.quantize(colors=255, method=Image.Quantize.MEDIANCUT, dither=Image.Dither.NONE))
    out[0].save(path, save_all=True, append_images=out[1:], duration=duration, loop=0, optimize=False)


# ---------------------------------------------------------------------------
# Player compositing: body frame + weapon rotated around the hand.
# ---------------------------------------------------------------------------
WEAPONS = ms.weapons()


def hand_world(x, y, facing, bob):
    """World position of the gun pivot for a player whose feet are at (x, y)."""
    ax, ay = ms.HAND_ANCHOR[facing]
    return x - 12 + ax, y - 21 + ay + bob


def muzzle_world(hx, hy, weapon, angle):
    _, info = WEAPONS[weapon]
    mx = info["muzzle"][0] - info["pivot"][0]
    my = info["muzzle"][1] - info["pivot"][1]
    if math.cos(angle) < 0:
        my = -my
    c, s = math.cos(angle), math.sin(angle)
    return hx + mx * c - my * s, hy + mx * s + my * c


def draw_gun(canvas, weapon, hx, hy, angle, recoil=0):
    img, info = WEAPONS[weapon]
    px_, py_ = info["pivot"]
    if math.cos(angle) < 0:                     # never upside down
        img = ImageOps.flip(img)
        py_ = img.height - 1 - py_
    big = Image.new("RGBA", (48, 48), (0, 0, 0, 0))
    big.alpha_composite(img, (24 - px_, 24 - py_))
    big = big.rotate(-math.degrees(angle), resample=Image.NEAREST, center=(24, 24))
    hx -= math.cos(angle) * recoil
    hy -= math.sin(angle) * recoil
    canvas.alpha_composite(big, (int(round(hx - 24)), int(round(hy - 24))))


def draw_player(canvas, x, y, angle, anim, t, weapon, recoil=0, hurt=None):
    facing = ms.facing_for_angle(angle)
    frames_key = anim
    i = int(t // ms.timing_for(anim))
    bob = (ms.WALK_BOB if anim == "walk" else ms.IDLE_BOB)[i % 4]
    if hurt is not None:
        body = ms.player_frame("hurt", facing, min(int(hurt // 80), 1))
        bob = 0
    else:
        body = ms.player_frame(frames_key, facing, i)
    hx, hy = hand_world(x, y, facing, bob)
    behind = facing in ms.GUN_BEHIND_BODY
    if behind:
        draw_gun(canvas, weapon, hx, hy, angle, recoil)
    canvas.alpha_composite(body, (int(round(x - 12)), int(round(y - 21))))
    if not behind:
        draw_gun(canvas, weapon, hx, hy, angle, recoil)
    return hx, hy


# ---------------------------------------------------------------------------
SCENE_MAP = [
    "####################",
    "#L#P##L#DL##P#L#####",
    "#..c...v.....k.k..##",
    "#..................#",
    "#..................#",
    "#..x......s........#",
    "#..................#",
    "#....g..........B.x#",
    "#.###.....h........#",
    "#.#L#.....h........#",
    "#...........z......#",
    "####################",
]


def is_solid(x, y):
    tx, ty = int(x // F), int(y // F)
    if ty < 0 or ty >= len(SCENE_MAP) or tx < 0 or tx >= len(SCENE_MAP[0]):
        return True
    return SCENE_MAP[ty][tx] in mt.WALL_CHARS


def angle_lerp(a, b, k):
    d = (b - a + math.pi) % (2 * math.pi) - math.pi
    return a + d * k


class Enemy:
    def __init__(self, kind, x, y, hp, t0=0):
        self.kind, self.x, self.y, self.hp = kind, x, y, hp
        self.t0 = t0           # spawn time
        self.dead_at = None
        self.flash_until = -1
        self.attack_at = None
        self.next_attack = t0 + 1200
        self.face_left = False


def scene():
    rng = random.Random(4)
    tiles = dict(mt.build_tiles())
    room = mt.render_room(SCENE_MAP, tiles)
    A = ms.build_all()
    C, S, D, X = A["crawler"], A["spitter"], A["drone"], A["fx"]

    total = 11000
    loadout = [(0, "rifle"), (4000, "shotgun"), (7500, "pistol")]
    spawns = [(400, 440, 150), (1600, 60, 90), (2900, 430, 230), (4200, 60, 230),
              (5200, 440, 100), (6300, 80, 140), (7400, 300, 250), (8500, 440, 210)]
    pending = list(spawns)
    enemies = [Enemy("drone", 330, 80, 7, 0), Enemy("spitter", 400, 248, 6, 0)]
    enemies[1].next_attack = 1500
    enemies[0].next_attack = 700
    bullets, fx, globs = [], [], []   # bullets: [x,y,vx,vy,ang,born]
    px, py = 220.0, 160.0
    aim = 0.0
    last_shot = -9999
    hurt_at = -9999
    cross = [px + 60, py]
    frames = []

    for t in range(0, total, TICK):
        weapon = [w for t0, w in loadout if t >= t0][-1]
        _, winfo = WEAPONS[weapon]
        while pending and pending[0][0] <= t:
            _, sx, sy = pending.pop(0)
            enemies.append(Enemy("crawler", sx, sy, 3, t))

        # --- player strafes on a slow loop, pausing now and then
        moving = (t % 3200) < 2300
        if moving:
            k = t / 6000 * 2 * math.pi
            tx_, ty_ = 220 + math.cos(k) * 46, 160 + math.sin(k * 2) * 26
            px += (tx_ - px) * 0.12
            py += (ty_ - py) * 0.12

        # --- aim: crosshair glides to the nearest living enemy (like a good player)
        alive = [e for e in enemies if e.dead_at is None]
        if alive:
            # crawlers are the urgent threat; otherwise take the nearest target
            rushers = [e for e in alive if e.kind == "crawler" and math.dist((e.x, e.y), (px, py)) < 120]
            tgt = min(rushers or alive, key=lambda e: math.dist((e.x, e.y), (px, py)))
            cx_t, cy_t = tgt.x, tgt.y - (12 if tgt.kind == "drone" else 6)
        else:
            k = t / 900
            cx_t, cy_t = px + math.cos(k) * 60, py + math.sin(k) * 40
        cross[0] += (cx_t - cross[0]) * 0.22
        cross[1] += (cy_t - cross[1]) * 0.22
        facing = ms.facing_for_angle(aim)
        bob_guess = 0
        hx, hy = hand_world(px, py, facing, bob_guess)
        aim = angle_lerp(aim, math.atan2(cross[1] - hy, cross[0] - hx), 0.5)

        # --- fire
        on_target = alive and abs(((math.atan2(cy_t - hy, cx_t - hx) - aim + math.pi) % (2 * math.pi)) - math.pi) < 0.35
        if on_target and t - last_shot >= winfo["fire_ms"]:
            last_shot = t
            mx, my = muzzle_world(hx, hy, weapon, aim)
            for p in range(winfo["pellets"]):
                spread = math.radians(winfo["spread_deg"])
                a = aim + (rng.uniform(-spread, spread) if winfo["pellets"] == 1 else
                           -spread + 2 * spread * p / max(winfo["pellets"] - 1, 1))
                v = winfo["bullet_speed"] / 1000.0
                bullets.append([mx, my, math.cos(a) * v, math.sin(a) * v, a, t])

        # --- bullets
        nb = []
        for b in bullets:
            # move in small sub-steps so fast bullets can't tunnel through enemies
            hit = None
            steps = 4
            for _ in range(steps):
                b[0] += b[2] * TICK / steps
                b[1] += b[3] * TICK / steps
                for e in enemies:
                    if e.dead_at is not None:
                        continue
                    ey = e.y - (12 if e.kind == "drone" else 7)
                    if math.dist((b[0], b[1]), (e.x, ey)) < 10:
                        hit = e
                        break
                if hit or is_solid(b[0], b[1] + 6):
                    break
            if hit:
                hit.hp -= 1
                hit.flash_until = t + 60
                fx.append(("hit_spark", b[0], b[1], t, 0))
                if hit.hp <= 0:
                    hit.dead_at = t
                    if DEBUG:
                        print("killed", hit.kind, "at", t)
                    if hit.kind == "drone":
                        fx.append(("explosion", hit.x, hit.y - 12, t, 0))
            elif is_solid(b[0], b[1] + 6) or t - b[5] > 1500:
                fx.append(("hit_spark", b[0], b[1], t, 0))
            else:
                nb.append(b)
        bullets = nb

        # --- enemies
        for e in enemies:
            if e.dead_at is not None:
                continue
            if e.kind == "crawler":
                d = math.dist((e.x, e.y), (px, py))
                e.face_left = px < e.x
                if d > 15:
                    sp = 0.036 * TICK
                    e.x += (px - e.x) / d * sp
                    e.y += (py - e.y) / d * sp
                    e.attack_at = None
                elif e.attack_at is None or t - e.attack_at > 700:
                    e.attack_at = t
                    hurt_at = t + 180
            elif e.kind == "drone" and t >= e.next_attack:
                e.attack_at = t
                e.next_attack = t + 2600
                globs.append(("laser", e.x, e.y - 12, px, py - 12, t + 180, 260))
            elif e.kind == "spitter" and t >= e.next_attack:
                e.attack_at = t
                e.next_attack = t + 2300
                globs.append(("acid", e.x, e.y - 10, px + rng.uniform(-14, 14), py + 4, t + 270, 700))

        # ---- compose
        fr = room.copy()
        layer = []
        for e in enemies:
            if e.kind == "crawler":
                if e.dead_at is not None:
                    img = frame_at(C["death"], 130, t - e.dead_at, loop=False)
                elif e.attack_at is not None and t - e.attack_at < 360:
                    img = frame_at(C["attack_right"], 90, t - e.attack_at, loop=False)
                else:
                    img = frame_at(C["walk_right"], 90, t - e.t0)
                if e.face_left:
                    img = ImageOps.mirror(img)
            elif e.kind == "drone":
                if e.dead_at is not None:
                    img = frame_at(D["death"], 130, t - e.dead_at, loop=False)
                elif e.attack_at is not None and t - e.attack_at < 360:
                    img = frame_at(D["attack"], 90, t - e.attack_at, loop=False)
                else:
                    img = frame_at(D["idle"], 180, t)
            else:
                if e.dead_at is not None:
                    img = frame_at(S["death"], 130, t - e.dead_at, loop=False)
                elif e.attack_at is not None and t - e.attack_at < 450:
                    img = frame_at(S["attack"], 90, t - e.attack_at, loop=False)
                else:
                    img = frame_at(S["idle"], 180, t)
            if e.dead_at is None and t < e.flash_until:
                img = ms.tint(img, ms.WHITE, .8)
            sort_y = e.y - (8 if e.dead_at is not None and e.kind == "crawler" else 0)
            layer.append((sort_y, "img", img, e.x, e.y))
        layer.append((py, "player", None, px, py))
        for sort_y, kind, img, x, y in sorted(layer, key=lambda l: l[0]):
            if kind == "img":
                fr.alpha_composite(img, (int(round(x - 12)), int(round(y - 21))))
            else:
                recoil = winfo["recoil_px"] if t - last_shot < 70 else 0
                hurt = t - hurt_at if 0 <= t - hurt_at < 160 else None
                hx, hy = draw_player(fr, x, y, aim, "walk" if moving else "idle", t, weapon,
                                     recoil=recoil, hurt=hurt)
                if t - last_shot < 80:
                    mx, my = muzzle_world(hx, hy, weapon, aim)
                    fl = frame_at(X["muzzle_flash"], 40, t - last_shot, loop=False)
                    fr.alpha_composite(fl, (int(round(mx - 12)), int(round(my - 12))))

        # enemy projectiles
        for g in globs:
            kind, sx, sy, ex, ey, t0, dur = g
            k = (t - t0) / dur
            if 0 <= k < 1:
                if kind == "laser":
                    ang = math.degrees(math.atan2(-(ey - sy), ex - sx))
                    im = frame_at(X["laser"], 50, t).rotate(ang, resample=Image.NEAREST, center=(12, 12))
                    gx, gy = sx + (ex - sx) * k, sy + (ey - sy) * k
                else:
                    im = frame_at(X["acid_glob"], 90, t)
                    gx = sx + (ex - sx) * k
                    gy = sy + (ey - sy) * k - math.sin(k * math.pi) * 36
                fr.alpha_composite(im, (int(round(gx - 12)), int(round(gy - 12))))
            elif 1 <= k and t - (t0 + dur) < TICK:
                if kind == "laser":
                    fx.append(("hit_spark", ex, ey, t, 0))
                    hurt_at = t
                else:
                    fx.append(("acid_splash", ex, ey, t, 0))

        for b in bullets:
            im = frame_at(X["bullet"], 50, t).rotate(-math.degrees(b[4]), resample=Image.NEAREST, center=(12, 12))
            fr.alpha_composite(im, (int(round(b[0] - 12)), int(round(b[1] - 12))))
        keep = []
        for name, x, y, t0, _ in fx:
            frames_ = X[name]
            dur = len(frames_) * ms.timing_for(name)
            if t - t0 < dur:
                fr.alpha_composite(frame_at(frames_, ms.timing_for(name), t - t0, loop=False),
                                   (int(round(x - 12)), int(round(y - 12))))
                keep.append((name, x, y, t0, _))
        fx = keep
        # crosshair + HUD
        fr.alpha_composite(frame_at(X["crosshair"], 300, t), (int(round(cross[0] - 12)), int(round(cross[1] - 12))))
        hud = ImageDraw.Draw(fr)
        img, _ = WEAPONS[weapon]
        hud.rectangle((4, 268, 92, 284), fill=(12, 14, 20, 255), outline=(70, 214, 230, 255))
        fr.alpha_composite(img, (6, 270))
        hud.text((34, 270), weapon.upper(), fill=(238, 176, 58, 255))
        frames.append(fr)

    to_gif(frames, "scene.gif", 2, TICK)
    frames[int(5200 / TICK)].resize((room.width * 3, room.height * 3), Image.NEAREST).save("scene_still_x3.png")


# ---------------------------------------------------------------------------
def aiming_sheet():
    """8 facings x idle, with the rifle aimed in each facing's direction."""
    S = 5
    cols = 8
    img = Image.new("RGBA", (cols * 40 * S, 2 * 40 * S), (34, 38, 50, 255))
    floor = dict(mt.build_tiles())["floor_plate_a"]
    for row, weapon in enumerate(("rifle", "shotgun")):
        for c, facing in enumerate(ms.FACINGS_ALL):
            cell = Image.new("RGBA", (40, 40), (0, 0, 0, 0))
            for yy in (0, 24):
                for xx in (0, 24):
                    cell.alpha_composite(floor.crop((0, 0, min(24, 40 - xx), min(24, 40 - yy))), (xx, yy))
            ang = math.radians(c * 45 + (row * 20 - 10))
            draw_player(cell, 20, 30, ang, "idle", 0, weapon)
            img.alpha_composite(cell.resize((40 * S, 40 * S), Image.NEAREST), (c * 40 * S, row * 40 * S))
    img.save("aiming.png")


def showcase():
    A = ms.build_all()
    tiles = dict(mt.build_tiles())
    cells = []
    for name, frames in A["player"].items():
        cells.append((name, frames, ms.timing_for(name)))
    for group in ("crawler", "spitter", "drone"):
        for name, frames in A[group].items():
            cells.append((f"{group} {name.replace('_right', '')}", frames, ms.timing_for(name)))
    for name, frames in A["fx"].items():
        cells.append((name, frames, ms.timing_for(name)))
    cols = 8
    Sc = 3
    cw, ch = F * Sc + 16, F * Sc + 22
    rows = (len(cells) + cols - 1) // cols + 1     # +1 row for spinning weapons
    W, H = cols * cw, rows * ch
    bg = Image.new("RGBA", (W, H), (22, 24, 32, 255))
    floor = tiles["floor_plate_a"].resize((F * Sc, F * Sc), Image.NEAREST)
    d = ImageDraw.Draw(bg)
    labels = [c[0] for c in cells] + [f"{w} (rotating)" for w in WEAPONS]
    for i, label in enumerate(labels):
        x, y = (i % cols) * cw + 8, (i // cols) * ch + 4
        bg.alpha_composite(floor, (x, y))
        d.text((x, y + F * Sc + 3), label, fill=(200, 210, 220, 255))
    total = 2880
    out = []
    for t in range(0, total, TICK):
        fr = bg.copy()
        for i, (label, frames, ms_) in enumerate(cells):
            n = len(frames)
            looping = n > 1 and any(k in label for k in ("idle", "walk", "bullet", "laser", "glob", "crosshair"))
            cycle = n * ms_ + (0 if looping else 500)
            im = frames[min(n - 1, int((t % cycle) // ms_))]
            x, y = (i % cols) * cw + 8, (i // cols) * ch + 4
            fr.alpha_composite(im.resize((F * Sc, F * Sc), Image.NEAREST), (x, y))
        for j, w in enumerate(WEAPONS):
            i = len(cells) + j
            x, y = (i % cols) * cw + 8, (i // cols) * ch + 4
            cell = Image.new("RGBA", (F, F), (0, 0, 0, 0))
            draw_gun(cell, w, 12, 12, t / total * 2 * math.pi)
            fr.alpha_composite(cell.resize((F * Sc, F * Sc), Image.NEAREST), (x, y))
        out.append(fr)
    to_gif(out, "animations.gif", 1, TICK)


if __name__ == "__main__":
    aiming_sheet()
    scene()
    showcase()
    print("ok")
