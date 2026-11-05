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
                        "pellets": 1, "recoil_px": 1, "bullet_speed": 420, "damage": 1,
                        "mag": 36, "reload_ms": 900, "ammo": 180}


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
                        "semi_auto": True, "mag": 12, "reload_ms": 800, "ammo": 0}


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
                        "pellets": 5, "recoil_px": 3, "bullet_speed": 400, "damage": 1,
                        "mag": 8, "reload_ms": 1000, "ammo": 36}


def weapon_smg():
    w = new(24, 12)
    rect(w, 1, 5, 4, 5, G0)                  # wire stock
    rect(w, 1, 7, 4, 7, G0)
    rect(w, 1, 5, 1, 7, G0)
    rect(w, 5, 4, 15, 6, G1)                 # receiver
    rect(w, 5, 4, 15, 4, G2)
    rect(w, 16, 5, 18, 5, G0)                # short barrel
    rect(w, 11, 7, 12, 10, G0)               # long thin magazine
    put(w, 12, 10, G1)
    rect(w, 13, 3, 14, 3, G0)                # sight
    put(w, 9, 5, CY)
    _glove(w, 6, 7)
    return outline(w), {"pivot": [6, 7], "muzzle": [19, 5], "fire_ms": 70, "spread_deg": 7,
                        "pellets": 1, "recoil_px": 1, "bullet_speed": 400, "damage": 1,
                        "mag": 40, "reload_ms": 1100, "ammo": 200}


def weapon_plasma():
    w = new(24, 12)
    rect(w, 1, 5, 4, 8, G0)                  # stock
    rect(w, 1, 5, 4, 5, G1)
    rect(w, 4, 4, 14, 7, G1)                 # body
    rect(w, 4, 4, 14, 4, G2)
    for x in (8, 10, 12):                    # glowing coils
        rect(w, x, 3, x, 7, CY_D)
        put(w, x, 3, CY)
        put(w, x, 5, CY)
    rect(w, 15, 5, 19, 6, G2)                # emitter
    rect(w, 15, 6, 19, 6, G1)
    rect(w, 20, 4, 21, 7, G1)                # muzzle ring
    put(w, 21, 5, CY)
    put(w, 21, 6, CY_D)
    _glove(w, 6, 8)
    return outline(w), {"pivot": [6, 8], "muzzle": [22, 5], "fire_ms": 300, "spread_deg": 1,
                        "pellets": 1, "recoil_px": 2, "bullet_speed": 330, "damage": 3,
                        "pierce": 3, "projectile": "plasma", "mag": 10, "reload_ms": 1300, "ammo": 40}


def weapon_launcher():
    w = new(24, 12)
    rect(w, 3, 3, 20, 7, G1)                 # fat tube
    rect(w, 3, 3, 20, 3, G2)
    rect(w, 3, 7, 20, 7, G0)
    rect(w, 21, 3, 21, 7, G0)                # open muzzle
    rect(w, 14, 3, 15, 7, AMB)               # hazard band
    put(w, 14, 5, AMB_D)
    rect(w, 9, 2, 12, 2, G0)                 # sight rail
    rect(w, 5, 8, 6, 10, G0)                 # grip
    rect(w, 10, 8, 13, 9, G0)                # foregrip
    _glove(w, 7, 8)
    return outline(w), {"pivot": [7, 8], "muzzle": [22, 5], "fire_ms": 750, "spread_deg": 2,
                        "pellets": 1, "recoil_px": 3, "bullet_speed": 240, "damage": 5,
                        "blast": 40, "projectile": "grenade", "semi_auto": True,
                        "mag": 4, "reload_ms": 1600, "ammo": 12}


# Heavy weapons: found rarely, one carried at a time, no reloads ("heavy"):
# "mag" is all the ammo it has and it is thrown away when empty.
def weapon_minigun():
    w = new(24, 12)
    rect(w, 2, 3, 9, 8, G1)                  # motor housing
    rect(w, 2, 3, 9, 3, G2)
    rect(w, 2, 8, 9, 8, G0)
    rect(w, 4, 5, 6, 6, AMB)                 # warning plate
    for y, c in ((4, G2), (6, G1), (8, G2)):  # three rotating barrels
        rect(w, 10, y, 21, y, c)
    rect(w, 10, 4, 10, 8, G0)                # barrel clamps
    rect(w, 17, 4, 17, 8, G0)
    rect(w, 22, 4, 22, 8, G0)
    rect(w, 4, 9, 7, 10, G0)                 # ammo belt feed
    put(w, 5, 10, AMB_D)
    _glove(w, 5, 7)
    return outline(w), {"pivot": [5, 7], "muzzle": [23, 6], "fire_ms": 45, "spread_deg": 6,
                        "pellets": 1, "recoil_px": 1, "bullet_speed": 440, "damage": 1,
                        "mag": 160, "heavy": True, "spin_ms": 400, "move_mul": 0.55}


def weapon_flamethrower():
    w = new(24, 12)
    ellipse(w, 4, 6, 3, 3.4, RED_D)          # fuel tank
    ellipse(w, 3.5, 5, 1.6, 1.6, RED)
    rect(w, 7, 5, 17, 6, G1)                 # body and nozzle
    rect(w, 7, 5, 17, 5, G2)
    rect(w, 18, 4, 20, 7, G0)                # flared tip
    put(w, 21, 5, FLASH2)                    # pilot light
    put(w, 21, 6, AMB)
    line(w, 5, 9, 9, 7, G0)                  # hose
    rect(w, 11, 7, 12, 9, G0)                # grip
    _glove(w, 9, 7)
    return outline(w), {"pivot": [9, 7], "muzzle": [21, 6], "fire_ms": 40, "spread_deg": 9,
                        "pellets": 1, "recoil_px": 0, "bullet_speed": 190, "damage": 1,
                        "mag": 110, "heavy": True, "projectile": "flame", "pierce": 99,
                        "life_ms": 380, "flame": True}


def weapons():
    return {"rifle": weapon_rifle(), "pistol": weapon_pistol(), "shotgun": weapon_shotgun(),
            "smg": weapon_smg(), "plasma": weapon_plasma(), "launcher": weapon_launcher(),
            "minigun": weapon_minigun(), "flamethrower": weapon_flamethrower()}


