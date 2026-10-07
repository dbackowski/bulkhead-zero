"""Procedural 24x24 character / enemy / FX sprites to match make_tiles.py.

Run directly to write:
  player.png, enemies.png, fx.png   - sprite sheets (one animation per row)
  sprites.json                      - row/frame/timing metadata per animation
  sprites_x4.png                    - all three sheets stacked, 4x, for viewing

Import and call build_all() to get {sheet: {anim: [frames]}} for previews.
"""
import json
import math
import random
from PIL import Image, ImageOps

F = 24  # frame size

# ---- palette (shares darks/metal/accents with the tileset) -----------------
OUT = (14, 16, 22, 255)
CLEAR = (0, 0, 0, 0)
SHADOW = (0, 0, 0, 96)
WHITE = (240, 248, 255, 255)

A0, A1, A2, A3 = (40, 58, 36, 255), (70, 96, 52, 255), (108, 140, 72, 255), (160, 186, 106, 255)
U0, U1 = (30, 32, 40, 255), (56, 60, 74, 255)
G0, G1, G2, G3 = (36, 38, 46, 255), (72, 78, 92, 255), (120, 128, 144, 255), (176, 184, 198, 255)
CY, CY_D, CY_DD = (90, 226, 240, 255), (30, 130, 150, 255), (18, 62, 76, 255)
AMB, AMB_D = (238, 176, 58, 255), (140, 90, 28, 255)
RED, RED_D, RED_DD = (232, 68, 60, 255), (138, 30, 36, 255), (70, 14, 20, 255)
FLASH1, FLASH2 = (255, 250, 200, 255), (255, 200, 70, 255)

X0, X1, X2, X3 = (34, 20, 44, 255), (74, 40, 88, 255), (122, 66, 132, 255), (182, 116, 176, 255)
BONE = (226, 214, 184, 255)
ACID, ACID_D, ACID_L = (150, 232, 60, 255), (70, 132, 30, 255), (220, 255, 160, 255)

M0, M1, M2, M3 = (72, 30, 36, 255), (132, 62, 58, 255), (190, 104, 86, 255), (232, 158, 122, 255)


# ---- tiny raster helpers ---------------------------------------------------
def new(w=F, h=F):
    return Image.new("RGBA", (w, h), CLEAR)


def put(img, x, y, c):
    x, y = int(round(x)), int(round(y))
    if 0 <= x < img.width and 0 <= y < img.height:
        img.putpixel((x, y), c)


def rect(img, x0, y0, x1, y1, c):
    for y in range(int(y0), int(y1) + 1):
        for x in range(int(x0), int(x1) + 1):
            put(img, x, y, c)


def ellipse(img, cx, cy, rx, ry, c):
    for y in range(int(cy - ry - 1), int(cy + ry + 2)):
        for x in range(int(cx - rx - 1), int(cx + rx + 2)):
            if ((x - cx) / max(rx, .1)) ** 2 + ((y - cy) / max(ry, .1)) ** 2 <= 1.0:
                put(img, x, y, c)


def line(img, x0, y0, x1, y1, c):
    n = int(max(abs(x1 - x0), abs(y1 - y0))) + 1
    for i in range(n):
        t = i / max(n - 1, 1)
        put(img, x0 + (x1 - x0) * t, y0 + (y1 - y0) * t, c)


def outline(img, c=OUT):
    """1px outline around everything opaque - this is what makes procedural
    shapes read as pixel art."""
    out = img.copy()
    w, h = img.size
    src = img.load()
    dst = out.load()
    for y in range(h):
        for x in range(w):
            if src[x, y][3] > 0:
                continue
            for dx, dy in ((1, 0), (-1, 0), (0, 1), (0, -1)):
                nx, ny = x + dx, y + dy
                if 0 <= nx < w and 0 <= ny < h and src[nx, ny][3] > 160:
                    dst[x, y] = c
                    break
    return out


def shift(img, dx, dy):
    out = new(img.width, img.height)
    out.paste(img, (dx, dy), img)
    return out


def with_shadow(sprite, cx=12, cy=21, rx=6, ry=1.6):
    base = new()
    ellipse(base, cx, cy, rx, ry, SHADOW)
    base.alpha_composite(sprite)
    return base


def tint(img, c, amount=1.0):
    out = img.copy()
    p = out.load()
    for y in range(out.height):
        for x in range(out.width):
            r, g, b, a = p[x, y]
            if a > 0:
                p[x, y] = (int(r + (c[0] - r) * amount), int(g + (c[1] - g) * amount),
                           int(b + (c[2] - b) * amount), a)
    return out