def flame_anim():
    """A puff of the flamethrower's fire over its short life: a hot core that
    swells, reddens and thins into smoke (drawn by age, not rotated)."""
    frames = []
    stages = [(2.5, WHITE, FLASH1), (4, FLASH1, FLASH2), (5.5, FLASH2, (232, 110, 40, 255)),
              (6.5, (232, 110, 40, 255), RED_D), (7, (110, 60, 50, 200), (60, 50, 50, 120))]
    for i, (r, inner, outer) in enumerate(stages):
        img = new()
        rng = random.Random(40 + i)
        ellipse(img, 12, 12, r, r * .85, outer)
        ellipse(img, 12, 11.5, r * .6, r * .5, inner)
        for _ in range(5):
            ang = rng.random() * math.tau
            put(img, 12 + math.cos(ang) * (r + .5), 12 + math.sin(ang) * r * .85, outer)
        frames.append(fade(img, 1 - i * .12))
    return frames


def pickup(kind, glint):
    """A small box lying on the floor: a medkit or an ammo crate."""
    img = new()
    if kind == "health":
        rect(img, 7, 13, 16, 19, G3)
        rect(img, 7, 13, 16, 13, WHITE)
        rect(img, 7, 19, 16, 19, G2)
        rect(img, 11, 14, 12, 18, RED)
        rect(img, 9, 15, 14, 16, RED)
    else:
        rect(img, 7, 14, 16, 19, A1)
        rect(img, 7, 14, 16, 14, A2)
        rect(img, 7, 19, 16, 19, A0)
        rect(img, 8, 16, 15, 17, AMB_D)
        for bx in (8, 10, 12, 14):          # brass rounds poking out of the top
            rect(img, bx, 11, bx + 1, 13, AMB)
            put(img, bx, 11, FLASH1)
    img = outline(img)
    if glint:
        put(img, 15, 13 if kind == "health" else 11, WHITE)
        put(img, 16, 12 if kind == "health" else 10, FLASH1)
    return with_shadow(img, rx=6)


def player_fx():
    a = {"muzzle_flash": [], "crosshair": [],
         "pickup_health": [pickup("health", g) for g in (0, 0, 0, 1)],
         "pickup_ammo": [pickup("ammo", g) for g in (0, 0, 0, 1)]}
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
# ENEMY 4 - Bomber (a crawler bloated with volatile acid). Faces right; mirror
# for left. Runs at the player, swells up and bursts; shooting it sets it off too.
# ============================================================================
GLOW = [(150, 80, 36, 255), (232, 110, 40, 255), FLASH2, FLASH1]


def bomber(leg_phase=0, bob=0, swell=0.0, glow=0):
    body = new()
    cy = 14 + bob
    for i, lx in enumerate((8, 12, 15)):           # far legs
        swing = 2 if (i + leg_phase) % 2 else -1
        line(body, lx, cy + 2, lx + swing - 1, 20, X0)
    r = 5.5 + swell                                  # the sac
    sy = cy - 2 - swell * .5
    ellipse(body, 9, sy, r, r * .85, X1)
    ellipse(body, 9, sy - .5, r - 1.6, (r - 1.6) * .8, GLOW[glow])
    ellipse(body, 8, sy - 1.5, (r - 1.6) * .5, (r - 1.6) * .35, GLOW[min(glow + 1, 3)])
    line(body, 6, sy - r * .6, 7, sy + r * .5, X1)   # veins
    line(body, 11, sy - r * .7, 12, sy + r * .4, X1)
    hx, hy = 16, cy + 1                              # small head
    ellipse(body, hx - 2, hy, 2.5, 2, X1)
    ellipse(body, hx, hy, 2.6, 2, X2)
    put(body, hx + 1, hy - 1, ACID)
    line(body, hx + 2, hy + 1, hx + 4, hy + 2, BONE)
    for i, lx in enumerate((9, 13, 16)):           # near legs
        swing = -2 if (i + leg_phase) % 2 else 1
        line(body, lx, cy + 2, lx + swing + 1, 20, X2)
        put(body, lx + swing + 1, 20, X3)
    return outline(body)


def bomber_anims():
    a = {}
    a["walk_right"] = [with_shadow(bomber(leg_phase=p, bob=b, glow=g), rx=7)
                       for p, b, g in ((0, 0, 0), (1, -1, 1), (0, 0, 0), (1, -1, 1))]
    a["attack_right"] = [with_shadow(bomber(swell=1, glow=1), rx=7),
                         with_shadow(bomber(swell=1.6, glow=2), rx=7),
                         with_shadow(tint(bomber(swell=2.2, glow=3), WHITE, .6), rx=7)]
    goo = new()
    rng = random.Random(11)
    ellipse(goo, 12, 19, 8, 2.4, X0)
    ellipse(goo, 11, 19, 5, 1.4, (150, 80, 36, 255))
    for _ in range(10):
        ang = rng.random() * math.tau
        put(goo, 12 + math.cos(ang) * rng.uniform(6, 10), 19 + math.sin(ang) * rng.uniform(1.5, 3.5), X1)
    a["death"] = [goo, fade(goo, .8)]
    return a


# ============================================================================
# ENEMY 5 - Brute (armoured, slow, charges). Faces right; mirror for left.
# ============================================================================
def brute(step=0, bob=0, lean=0, arm=0, mouth=0):
    body = new()
    by = 11 + bob
    rect(body, 7 + step, by + 4, 9 + step, 20, X0)            # far leg
    rect(body, 6 + step, 19, 9 + step, 20, X0)
    ellipse(body, 11 + lean, by + 1, 7.5, 6.5, X1)            # hunched torso
    ellipse(body, 10 + lean, by - 1, 6, 4, X2)
    ellipse(body, 9 + lean, by - 2.5, 3, 1.5, X3)
    for px, py in ((5, by - 3), (8, by - 5), (12, by - 5)):  # bone plates on the back
        ellipse(body, px + lean, py, 2.2, 1.5, BONE)
        put(body, px + lean, py + 1, M0)
    rect(body, 12 - step, by + 5, 14 - step, 20, X1)          # near leg
    rect(body, 12 - step, 19, 15 - step, 20, X0)
    put(body, 13 - step, by + 7, X2)
    hx, hy = 17 + lean, by + 2                                 # low head
    ellipse(body, hx, hy, 3, 2.5, X2)
    put(body, hx + 1, hy - 1, RED)
    put(body, hx + 2, hy - 1, RED)
    if mouth:
        rect(body, hx, hy + 1, hx + 3, hy + 1 + mouth, RED_DD)
        put(body, hx + 1, hy + 1, BONE); put(body, hx + 3, hy + 1, BONE)
    else:
        line(body, hx, hy + 1, hx + 3, hy + 1, X0)
    sx, sy = 13 + lean, by                                     # big arm and claw
    ex, ey = 18 + lean + arm, by + 5 - arm * 2
    line(body, sx, sy, ex, ey, X2)
    line(body, sx, sy + 1, ex, ey + 1, X1)
    for dx, dy in ((2, -1), (2, 1), (1, 2)):
        line(body, ex, ey, ex + dx, ey + dy, BONE)
    return outline(body)