def fade(img, k):
    out = img.copy()
    p = out.load()
    for y in range(out.height):
        for x in range(out.width):
            r, g, b, a = p[x, y]
            p[x, y] = (r, g, b, int(a * k))
    return out


def flash_star(img, cx, cy, size):
    """Muzzle flash / spark: a cross with a hot core."""
    if size <= 0:
        return
    for i in range(-size, size + 1):
        put(img, cx + i, cy, FLASH2)
        put(img, cx, cy + i, FLASH2)
    for i in range(-(size // 2), size // 2 + 1):
        put(img, cx + i, cy + i, FLASH2)
        put(img, cx + i, cy - i, FLASH2)
    rect(img, cx - 1, cy - 1, cx + 1, cy + 1, FLASH1)
    put(img, cx, cy, WHITE)


# ============================================================================
# PLAYER - space marine body WITHOUT a gun, 8 facings.
# Drawn facings: down, down_right, right, up_right, up.
# down_left / left / up_left = the matching *_right frames flipped horizontally.
# The weapon is a separate sprite rotated around HAND_ANCHOR (see weapons below).
# ============================================================================
FACINGS_DRAWN = ("down", "down_right", "right", "up_right", "up")
FACINGS_ALL = ("right", "down_right", "down", "down_left", "left", "up_left", "up", "up_right")

# where the gun pivot sits in the 24x24 body frame (add the frame's bob to y)
HAND_ANCHOR = {
    "down": (15, 14), "down_right": (15, 13), "right": (13, 13),
    "up_right": (15, 11), "up": (15, 11),
}
for _f in ("down_right", "right", "up_right"):
    _x, _y = HAND_ANCHOR[_f]
    HAND_ANCHOR[_f.replace("right", "left")] = (F - 1 - _x, _y)
GUN_BEHIND_BODY = ("up", "up_left", "up_right")


def _helmet(body, x0, hy, visor=None, crest=None, side_shade=None):
    rect(body, x0, hy, x0 + 5, hy + 6, A1)
    rect(body, x0, hy, x0 + 5, hy + 1, A2)
    rect(body, x0 + 1, hy, x0 + 4, hy, A3)
    put(body, x0, hy, CLEAR); put(body, x0 + 5, hy, CLEAR)
    if side_shade is not None:
        vline_ = lambda x: rect(body, x, hy + 1, x, hy + 6, A0)
        vline_(side_shade)
    if crest is not None:
        rect(body, crest, hy, crest + 1, hy + 5, A2)
    if visor is not None:
        vx0, vx1 = visor
        rect(body, vx0, hy + 3, vx1, hy + 4, CY)
        rect(body, vx0, hy + 4, vx1, hy + 4, CY_D)
        put(body, vx0 if vx1 - vx0 > 0 else vx1, hy + 3, WHITE)
    rect(body, x0 + 1, hy + 6, x0 + 4, hy + 6, A0)


def _pad(body, x0, x1, y):
    rect(body, x0, y, x1, y + 3, A2)
    rect(body, x0, y, x1, y, A3)


def _backpack(body, x0, x1, y0, y1):
    rect(body, x0, y0, x1, y1, G1)
    rect(body, x0, y0, x1, y0, G2)
    for vy in range(y0 + 2, y1, 2):
        rect(body, x0 + 1, vy, x1 - 1, vy, G0)
    put(body, x1 - 1, y0 + 1, AMB)


def marine_body(facing, legs=(0, 0), bob=0):
    body = new()
    by = bob

    # ---------- legs
    if facing == "right":
        far, near = legs
        rect(body, 9 + far, 15, 10 + far, 20, U0)
        rect(body, 8 + far, 19, 10 + far, 20, A0)
        rect(body, 12 + near, 15, 13 + near, 20, U1)
        rect(body, 12 + near, 19, 14 + near, 20, A0)
        put(body, 13 + near, 16, A2)
    else:
        diag = facing in ("down_right", "up_right")
        xs = (8, 12) if diag else (8, 13)
        for i, (lx, off) in enumerate(zip(xs, legs)):
            lx += off if diag else 0           # diagonal stride drifts sideways too
            bottom = 20 + off
            rect(body, lx, 15, lx + 2, bottom, U1 if not (diag and i == 0) else U0)
            rect(body, lx, bottom - 1, lx + 2, bottom, A0)
            rect(body, lx, 16 + max(off, 0) // 2, lx + 2, 16 + max(off, 0) // 2, A2)

    # ---------- torso, arms, pads, helmet
    if facing == "down":
        rect(body, 8, 9 + by, 15, 15 + by, A1)
        rect(body, 9, 10 + by, 14, 13 + by, A2)
        rect(body, 9, 10 + by, 14, 10 + by, A3)
        rect(body, 8, 14 + by, 15, 14 + by, U0)
        put(body, 11, 14 + by, AMB); put(body, 12, 14 + by, AMB_D)
        rect(body, 6, 11 + by, 7, 14 + by, U1)
        rect(body, 16, 11 + by, 17, 14 + by, U1)
        _pad(body, 5, 8, 8 + by); _pad(body, 15, 18, 8 + by)
        _helmet(body, 9, 2 + by, visor=(10, 13))

    elif facing == "down_right":
        rect(body, 8, 9 + by, 15, 15 + by, A1)
        rect(body, 10, 10 + by, 15, 13 + by, A2)
        rect(body, 10, 10 + by, 15, 10 + by, A3)
        rect(body, 8, 9 + by, 8, 15 + by, A0)
        rect(body, 8, 14 + by, 15, 14 + by, U0)
        put(body, 13, 14 + by, AMB)
        rect(body, 5, 9 + by, 7, 13 + by, G1)              # backpack peeks out left
        rect(body, 6, 11 + by, 7, 14 + by, U1)
        rect(body, 16, 11 + by, 16, 14 + by, U1)
        _pad(body, 5, 8, 8 + by); _pad(body, 15, 17, 8 + by)
        _helmet(body, 9, 2 + by, visor=(11, 14), side_shade=9)

    elif facing == "right":
        _backpack(body, 6, 8, 9 + by, 14 + by)
        rect(body, 9, 9 + by, 13, 15 + by, A1)
        rect(body, 12, 10 + by, 13, 13 + by, A2)
        rect(body, 9, 14 + by, 13, 14 + by, U0)
        rect(body, 10, 12 + by, 12, 14 + by, U1)           # arm hanging
        _pad(body, 9, 12, 8 + by)
        _helmet(body, 9, 2 + by, visor=(13, 14))

    elif facing == "up_right":
        rect(body, 8, 9 + by, 15, 15 + by, A1)
        rect(body, 14, 10 + by, 15, 13 + by, A2)
        rect(body, 8, 14 + by, 15, 14 + by, U0)
        _backpack(body, 7, 12, 9 + by, 14 + by)
        rect(body, 6, 11 + by, 6, 14 + by, U1)
        rect(body, 16, 11 + by, 17, 14 + by, U1)
        _pad(body, 6, 8, 8 + by); _pad(body, 14, 18, 8 + by)
        _helmet(body, 9, 2 + by, crest=10)
        rect(body, 14, 2 + by + 3, 14, 2 + by + 4, CY_D)     # edge of the visor
        put(body, 12, 1 + by, G2)                          # antenna

    elif facing == "up":
        rect(body, 8, 9 + by, 15, 15 + by, A1)
        rect(body, 8, 14 + by, 15, 14 + by, U0)
        _backpack(body, 9, 14, 9 + by, 14 + by)
        rect(body, 6, 11 + by, 7, 14 + by, U1)
        rect(body, 16, 11 + by, 17, 14 + by, U1)
        _pad(body, 5, 8, 8 + by); _pad(body, 15, 18, 8 + by)
        _helmet(body, 9, 2 + by, crest=11)
        put(body, 13, 1 + by, G2)

    return outline(body)


WALK_BOB = [0, -1, 0, -1]
IDLE_BOB = [0, 0, -1, -1]


def player_anims():
    anims = {}
    walk_v = [(1, -2), (0, -1), (-2, 1), (-1, 0)]
    walk_h = [(-2, 2), (0, 0), (2, -2), (0, 0)]
    for f in FACINGS_DRAWN:
        wl = walk_h if f == "right" else walk_v
        anims[f"idle_{f}"] = [with_shadow(marine_body(f, bob=b)) for b in IDLE_BOB]
        anims[f"walk_{f}"] = [with_shadow(marine_body(f, legs=l, bob=b)) for l, b in zip(wl, WALK_BOB)]
        base = with_shadow(marine_body(f))
        anims[f"hurt_{f}"] = [tint(base, WHITE, .85), tint(shift(base, 0, -1), RED, .45)]
    # death: stagger, kneel, fall, lie in a spreading blood pool
    stand = marine_body("down")
    kneel = new()
    kneel.alpha_composite(stand.crop((0, 0, F, 15)), (0, 3))
    kneel.alpha_composite(stand.crop((0, 15, F, 21)).resize((F, 3), Image.NEAREST), (0, 18))
    lying = shift(stand.rotate(90, resample=Image.NEAREST), 0, 3)
    pool = new()
    ellipse(pool, 13, 17, 9, 4.5, RED_D)
    ellipse(pool, 15, 18, 5, 2.5, RED)
    pool2 = new()
    ellipse(pool2, 13, 17, 11, 5.5, RED_DD)
    ellipse(pool2, 13, 17, 9, 4.5, RED_D)
    ellipse(pool2, 16, 19, 4, 2, RED)
    anims["death"] = [
        with_shadow(tint(stand, WHITE, .85)),
        with_shadow(tint(shift(stand, 0, 1), RED, .3)),
        with_shadow(kneel),
        with_shadow(lying, cy=20, rx=9),
        Image.alpha_composite(pool, lying),
        Image.alpha_composite(pool2, lying),
    ]
    return anims


def player_frame(anim, facing, i):
    """Pick a body frame for any of the 8 facings, mirroring left ones."""
    A = player_anims_cached()
    if facing.endswith("left"):
        src = A[f"{anim}_{facing.replace('left', 'right')}"]
        return ImageOps.mirror(src[i % len(src)])
    src = A[f"{anim}_{facing}"]
    return src[i % len(src)]


_PA = None


def player_anims_cached():
    global _PA
    if _PA is None:
        _PA = player_anims()
    return _PA


def facing_for_angle(rad):
    """Screen angle (0 = right, +y down) -> one of FACINGS_ALL."""
    return FACINGS_ALL[int(round(math.degrees(rad) / 45.0)) % 8]


# ============================================================================
# WEAPONS - drawn pointing right. Rotate around `pivot` (the gloved hand),
# flip vertically when aiming left so the gun is never upside down.
# ============================================================================
WOOD, WOOD_D = (128, 78, 46, 255), (82, 46, 28, 255)


def _glove(img, x, y):
    rect(img, x - 1, y, x, y + 1, U1)
    put(img, x, y, A2)


def weapon_rifle():
    w = new(24, 12)
    rect(w, 1, 6, 4, 8, G0)                  # stock
    rect(w, 1, 6, 4, 6, G1)
    rect(w, 4, 5, 18, 6, G1)                 # body
    rect(w, 4, 5, 18, 5, G2)
    rect(w, 19, 5, 21, 5, G0)                # barrel
    rect(w, 9, 7, 10, 9, G0)                 # magazine
    rect(w, 12, 4, 15, 4, G0)                # sight
    put(w, 16, 6, AMB)
    _glove(w, 6, 7)
    return outline(w), {"pivot": [6, 7], "muzzle": [22, 5], "fire_ms": 120, "spread_deg": 3,
                        "pellets": 1, "recoil_px": 1, "bullet_speed": 420, "damage": 1}


def weapon_pistol():
    w = new(24, 12)
    rect(w, 5, 5, 12, 6, G2)                 # slide
    rect(w, 5, 6, 12, 6, G1)
    put(w, 13, 5, G0)
    rect(w, 5, 7, 6, 9, G0)                  # grip
    put(w, 11, 5, AMB)
    _glove(w, 6, 7)
    return outline(w), {"pivot": [6, 7], "muzzle": [14, 5], "fire_ms": 260, "spread_deg": 1,
                        "pellets": 1, "recoil_px": 1, "bullet_speed": 380, "damage": 2,
                        "semi_auto": True}


def weapon_shotgun():
    w = new(24, 12)
    rect(w, 1, 5, 6, 7, WOOD)                # wooden stock
    rect(w, 1, 7, 6, 7, WOOD_D)
    rect(w, 6, 4, 20, 4, G2)                 # double barrel
    rect(w, 6, 5, 20, 5, G1)
    rect(w, 21, 4, 21, 5, G0)
    rect(w, 12, 6, 16, 7, WOOD)              # pump
    rect(w, 12, 7, 16, 7, WOOD_D)
    rect(w, 6, 6, 9, 6, G0)
    _glove(w, 7, 7)
    return outline(w), {"pivot": [7, 7], "muzzle": [22, 5], "fire_ms": 520, "spread_deg": 18,
                        "pellets": 5, "recoil_px": 3, "bullet_speed": 400, "damage": 1}


def weapons():
    return {"rifle": weapon_rifle(), "pistol": weapon_pistol(), "shotgun": weapon_shotgun()}


def player_fx():
    a = {"muzzle_flash": [], "crosshair": []}
    for s in (3, 2, 1):
        img = new()
        flash_star(img, 12, 12, s)
        a["muzzle_flash"].append(img)
    for k in (0, 1):
        img = new()
        r = 4 + k
        for ang in range(0, 360, 10):
            if ang % 90 in (40, 50):     # leave gaps on the diagonals
                continue
            put(img, 12 + math.cos(math.radians(ang)) * r, 12 + math.sin(math.radians(ang)) * r, WHITE)
        put(img, 12, 12, RED)
        a["crosshair"].append(outline(img))
    return a


# ============================================================================
# ENEMY 1 - Xeno Crawler (melee, fast). Faces right; mirror for left.
# ============================================================================
def crawler(leg_phase=0, dx=0, bob=0, mouth=0, stretch=0, head_dip=0):
    body = new()
    cy = 13 + bob
    # far legs (darker, behind)
    for i, lx in enumerate((9, 12, 15)):
        swing = (2 if (i + leg_phase) % 2 else -1) if leg_phase >= 0 else 0
        line(body, lx + dx, cy + 1, lx + dx + swing - 1, 20, X0)
    # tail
    line(body, 3 + dx, cy - 1, 1 + dx, cy - 4, X2)
    put(body, 0 + dx, cy - 5, BONE)
    # abdomen
    ellipse(body, 7 + dx, cy, 5 + stretch * .5, 3.5, X1)
    ellipse(body, 7 + dx, cy - 1.5, 4, 1.6, X2)
    for sx in (5, 8):
        line(body, sx + dx, cy - 3, sx + dx, cy + 2, X0)
    put(body, 6 + dx, cy - 2, X3)
    # thorax + head
    tx = 13 + dx + stretch
    ellipse(body, tx, cy, 3, 2.6, X1)
    ellipse(body, tx, cy - 1, 2, 1.2, X2)
    hx, hy = tx + 4, cy - 1 + head_dip
    ellipse(body, hx, hy, 3, 2.2, X2)
    ellipse(body, hx - .5, hy - 1, 1.8, .9, X3)
    put(body, hx + 1, hy - 1, ACID)                  # eye
    put(body, hx + 1, hy - 2, ACID_L)
    # mandibles: mouth 0=closed .. 2=wide
    jaw = [1, 2, 3][mouth]
    line(body, hx + 2, hy, hx + 4, hy - jaw + 1, BONE)
    line(body, hx + 2, hy + 1, hx + 4, hy + jaw, BONE)
    if mouth:
        put(body, hx + 3, hy + 1, ACID_D)
    # near legs
    for i, lx in enumerate((10, 13, 16)):
        swing = (-2 if (i + leg_phase) % 2 else 1) if leg_phase >= 0 else 0
        mid = (lx + dx + 1, cy - 1)
        line(body, lx + dx, cy + 1, *mid, X2)
        line(body, *mid, lx + dx + swing + 1, 20, X2)
        put(body, lx + dx + swing + 1, 20, X3)
    return outline(body)


def crawler_anims():
    a = {}
    a["walk_right"] = [with_shadow(crawler(leg_phase=p, bob=b), rx=8)
                       for p, b in ((0, 0), (1, -1), (0, 0), (1, -1))]
    a["attack_right"] = [
        with_shadow(crawler(dx=-2, mouth=1, head_dip=1, leg_phase=0), rx=8),   # wind-up
        with_shadow(crawler(dx=2, mouth=2, stretch=2, leg_phase=1), rx=8),      # lunge
        with_shadow(crawler(dx=3, mouth=0, stretch=2, leg_phase=1), rx=8),      # bite
        with_shadow(crawler(dx=1, mouth=1, leg_phase=0), rx=8),                 # recover
    ]
    alive = crawler()
    belly_up = shift(ImageOps.flip(alive), 0, 8)
    pool = new()
    ellipse(pool, 12, 19, 9, 2.4, ACID_D)
    ellipse(pool, 11, 19, 6, 1.4, ACID)
    a["death"] = [
        with_shadow(tint(alive, WHITE, .85), rx=8),
        with_shadow(shift(alive, 0, -2), rx=8),
        Image.alpha_composite(pool, belly_up),
        Image.alpha_composite(pool, fade(belly_up, .55)),
    ]
    return a


# ============================================================================
# ENEMY 2 - Spitter (stationary mutant, ranged acid). Faces the camera.
# ============================================================================
def spitter(sac=4.0, mouth=0, lean=0, squash=0.0, burst=False):
    body = new()
    by = lean
    # stubby feet
    for fx in (6, 9, 14, 17):
        rect(body, fx, 19, fx + 1, 20, M0)
    # body
    ellipse(body, 12, 15 + by + squash, 7 + squash, 5.5 - squash, M1)
    ellipse(body, 10.5, 13 + by + squash, 4.5, 2.5 - squash * .5, M2)
    put(body, 9, 12 + by, M3)
    ellipse(body, 12, 18 + by, 6, 1.2, M0)
    # acid sac on the back
    if not burst:
        sy = 8 + by - (sac - 4)
        ellipse(body, 12, sy, sac, sac * .9, ACID_D)
        ellipse(body, 11, sy - 1, sac * .65, sac * .5, ACID)
        put(body, 10, sy - 2, ACID_L)
        line(body, 13, sy - sac * .6, 14, sy + 1, M0)   # veins
        line(body, 10, sy + 1, 9, sy + sac * .6, M0)
    else:
        top = int(round(10 + by + squash * 1.5))
        for tx, ty, c in ((9, 0, M0), (10, -1, ACID_D), (12, 0, M0), (13, -1, ACID_D), (15, 0, M0)):
            put(body, tx, top + ty, c)
    # eyes
    for ex in (9, 12, 15):
        put(body, ex, 12 + by + int(squash * 1.5) + (1 if ex != 12 else 0), AMB)
    # mouth
    if mouth == 0:
        line(body, 9, 16 + by, 15, 16 + by, M0)
    else:
        h = mouth
        rect(body, 9, 15 + by, 15, 15 + by + h, M0)
        rect(body, 10, 16 + by, 14, 15 + by + h, ACID_D if h > 1 else M0)
        for tx in (9, 11, 13, 15):
            put(body, tx, 15 + by, BONE)
    return outline(body)


def spitter_anims():
    a = {}
    a["idle"] = [with_shadow(spitter(sac=s), rx=8) for s in (4.0, 4.4, 4.8, 4.4)]
    a["attack"] = [
        with_shadow(spitter(sac=5.4, lean=-1), rx=8),
        with_shadow(spitter(sac=6.0, lean=-1, mouth=1), rx=8),
        with_shadow(spitter(sac=3.4, lean=1, mouth=3), rx=8),     # glob leaves here
        with_shadow(spitter(sac=3.6, mouth=2), rx=8),
        with_shadow(spitter(sac=4.0, mouth=0), rx=8),
    ]
    burst = spitter(burst=True, mouth=2)
    splat = new()
    rng = random.Random(3)
    for _ in range(14):
        ang = rng.random() * math.tau
        r = rng.uniform(4, 9)
        put(splat, 12 + math.cos(ang) * r, 8 + math.sin(ang) * r * .7, rng.choice([ACID, ACID_L, ACID_D]))
    puddle = new()
    ellipse(puddle, 12, 18, 9, 3, M0)
    ellipse(puddle, 12, 17.5, 7, 2, M1)
    ellipse(puddle, 9, 18, 3, 1, ACID_D)
    ellipse(puddle, 15, 18.5, 2, 1, ACID)
    a["death"] = [
        with_shadow(tint(spitter(sac=6.2), WHITE, .85), rx=8),
        with_shadow(Image.alpha_composite(burst, splat), rx=8),
        with_shadow(spitter(burst=True, squash=2.0, mouth=1), rx=9),
        outline(puddle),
    ]
    return a


# ============================================================================
# ENEMY 3 - Sentry Drone (hovering, ranged laser).
# ============================================================================
def drone(hover=0, eye=0, thrust=2, tilt=0):
    body = new()
    cy = 9 + hover
    # side fins
    rect(body, 4, cy - 1, 6, cy + 1, G1)
    rect(body, 17, cy - 1, 19, cy + 1, G1)
    put(body, 4, cy - 2, RED if eye else G2)
    put(body, 19, cy - 2, RED if eye else G2)
    # hull
    ellipse(body, 12, cy, 6, 5.5, G1)
    ellipse(body, 10.5, cy - 2, 3.5, 2, G2)
    put(body, 9, cy - 3, G3)
    ellipse(body, 13.5, cy + 3, 4, 1.2, G0)
    line(body, 7, cy + 1, 17, cy + 1, G0)
    # eye
    ellipse(body, 12, cy, 2.6, 2.3, U0)
    eye_c = [RED_D, RED, FLASH2, RED_D][eye]
    ellipse(body, 12, cy, 1.4 + (eye in (1, 2)) * .6, 1.3 + (eye in (1, 2)) * .5, eye_c)
    put(body, 11, cy - 1, WHITE if eye == 2 else (255, 160, 150, 255))
    # thruster
    rect(body, 11, cy + 5, 13, cy + 6, G0)
    img = outline(body)
    for i in range(thrust):
        put(img, 12, cy + 8 + i, CY if i < thrust - 1 else CY_D)
        if i == 0:
            put(img, 11, cy + 8, CY_D); put(img, 13, cy + 8, CY_D)
    if eye == 2:  # glow ring when firing
        for ang in range(0, 360, 45):
            put(img, 12 + math.cos(math.radians(ang)) * 4, cy + math.sin(math.radians(ang)) * 4, (255, 120, 90, 200))
    if tilt:
        img = img.rotate(-tilt, resample=Image.NEAREST, center=(12, cy))
    return img


def drone_shadow(sprite, hover):
    # floats: shadow is small and shrinks as it rises
    return with_shadow(sprite, cy=21, rx=4.5 + hover * .5, ry=1.2)


def drone_anims():
    a = {}
    hov = [0, -1, -2, -1]
    a["idle"] = [drone_shadow(drone(hover=h, thrust=2 + (i % 2)), h) for i, h in enumerate(hov)]
    a["attack"] = [drone_shadow(drone(hover=-1, eye=e, thrust=t), -1)
                   for e, t in ((1, 2), (1, 3), (2, 3), (3, 2))]
    smoke = (60, 60, 64, 200)
    dead = []
    for i, (tilt, drop) in enumerate(((0, 0), (20, 1), (45, 5), (90, 8))):
        spr = drone(hover=drop, eye=3, thrust=0, tilt=tilt)
        if i == 0:
            spr = tint(spr, WHITE, .85)
        if i >= 1:
            for k in range(i * 2):
                put(spr, 9 + k * 2, 4 - k % 2 + drop, smoke)
                flash_star(spr, 15, 6 + drop, 1 if i < 3 else 0)
        dead.append(with_shadow(spr, rx=5, ry=1.4))
    a["death"] = dead
    return a


# ============================================================================
# FX - projectiles and impacts (game rotates bullets/lasers to their heading)
# ============================================================================
def fx_anims():
    a = {}
    b = []
    for k in (0, 1):
        img = new()
        line(img, 6, 12, 15, 12, FLASH2)
        line(img, 10 + k, 12, 17, 12, FLASH1)
        put(img, 17, 12, WHITE)
        put(img, 16, 11, FLASH2); put(img, 16, 13, FLASH2)
        b.append(img)
    a["bullet"] = b
    l = []
    for k in (0, 1):
        img = new()
        rect(img, 4, 11, 19, 13, RED_D)
        rect(img, 5 - k, 11, 19, 12 + k, RED)
        line(img, 7, 12, 19, 12, (255, 200, 190, 255))
        l.append(img)
    a["laser"] = l
    a["hit_spark"] = []
    for s in (3, 2, 1):
        img = new()
        flash_star(img, 12, 12, s)
        a["hit_spark"].append(fade(img, .5 + s / 6))
    a["acid_glob"] = []
    for k in (0, 1):
        img = new()
        ellipse(img, 12, 12, 3 + k * .4, 3 - k * .4, ACID_D)
        ellipse(img, 11.5, 11.5, 2, 1.6, ACID)
        put(img, 11, 11, ACID_L)
        put(img, 12, 16 + k, ACID_D)
        a["acid_glob"].append(outline(img))
    a["acid_splash"] = []
    rng = random.Random(8)
    for i in range(4):
        img = new()
        ellipse(img, 12, 14, 3 + i * 2, 1.5 + i * .7, ACID_D)
        if i < 3:
            ellipse(img, 12, 14, 2 + i * 1.5, 1 + i * .4, ACID)
        for _ in range(8 - i * 2):
            ang = rng.random() * math.pi
            r = 3 + i * 2.5
            put(img, 12 + math.cos(ang) * r, 14 - math.sin(ang) * r, ACID_L if i < 2 else ACID)
        a["acid_splash"].append(fade(img, 1 - i * .2))
    a["explosion"] = []
    stages = [(3, WHITE, FLASH1), (6, FLASH1, FLASH2), (8, FLASH2, (232, 110, 40, 255)),
              (9, (232, 110, 40, 255), RED_D), (10, (90, 70, 66, 255), (52, 46, 48, 255)),
              (10, (60, 58, 60, 140), (40, 40, 42, 90))]
    for i, (r, inner, outer) in enumerate(stages):
        img = new()
        ellipse(img, 12, 12, r, r * .9, outer)
        ellipse(img, 11, 11, r * .65, r * .55, inner)
        rr = random.Random(i)
        for _ in range(6):
            ang = rr.random() * math.tau
            put(img, 12 + math.cos(ang) * (r + 1), 12 + math.sin(ang) * (r + 1), outer)
        a["explosion"].append(img)
    return a


# ---- sheet assembly --------------------------------------------------------
TIMING_MS = {  # per-frame durations used by the game and by the previews
    "idle": 180, "walk": 110, "hurt": 80, "death": 130, "attack": 90,
    "bullet": 50, "laser": 50, "hit_spark": 50, "acid_glob": 90, "acid_splash": 80, "explosion": 70,
    "muzzle_flash": 40, "crosshair": 300,
}


def timing_for(anim):
    for k, v in TIMING_MS.items():
        if anim.startswith(k):
            return v
    return 100


def build_all():
    fx = fx_anims()
    fx.update(player_fx())
    return {
        "player": player_anims_cached(),
        "weapons": weapons(),
        "crawler": crawler_anims(),
        "spitter": spitter_anims(),
        "drone": drone_anims(),
        "fx": fx,
    }


ONE_SHOT = ("death", "hurt", "attack", "splash", "explosion", "spark", "muzzle")


def write_sheet(path, groups):
    """groups: list of (prefix, anims dict). One animation per row."""
    rows = [(f"{p}{name}", frames) for p, anims in groups for name, frames in anims.items()]
    cols = max(len(f) for _, f in rows)
    sheet = Image.new("RGBA", (cols * F, len(rows) * F), CLEAR)
    meta = {}
    for r, (name, frames) in enumerate(rows):
        for c, fr in enumerate(frames):
            sheet.alpha_composite(fr, (c * F, r * F))
        short = name.split(".")[-1]
        meta[name] = {"row": r, "frames": len(frames), "frame_ms": timing_for(short),
                      "loop": not any(k in short for k in ONE_SHOT)}
        if short.startswith("idle_"):
            meta[name]["bob"] = IDLE_BOB
        elif short.startswith("walk_") and name.count(".") == 0:
            meta[name]["bob"] = WALK_BOB
    sheet.save(path)
    return sheet, meta


def write_weapons(path, wpn):
    sheet = Image.new("RGBA", (24, 12 * len(wpn)), CLEAR)
    meta = {}
    for i, (name, (img, info)) in enumerate(wpn.items()):
        sheet.alpha_composite(img, (0, i * 12))
        meta[name] = {"rect": [0, i * 12, 24, 12], **info}
    sheet.save(path)
    return sheet, meta


def main():
    all_ = build_all()
    p_sheet, p_meta = write_sheet("player.png", [("", all_["player"])])
    w_sheet, w_meta = write_weapons("weapons.png", all_["weapons"])
    e_sheet, e_meta = write_sheet("enemies.png", [("crawler.", all_["crawler"]),
                                                  ("spitter.", all_["spitter"]),
                                                  ("drone.", all_["drone"])])
    f_sheet, f_meta = write_sheet("fx.png", [("", all_["fx"])])
    with open("sprites.json", "w") as f:
        json.dump({
            "frame_size": F,
            "notes": {
                "origin": "Feet/ground contact is at roughly (12, 21) in every 24x24 frame.",
                "mirror": "Only *_right facings are drawn. For *_left use the *_right frames flipped horizontally (enemies too).",
                "fx": "bullet and laser point right; rotate them to the projectile heading.",
                "loop": "loop=false animations play once, then hold (death) or return to idle.",
            },
            "player": {
                "facings": list(FACINGS_ALL),
                "facing_from_aim": "angle = atan2(mouseY - handY, mouseX - handX); facing = facings[round(deg(angle) / 45) mod 8]",
                "hand_anchor": HAND_ANCHOR,
                "hand_anchor_note": "Gun pivot position inside the body frame; add the current frame's bob to y.",
                "gun_behind_body": list(GUN_BEHIND_BODY),
                "weapon_draw": "translate(-pivot) -> if aiming left (cos<0) scale(1,-1) -> rotate(angle) -> translate(hand). Pull back by recoil_px along the aim for ~60ms after a shot.",
                "muzzle_world": "hand + rotate((muzzle - pivot) with y negated when aiming left, angle)",
            },
            "sheets": {"player.png": p_meta, "enemies.png": e_meta, "fx.png": f_meta},
            "weapons.png": w_meta,
        }, f, indent=2)
    # viewing copy
    sheets = (p_sheet, w_sheet, e_sheet, f_sheet)
    w = max(s.width for s in sheets)
    h = sum(s.height for s in sheets) + 3 * F
    view = Image.new("RGBA", (w, h), (34, 38, 50, 255))
    y = 0
    for s in sheets:
        view.alpha_composite(s, (0, y))
        y += s.height + F
    view.resize((w * 4, h * 4), Image.NEAREST).save("sprites_x4.png")
    print("player rows", len(p_meta), "weapons", len(w_meta), "enemy rows", len(e_meta), "fx rows", len(f_meta))


if __name__ == "__main__":
    main()