def brute_anims():
    a = {}
    a["walk_right"] = [with_shadow(brute(step=s, bob=b), rx=9)
                       for s, b in ((0, 0), (1, -1), (0, 0), (-1, -1))]
    a["attack_right"] = [                                       # 0-1 wind-up, 2-3 charge
        with_shadow(brute(lean=-2, arm=-1, mouth=1), rx=9),
        with_shadow(brute(lean=-3, arm=-1, mouth=2, bob=1), rx=9),
        with_shadow(brute(lean=2, arm=2, mouth=2, step=1), rx=9),
        with_shadow(brute(lean=3, arm=2, mouth=2, step=-1, bob=-1), rx=9),
    ]
    alive = brute()
    lying = shift(alive.rotate(-90, resample=Image.NEAREST), 2, 4)
    pool = new()
    ellipse(pool, 12, 19, 10, 3, RED_DD)
    ellipse(pool, 13, 19, 6, 1.6, RED_D)
    a["death"] = [
        with_shadow(tint(alive, WHITE, .85), rx=9),
        with_shadow(shift(brute(lean=-2, mouth=2), 0, 1), rx=9),
        with_shadow(lying, rx=10),
        Image.alpha_composite(pool, lying),
    ]
    return a


# ============================================================================
# ENEMY 6 - Splitter (a fat grub carrying a brood in a see-through sac).
# Faces right; mirror for left. Bites like a crawler; when it dies the sac
# bursts and three small crawlers come out.
# ============================================================================
def splitter(leg_phase=0, bob=0, mouth=0, lunge=0, pulse=0):
    body = new()
    cy = 14 + bob
    for i, lx in enumerate((7, 10, 13, 16)):        # far legs
        swing = 2 if (i + leg_phase) % 2 else -1
        line(body, lx + lunge, cy + 2, lx + lunge + swing - 1, 20, M0)
    sx = 9 + lunge                                   # the sac with its brood
    ellipse(body, sx, cy - 1, 6.5 + pulse * .5, 5 + pulse * .4, M1)
    ellipse(body, sx - 1, cy - 2.5, 4.5, 2.4, M2)
    for ex, ey in ((-3, -1), (0, -3), (1, 1)):       # curled shapes inside
        ellipse(body, sx + ex, cy + ey, 1.6, 1.1, X1)
        put(body, sx + ex + 1, cy + ey, ACID)
    put(body, sx - 3, cy - 4, M3)
    for vx in (-4, 2):                               # sac seams
        line(body, sx + vx, cy - 5, sx + vx + 1, cy + 3, M0)
    hx, hy = 17 + lunge, cy + 1                      # head
    ellipse(body, hx - 1, hy, 3, 2.4, X1)
    ellipse(body, hx, hy - .5, 2.4, 1.6, X2)
    put(body, hx + 1, hy - 1, ACID)
    put(body, hx + 1, hy - 2, ACID_L)
    jaw = [1, 2, 3][mouth]
    line(body, hx + 2, hy, hx + 4, hy - jaw + 1, BONE)
    line(body, hx + 2, hy + 1, hx + 4, hy + jaw, BONE)
    for i, lx in enumerate((8, 11, 14, 17)):        # near legs
        swing = -2 if (i + leg_phase) % 2 else 1
        line(body, lx + lunge, cy + 2, lx + lunge + swing + 1, 20, M2)
        put(body, lx + lunge + swing + 1, 20, M3)
    return outline(body)


def splitter_anims():
    a = {}
    a["walk_right"] = [with_shadow(splitter(leg_phase=p, bob=b, pulse=u), rx=9)
                       for p, b, u in ((0, 0, 0), (1, -1, 1), (0, 0, 0), (1, -1, 1))]
    a["attack_right"] = [
        with_shadow(splitter(lunge=-2, mouth=1), rx=9),
        with_shadow(splitter(lunge=2, mouth=2, leg_phase=1), rx=9),
        with_shadow(splitter(lunge=3, mouth=0, leg_phase=1), rx=9),
        with_shadow(splitter(lunge=1, mouth=1), rx=9),
    ]
    torn = new()                                     # the empty, burst sac
    ellipse(torn, 11, 18, 9, 3, M0)
    ellipse(torn, 11, 17, 6.5, 2.2, M1)
    for tx, ty in ((6, 15), (9, 14), (13, 14), (16, 15)):
        line(torn, tx, ty, tx + 1, ty + 2, M2)
    ellipse(torn, 14, 19, 3, 1, ACID_D)
    torn = outline(torn)
    splat = new()
    rng = random.Random(17)
    for _ in range(16):
        ang = rng.random() * math.tau
        r = rng.uniform(4, 10)
        put(splat, 11 + math.cos(ang) * r, 12 + math.sin(ang) * r * .7, rng.choice([M2, M3, ACID, ACID_D]))
    a["death"] = [
        with_shadow(tint(splitter(pulse=2), WHITE, .85), rx=9),
        with_shadow(Image.alpha_composite(splitter(pulse=3), splat), rx=10),
        Image.alpha_composite(torn, splat),
        torn,
    ]
    return a


# ============================================================================
# ENEMY 7 - Shield-bearer (an infested security trooper in riot armour).
# Faces right; mirror for left. The shield is a separate sprite ("shield",
# a plate pointing right) that the game rotates around the body toward its
# heading, the way the player's gun is drawn.
# ============================================================================
def shielder(step=0, bob=0, lean=0, arm=0):
    body = new()
    by = 12 + bob
    rect(body, 8 + step, by + 3, 10 + step, 20, U0)            # far leg
    rect(body, 8 + step, 19, 11 + step, 20, U0)
    ellipse(body, 11 + lean, by + 1, 5.5, 5.5, G1)              # armoured torso
    ellipse(body, 10 + lean, by - 1, 4, 3, G2)
    put(body, 9 + lean, by - 2, G3)
    for fx, fy in ((8, by + 3), (13, by - 2)):                 # flesh bulging through
        ellipse(body, fx + lean, fy, 1.6, 1.2, X2)
        put(body, fx + lean, fy, X3)
    rect(body, 12 - step, by + 4, 14 - step, 20, G1)            # near leg
    rect(body, 12 - step, 19, 15 - step, 20, U1)
    hx, hy = 13 + lean, by - 6                                  # helmet with a red visor
    ellipse(body, hx, hy, 3.2, 2.8, G1)
    ellipse(body, hx - 1, hy - 1, 2, 1.2, G2)
    line(body, hx + 1, hy, hx + 3, hy, RED)
    put(body, hx + 3, hy - 1, RED_D)
    ex, ey = 16 + lean + arm, by + 1                            # arm out front, holding the shield
    line(body, 12 + lean, by - 1, ex, ey, X1)
    line(body, 12 + lean, by, ex, ey + 1, X2)
    return outline(body)


def shield_plate():
    """The riot shield, pointing right: a tall plate centred in the frame."""
    img = new()
    rect(img, 11, 4, 13, 19, G1)
    rect(img, 13, 5, 13, 18, G3)              # front edge catching the light
    rect(img, 11, 4, 11, 19, G0)
    line(img, 12, 6, 12, 17, G2)
    for y in (8, 15):
        put(img, 12, y, CY)                   # warning lights
    put(img, 12, 11, AMB)
    put(img, 12, 12, AMB_D)
    return outline(img)


def shielder_anims():
    a = {}
    a["walk_right"] = [with_shadow(shielder(step=s, bob=b), rx=8)
                       for s, b in ((0, 0), (1, -1), (0, 0), (-1, -1))]
    a["attack_right"] = [                                       # 0-1 brace, 2-3 bash
        with_shadow(shielder(lean=-2, arm=-1), rx=8),
        with_shadow(shielder(lean=-2, arm=-2, bob=1), rx=8),
        with_shadow(shielder(lean=2, arm=2, step=1), rx=8),
        with_shadow(shielder(lean=3, arm=2, step=-1, bob=-1), rx=8),
    ]
    a["shield"] = [shield_plate()]
    alive = shielder()
    lying = shift(alive.rotate(-90, resample=Image.NEAREST), 2, 5)
    plate = shift(shield_plate().rotate(90, resample=Image.NEAREST), -3, 8)
    pool = new()
    ellipse(pool, 12, 19, 9, 2.6, X0)
    ellipse(pool, 11, 19, 5, 1.4, X1)
    a["death"] = [
        with_shadow(tint(alive, WHITE, .85), rx=8),
        with_shadow(shift(shielder(lean=-2, arm=-2), 0, 1), rx=8),
        with_shadow(Image.alpha_composite(lying, plate), rx=10),
        Image.alpha_composite(pool, Image.alpha_composite(lying, plate)),
    ]
    return a


# ============================================================================
# ENEMY 8 - Nest (a pulsing hive mound that births crawlers). Faces the camera.
# ============================================================================
def nest(pulse=0.0, open=0, burst=False):
    body = new()
    ellipse(body, 12, 18, 10, 3.6, X0)                          # roots spreading on the floor
    for rx, ry in ((2, 19), (21, 18), (5, 21), (19, 21)):
        line(body, 12, 18, rx, ry, X0)
    if burst:
        ellipse(body, 12, 16, 8, 3.5, X1)
        ellipse(body, 12, 15.5, 5, 2, ACID_D)
        for tx, ty in ((5, 13), (8, 11), (15, 11), (18, 13)):
            line(body, tx, ty, tx + (1 if tx < 12 else -1), ty + 4, X2)
        return outline(body)
    r = 7.5 + pulse
    ellipse(body, 12, 13 - pulse * .5, r, 6.5 + pulse * .7, X1)  # the dome
    ellipse(body, 10, 10 - pulse * .5, r * .55, 3, X2)
    put(body, 8, 9, X3)
    for vx0, vx1 in ((6, 8), (17, 15), (11, 12)):              # veins
        line(body, vx0, 16, vx1, 8, X0)
    for hx, hy in ((7, 14), (17, 13)):                           # small glowing pores
        put(body, hx, hy, ACID_D)
        put(body, hx, hy - 1, ACID)
    w = [1, 2, 3, 4][open]                                       # the mouth on top
    ellipse(body, 12, 9 - pulse * .5, w, max(1, w * .6), X0)
    if open:
        ellipse(body, 12, 9 - pulse * .5, w - .6, max(.6, w * .4), ACID if open > 1 else ACID_D)
    return outline(body)


def nest_anims():
    a = {}
    a["idle"] = [with_shadow(nest(pulse=p), rx=10) for p in (0, .4, .8, .4)]
    a["attack"] = [                                              # the birth: swell, open, push out
        with_shadow(nest(pulse=1.2, open=1), rx=10),
        with_shadow(nest(pulse=1.6, open=2), rx=10),
        with_shadow(nest(pulse=1.0, open=3), rx=10),
        with_shadow(nest(pulse=.4, open=1), rx=10),
    ]
    splat = new()
    rng = random.Random(23)
    for _ in range(18):
        ang = rng.random() * math.tau
        r = rng.uniform(5, 11)
        put(splat, 12 + math.cos(ang) * r, 11 + math.sin(ang) * r * .7, rng.choice([X2, X3, ACID, ACID_L]))
    a["death"] = [
        with_shadow(tint(nest(pulse=1.8, open=3), WHITE, .85), rx=10),
        with_shadow(Image.alpha_composite(nest(pulse=2, open=3), splat), rx=10),
        Image.alpha_composite(nest(burst=True), splat),
        nest(burst=True),
    ]
    return a


# ============================================================================
# BOSS - Brood Mother, 48x48 frames (feet at 24, 45). Faces the camera.
# ============================================================================
B = 48


def boss_shadow(sprite, rx=17):
    base = new(B, B)
    ellipse(base, 24, 45, rx, 3, SHADOW)
    base.alpha_composite(sprite)
    return base


def boss(breath=0, leg=0, maw=0, rear=0, glow=0, burst=False):
    img = new(B, B)
    cy = 26 - rear + breath
    for i, (x0, x1) in enumerate(((14, 5), (18, 9), (30, 39), (34, 43))):   # far legs
        k = (leg if i % 2 else -leg)
        line(img, x0, cy + 4, (x0 + x1) / 2 + k, cy - 6, X0)
        line(img, (x0 + x1) / 2 + k, cy - 6, x1 + k, 44, X0)
    ellipse(img, 24, cy - 2, 15, 11 + breath * .5, X1)                       # abdomen
    ellipse(img, 22, cy - 5, 11, 6, X2)
    ellipse(img, 19, cy - 8, 5, 2, X3)
    for sx in (16, 24, 32):
        line(img, sx, cy - 12, sx + 1, cy + 7, X0)
    if not burst:                                                         # egg sacs
        for sx, sy, r in ((14, cy - 1, 3.2), (24, cy - 9, 3.6), (33, cy - 2, 3.0)):
            ellipse(img, sx, sy, r + glow * .4, r * .85 + glow * .3, ACID_D)
            ellipse(img, sx - .6, sy - .6, (r + glow * .4) * .6, r * .45, ACID)
            put(img, sx - 1, sy - 1, ACID_L)
    else:
        for sx, sy in ((14, cy - 1), (24, cy - 9), (33, cy - 2)):
            ellipse(img, sx, sy, 2.5, 1.5, M0)
            put(img, sx, sy - 2, ACID_D)
    hy = cy + 9                                                          # head
    ellipse(img, 24, hy, 10.5, 7.5, OUT)
    ellipse(img, 24, hy, 9, 6, X1)
    ellipse(img, 24, hy - 1, 7.5, 4.5, X2)
    for hx in (17, 31):                                                  # horns
        line(img, hx, hy - 3, hx + (-3 if hx < 24 else 3), hy - 9 - rear, BONE)
    for ex in (19, 22, 26, 29):
        put(img, ex, hy - 2 + (1 if ex in (19, 29) else 0), FLASH2 if glow else AMB)
    if maw:
        ellipse(img, 24, hy + 2, 3 + maw, 1 + maw, RED_DD)
        for tx in range(21 - maw, 28 + maw, 2):
            put(img, tx, hy + 1, BONE)
        if maw > 1:
            put(img, 24, hy + 3 + maw, ACID)
    else:
        line(img, 20, hy + 2, 28, hy + 2, X0)
    for i, (x0, sgn) in enumerate(((16, -1), (32, 1))):                 # scythe arms
        mx, my = x0 + sgn * 6, hy - 2 - rear * 2
        line(img, x0, hy, mx, my, X2)
        line(img, x0, hy + 1, mx, my + 1, X1)
        line(img, mx, my, mx + sgn * 2, 43 - rear * 3 + (leg if i else -leg), BONE)
    return outline(img)


def boss_anims():
    a = {}
    a["idle"] = [boss_shadow(boss(breath=b, leg=l)) for b, l in ((0, 0), (1, 1), (1, 0), (0, -1))]
    a["attack"] = [boss_shadow(boss(rear=r, maw=m, glow=g))
                   for r, m, g in ((1, 1, 0), (3, 2, 1), (3, 3, 1), (1, 1, 0))]
    flat = new(B, B)
    ellipse(flat, 24, 40, 19, 5, X0)
    ellipse(flat, 24, 39, 15, 3.5, X1)
    ellipse(flat, 18, 41, 4, 1.4, ACID_D)
    ellipse(flat, 31, 40, 3, 1, ACID)
    dead = boss(burst=True, maw=3)
    sunk = new(B, B)
    sunk.alpha_composite(dead.resize((B, 30), Image.NEAREST), (0, 16))
    a["death"] = [
        boss_shadow(tint(boss(rear=3, maw=3, glow=1), WHITE, .85)),
        boss_shadow(boss(rear=4, maw=3, glow=1, breath=-1)),
        boss_shadow(dead),
        Image.alpha_composite(flat, sunk),
        Image.alpha_composite(flat, fade(sunk, .6)),
        outline(flat),
    ]
    return a


# ============================================================================
# BOSS 2 - Warden, a security mech guarding the reactor. 48x48, faces the camera.
# ============================================================================
def warden(bob=0, step=0, visor=0, cannons=0, tilt=0, wreck=False):
    img = new(B, B)
    ty = 13 + bob                                                       # torso top
    for lx, k in ((15, step), (29, -step)):                            # legs
        rect(img, lx, ty + 17, lx + 4, 42 + min(k, 0), G0)
        rect(img, lx + 1, ty + 17, lx + 3, 40 + min(k, 0), G1)
        rect(img, lx - 2, 42 + min(k, 0), lx + 6, 44 + min(k, 0), G1)   # feet
        rect(img, lx - 2, 42 + min(k, 0), lx + 6, 42 + min(k, 0), G2)
        put(img, lx + 2, ty + 24, AMB)                                  # knee light
    rect(img, 13, ty + 15, 35, ty + 18, G0)                              # hips
    rect(img, 10, ty, 38, ty + 15, G1)                                   # torso
    rect(img, 10, ty, 38, ty + 1, G2)
    rect(img, 11, ty, 37, ty, G3)
    rect(img, 10, ty + 13, 38, ty + 15, G0)
    for px in (10, 38):
        put(img, px, ty, CLEAR)
    rect(img, 15, ty + 7, 33, ty + 12, G2)                               # chest plate
    rect(img, 15, ty + 7, 33, ty + 7, G3)
    rect(img, 24, ty + 7, 24, ty + 12, G1)
    for hx in range(13, 36):                              # hazard band on the hips
        rect(img, hx, ty + 15, hx, ty + 17, AMB if (hx // 2) % 2 else U0)
    vc = [RED_D, RED, FLASH2][visor]                                     # visor bar
    rect(img, 16, ty + 3, 32, ty + 5, U0)
    rect(img, 17, ty + 4, 31, ty + 4, vc)
    if visor:
        put(img, 18 + (visor * 5) % 12, ty + 4, WHITE)
    for cx, sgn in ((3, -1), (39, 1)):                                   # shoulder cannons
        rect(img, cx, ty - 3, cx + 6, ty + 8, G1)
        rect(img, cx, ty - 3, cx + 6, ty - 2, G2)
        rect(img, cx + 1, ty + 1, cx + 5, ty + 2, AMB)                  # warning stripe
        put(img, cx + 3, ty + 1, AMB_D)
        rect(img, cx + 1, ty + 9, cx + 5, ty + 11, G0)                  # muzzles
        mc = [U0, RED, FLASH1][cannons]
        rect(img, cx + 2, ty + 10, cx + 4, ty + 11, mc)
    rect(img, 23, ty - 5, 24, ty - 1, G2)                                # antenna
    put(img, 23, ty - 6, RED if visor else RED_D)
    out = outline(img)
    if wreck:
        rng = random.Random(5)
        for _ in range(40):                                              # scorch and holes
            put(out, rng.randint(10, 38), rng.randint(ty, ty + 18), rng.choice([U0, G0, (60, 60, 64, 255)]))
    if tilt:
        out = out.rotate(-tilt, resample=Image.NEAREST, center=(24, 44))
    return out


def warden_anims():
    a = {}
    a["idle"] = [boss_shadow(warden(bob=b, step=s, visor=v), rx=15)
                 for b, s, v in ((0, 0, 0), (1, 1, 1), (0, 0, 0), (1, -1, 1))]
    a["attack"] = [boss_shadow(warden(visor=v, cannons=c, bob=b), rx=15)
                   for v, c, b in ((1, 1, 0), (2, 2, -1), (2, 2, -1), (1, 1, 0))]
    smoke = (60, 60, 64, 200)
    dead = []
    for i, (tilt, drop) in enumerate(((0, 0), (6, 0), (14, 2), (24, 5), (24, 5), (24, 5))):
        spr = warden(visor=0, wreck=i >= 2, tilt=tilt)
        if i == 0:
            spr = tint(spr, WHITE, .85)
        spr = shift(spr, 0, drop)
        if 1 <= i <= 3:
            flash_star(spr, 16 + i * 5, 18 + i * 2, 2)
        if i >= 3:
            for k in range(6):
                put(spr, 18 + k * 2, 12 - k % 2 + i, smoke)
        dead.append(boss_shadow(spr, rx=16))
    a["death"] = dead
    return a


# ============================================================================
# BOSS 3 - the Maw, a giant worm rising out of the hydroponic beds. 48x48,
# faces the camera. It burrows: the game sinks and raises it by cropping
# frames at the feet, so the hole is drawn at the bottom of every frame.
# ============================================================================
def maw_hole(img):
    ellipse(img, 24, 43, 14, 3.6, OUT)
    ellipse(img, 24, 43.5, 12, 2.4, (10, 8, 6, 255))
    for rx, ry, c in ((9, 41, (86, 62, 40, 255)), (37, 42, (86, 62, 40, 255)), (12, 45, (60, 44, 30, 255)),
                      (34, 45, (60, 44, 30, 255)), (17, 40, G1), (31, 40, G1)):
        ellipse(img, rx, ry, 2.2, 1.3, c)                                # rubble and broken plating


def maw(sway=0, open=0, glow=0, droop=0, wreck=False, hole=True):
    img = new(B, B)
    if hole:
        maw_hole(img)
    hx, hy = 24 + sway + droop, 15 + droop * .6                         # head centre
    for i in range(6, -1, -1):                                           # body rings, bottom up
        k = i / 6
        sx = 24 + (hx - 24) * (1 - k) ** 2 + math.sin(k * 3) * sway * .5
        sy = 43 - (43 - hy - 7) * (1 - k)
        r = 7.5 + 2.5 * k                                                # wider where it leaves the hole
        ellipse(img, sx, sy, r, 4.2, M0)
        ellipse(img, sx, sy - .8, r - .8, 3.2, M1)
        ellipse(img, sx - r * .35, sy - 1.6, r * .4, 1.2, M2)            # sheen, lit from the top left
        if i % 2:
            put(img, sx + r * .55, sy - .5, ACID_D)                      # pores
    ellipse(img, hx, hy, 12.5, 10, OUT)                                  # head
    ellipse(img, hx, hy, 11.5, 9, M1)
    ellipse(img, hx - 2, hy - 3, 8, 5, M2)
    ellipse(img, hx - 4, hy - 5, 3, 1.4, M3)
    mr = 3 + open * 1.6                                                  # mouth, facing us
    ellipse(img, hx, hy + 1, mr + 1.5, mr * .8 + 1.5, M0)
    ellipse(img, hx, hy + 1, mr, mr * .8, RED_DD)
    if open:
        ellipse(img, hx, hy + 1.5, mr - 1.5, mr * .8 - 1.2, RED_D)
        ellipse(img, hx, hy + 2, 1 + glow, .8 + glow * .6, ACID if glow else ACID_D)
    for t in range(10):                                                  # ring of teeth
        a = t / 10 * 2 * math.pi
        put(img, hx + math.cos(a) * (mr + .6), hy + 1 + math.sin(a) * (mr * .8 + .6), BONE)
    for ex, ey in ((-8, -2), (8, -2), (-6, 5), (6, 5)):                  # eye spots
        put(img, hx + ex, hy + ey, FLASH2 if glow else AMB)
    if glow > 1:
        put(img, hx, hy + 4 + open, ACID)                                # drool
        put(img, hx, hy + 5 + open, ACID_D)
    out = outline(img)
    if wreck:
        rng = random.Random(9)
        for _ in range(30):
            put(out, rng.randint(14, 34), rng.randint(int(hy) - 6, 40), rng.choice([M0, RED_DD, ACID_D]))
    return out


def maw_anims():
    a = {}
    a["idle"] = [boss_shadow(maw(sway=s, open=o), rx=15)
                 for s, o in ((0, 0), (1, 1), (2, 0), (1, 1), (0, 0), (-1, 1), (-2, 0), (-1, 1))]
    a["attack"] = [boss_shadow(maw(open=o, glow=g), rx=15) for o, g in ((1, 1), (2, 2), (3, 2), (2, 1))]
    dead = []
    for i, (droop, sink) in enumerate(((0, 0), (3, 0), (6, 4), (6, 12), (6, 22), (6, 34))):
        spr = maw(open=3 if i < 2 else 1, droop=droop, wreck=i >= 2, hole=not sink)
        if i == 0:
            spr = tint(spr, WHITE, .85)
        if sink:  # slides back down into its hole: shift it down, cut it off at the hole
            body = spr.crop((0, 0, B, 43 - sink))
            spr = new(B, B)
            maw_hole(spr)
            spr = outline(spr)
            spr.alpha_composite(body, (0, sink))
            if i == 5:
                ellipse(spr, 24, 43, 4, 1.2, ACID_D)                    # it leaves a puddle
        dead.append(boss_shadow(spr, rx=15))
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
    a["plasma"] = []
    for k in (0, 1):
        img = new()
        ellipse(img, 12, 12, 5 + k, 2.2, CY_D)
        ellipse(img, 13, 12, 3.5 + k, 1.2, CY)
        line(img, 11, 12, 16, 12, WHITE)
        put(img, 6 - k, 12, CY_D)
        a["plasma"].append(img)
    a["grenade"] = []
    for k in (0, 1):
        img = new()
        ellipse(img, 12, 12, 2.6, 2.4, G1)
        put(img, 11, 11, G3)
        put(img, 13, 12, RED if k else AMB)
        a["grenade"].append(outline(img))
    a["spine"] = []
    for k in (0, 1):
        img = new()
        line(img, 7, 12, 16, 12, BONE)
        line(img, 7, 11, 11, 11, X2)
        line(img, 7, 13, 11, 13, X1)
        rect(img, 17, 12, 18, 12, ACID if k else ACID_L)
        put(img, 5 - k, 12, ACID_D)
        a["spine"].append(outline(img))
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
    a["debris"] = []  # a crate breaking: plates and bits fly out and drop
    rr = random.Random(12)
    bits = [(rr.uniform(0, math.tau), rr.uniform(2, 7), rr.uniform(18, 44), rr.uniform(-30, -8),
             rr.choice((0, 1, 1, 2)), rr.random() < .5) for _ in range(18)]
    shades = [(170, 110, 70, 255), (120, 70, 44, 255), (70, 40, 26, 255)]
    for i in range(5):
        img = new()
        if i < 3:  # dust
            ellipse(img, 12, 13, 5 + i * 2.5, 4 + i * 1.8, (190, 180, 160, 150 - i * 40))
        t = i * .07
        for ang, r0, v, up, c, big in bits:
            r = r0 + v * t
            x = 12 + math.cos(ang) * r
            y = 13 + math.sin(ang) * r * .6 + up * t + 170 * t * t
            put(img, x, y, shades[c])
            if big:
                put(img, x + 1, y, shades[c])
                put(img, x, y + 1, shades[min(c + 1, 2)])
                put(img, x + 1, y + 1, shades[min(c + 1, 2)])
        if i == 0:
            rect(img, 10, 11, 12, 12, AMB)  # the latch flies too
        a["debris"].append(fade(img, 1 - i * .15))
    return a


# ============================================================================
# FONT - 5x7 pixel glyphs for the HUD, white with a baked 1px dark outline so
# text reads over any background. The game tints them and draws titles at 2x.
# ============================================================================
GLYPHS = {
    "A": ".###.|#...#|#...#|#####|#...#|#...#|#...#",
    "B": "####.|#...#|#...#|####.|#...#|#...#|####.",
    "C": ".###.|#...#|#....|#....|#....|#...#|.###.",
    "D": "####.|#...#|#...#|#...#|#...#|#...#|####.",
    "E": "#####|#....|#....|####.|#....|#....|#####",
    "F": "#####|#....|#....|####.|#....|#....|#....",
    "G": ".###.|#...#|#....|#.###|#...#|#...#|.####",
    "H": "#...#|#...#|#...#|#####|#...#|#...#|#...#",
    "I": ".###.|..#..|..#..|..#..|..#..|..#..|.###.",
    "J": "..###|...#.|...#.|...#.|...#.|#..#.|.##..",
    "K": "#...#|#..#.|#.#..|##...|#.#..|#..#.|#...#",
    "L": "#....|#....|#....|#....|#....|#....|#####",
    "M": "#...#|##.##|#.#.#|#.#.#|#...#|#...#|#...#",
    "N": "#...#|#...#|##..#|#.#.#|#..##|#...#|#...#",
    "O": ".###.|#...#|#...#|#...#|#...#|#...#|.###.",
    "P": "####.|#...#|#...#|####.|#....|#....|#....",
    "Q": ".###.|#...#|#...#|#...#|#.#.#|#..#.|.##.#",
    "R": "####.|#...#|#...#|####.|#.#..|#..#.|#...#",
    "S": ".####|#....|#....|.###.|....#|....#|####.",
    "T": "#####|..#..|..#..|..#..|..#..|..#..|..#..",
    "U": "#...#|#...#|#...#|#...#|#...#|#...#|.###.",
    "V": "#...#|#...#|#...#|#...#|#...#|.#.#.|..#..",
    "W": "#...#|#...#|#...#|#.#.#|#.#.#|#.#.#|.#.#.",
    "X": "#...#|#...#|.#.#.|..#..|.#.#.|#...#|#...#",
    "Y": "#...#|#...#|.#.#.|..#..|..#..|..#..|..#..",
    "Z": "#####|....#|...#.|..#..|.#...|#....|#####",
    "0": ".###.|#...#|#..##|#.#.#|##..#|#...#|.###.",
    "1": "..#..|.##..|..#..|..#..|..#..|..#..|.###.",
    "2": ".###.|#...#|....#|...#.|..#..|.#...|#####",
    "3": "####.|....#|....#|.###.|....#|....#|####.",
    "4": "...#.|..##.|.#.#.|#..#.|#####|...#.|...#.",
    "5": "#####|#....|####.|....#|....#|#...#|.###.",
    "6": ".###.|#....|#....|####.|#...#|#...#|.###.",
    "7": "#####|....#|...#.|..#..|.#...|.#...|.#...",
    "8": ".###.|#...#|#...#|.###.|#...#|#...#|.###.",
    "9": ".###.|#...#|#...#|.####|....#|....#|.###.",
    " ": ".....|.....|.....|.....|.....|.....|.....",
    ".": ".....|.....|.....|.....|.....|.....|..#..",
    ",": ".....|.....|.....|.....|.....|..#..|.#...",
    ":": ".....|..#..|.....|.....|.....|..#..|.....",
    "!": "..#..|..#..|..#..|..#..|..#..|.....|..#..",
    "?": ".###.|#...#|....#|...#.|..#..|.....|..#..",
    "-": ".....|.....|.....|.###.|.....|.....|.....",
    "+": ".....|..#..|..#..|#####|..#..|..#..|.....",
    "=": ".....|.....|#####|.....|#####|.....|.....",
    "/": "....#|...#.|...#.|..#..|.#...|.#...|#....",
    "%": "##..#|##.#.|...#.|..#..|.#...|.#.##|#..##",
    "(": "...#.|..#..|.#...|.#...|.#...|..#..|...#.",
    ")": ".#...|..#..|...#.|...#.|...#.|..#..|.#...",
    "[": ".###.|.#...|.#...|.#...|.#...|.#...|.###.",
    "]": ".###.|...#.|...#.|...#.|...#.|...#.|.###.",
    "<": "...#.|..#..|.#...|#....|.#...|..#..|...#.",
    ">": ".#...|..#..|...#.|....#|...#.|..#..|.#...",
    "'": "..#..|..#..|.....|.....|.....|.....|.....",
}
FONT_CELL = (7, 9)  # 5x7 glyph plus the outline on every side
FONT_ADVANCE = 6


def font_sheet():
    chars = "".join(GLYPHS)
    cw, ch = FONT_CELL
    sheet = Image.new("RGBA", (cw * len(chars), ch), CLEAR)
    for i, c in enumerate(chars):
        g = Image.new("RGBA", FONT_CELL, CLEAR)
        for y, row in enumerate(GLYPHS[c].split("|")):
            assert len(row) == 5, c
            for x, bit in enumerate(row):
                if bit == "#":
                    put(g, x + 1, y + 1, WHITE)
        sheet.alpha_composite(outline(g), (i * cw, 0))
    meta = {"chars": chars, "cell": list(FONT_CELL), "advance": FONT_ADVANCE, "glyph_origin": [1, 1]}
    return sheet, meta


# ---- sheet assembly --------------------------------------------------------
TIMING_MS = {  # per-frame durations used by the game and by the previews
    "idle": 180, "walk": 110, "hurt": 80, "death": 130, "attack": 90,
    "bullet": 50, "laser": 50, "hit_spark": 50, "acid_glob": 90, "acid_splash": 80, "explosion": 70,
    "muzzle_flash": 40, "crosshair": 300, "plasma": 60, "grenade": 100, "spine": 80, "pickup": 250,
    "debris": 70, "flame": 76,
}


def timing_for(anim):
    for k, v in TIMING_MS.items():
        if anim.startswith(k):
            return v
    return 100


def build_all():
    fx = fx_anims()
    fx.update(player_fx())
    fx["flame"] = flame_anim()
    return {
        "player": player_anims_cached(),
        "weapons": weapons(),
        "crawler": crawler_anims(),
        "spitter": spitter_anims(),
        "drone": drone_anims(),
        "bomber": bomber_anims(),
        "brute": brute_anims(),
        "boss": boss_anims(),
        "warden": warden_anims(),
        "maw": maw_anims(),
        "splitter": splitter_anims(),
        "shielder": shielder_anims(),
        "nest": nest_anims(),
        "fx": fx,
    }


ONE_SHOT = ("death", "hurt", "attack", "splash", "explosion", "spark", "muzzle")


def write_sheet(path, groups, size=F):
    """groups: list of (prefix, anims dict). One animation per row of size x size frames."""
    rows = [(f"{p}{name}", frames) for p, anims in groups for name, frames in anims.items()]
    cols = max(len(f) for _, f in rows)
    sheet = Image.new("RGBA", (cols * size, len(rows) * size), CLEAR)
    meta = {}
    for r, (name, frames) in enumerate(rows):
        for c, fr in enumerate(frames):
            sheet.alpha_composite(fr, (c * size, r * size))
        short = name.split(".")[-1]
        meta[name] = {"row": r, "frames": len(frames), "frame_ms": timing_for(short),
                      "loop": not any(k in short for k in ONE_SHOT)}
        if size != F:
            meta[name]["size"] = size
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
                                                  ("drone.", all_["drone"]),
                                                  ("bomber.", all_["bomber"]),
                                                  ("brute.", all_["brute"]),
                                                  ("splitter.", all_["splitter"]),
                                                  ("shielder.", all_["shielder"]),
                                                  ("nest.", all_["nest"])])
    b_sheet, b_meta = write_sheet("bosses.png", [("brood.", all_["boss"]), ("warden.", all_["warden"]),
                                                ("maw.", all_["maw"])], size=B)
    t_sheet, t_meta = font_sheet()
    t_sheet.save("font.png")
    f_sheet, f_meta = write_sheet("fx.png", [("", all_["fx"])])
    with open("sprites.json", "w") as f:
        json.dump({
            "frame_size": F,
            "notes": {
                "origin": "Feet/ground contact is at roughly (12, 21) in every 24x24 frame; (size/2, size-3) for bigger frames (\"size\").",
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
            "sheets": {"player.png": p_meta, "enemies.png": e_meta, "fx.png": f_meta, "bosses.png": b_meta},
            "weapons.png": w_meta,
            "font.png": t_meta,
        }, f, indent=2)
    # viewing copy
    sheets = (p_sheet, w_sheet, e_sheet, f_sheet, b_sheet, t_sheet)
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
