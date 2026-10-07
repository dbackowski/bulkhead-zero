"""Procedural 24x24 top-down sci-fi tileset generator.

Outputs:
  tileset.png        - the sheet (8 tiles per row, 24x24 each, no padding)
  tileset_x4.png     - same sheet scaled 4x for viewing
  preview_x3.png     - a small station room built from the tiles, scaled 3x
  tiles.json         - tile index -> name map
"""
import json
import random
from PIL import Image

T = 24          # tile size
COLS = 8

# ---- palette ---------------------------------------------------------------
P = {
    "outline": (12, 14, 20),
    "void":    (18, 20, 28),
    "d1":      (30, 34, 46),
    "d2":      (42, 48, 62),
    "m1":      (58, 66, 82),
    "m2":      (78, 88, 106),
    "l1":      (106, 118, 138),
    "l2":      (146, 158, 176),
    "cyan":    (70, 214, 230),
    "cyan_d":  (28, 120, 140),
    "cyan_dd": (18, 60, 74),
    "amber":   (232, 168, 52),
    "amber_d": (130, 84, 26),
    "red":     (214, 62, 62),
    "red_d":   (110, 28, 34),
    "green":   (110, 210, 80),
    "green_d": (42, 104, 40),
    "rust":    (120, 70, 44),
    "acid":    (150, 230, 60),
    "acid_d":  (70, 130, 30),
    "soot":    (24, 24, 26),
}


def new():
    return Image.new("RGBA", (T, T), (0, 0, 0, 0))


def px(img, x, y, c, a=255):
    if 0 <= x < T and 0 <= y < T:
        img.putpixel((x, y), (*P[c], a) if isinstance(c, str) else c)


def rect(img, x0, y0, x1, y1, c):
    for y in range(y0, y1 + 1):
        for x in range(x0, x1 + 1):
            px(img, x, y, c)


def hline(img, x0, x1, y, c):
    rect(img, x0, y, x1, y, c)


def vline(img, x, y0, y1, c):
    rect(img, x, y0, x, y1, c)


def bevel(img, x0, y0, x1, y1, face, hi, lo):
    rect(img, x0, y0, x1, y1, face)
    hline(img, x0, x1, y0, hi)
    vline(img, x0, y0, y1, hi)
    hline(img, x0, x1, y1, lo)
    vline(img, x1, y0, y1, lo)


def noise(img, rng, colors, n, x0=0, y0=0, x1=T - 1, y1=T - 1):
    for _ in range(n):
        px(img, rng.randint(x0, x1), rng.randint(y0, y1), rng.choice(colors))


def rivet(img, x, y):
    px(img, x, y, "l2")
    px(img, x + 1, y + 1, "d1")


# ---- floors ----------------------------------------------------------------
def floor_plate(seed, scratches=0, dirt=0):
    rng = random.Random(seed)
    img = new()
    # two 12x24 plates side by side reads as a big panelled deck
    for ox in (0, 12):
        bevel(img, ox, 0, ox + 11, T - 1, "d2", "m1", "d1")
        rivet(img, ox + 2, 2)
        rivet(img, ox + 8, 2)
        rivet(img, ox + 2, T - 4)
        rivet(img, ox + 8, T - 4)
    noise(img, rng, ["d1", "m1"], 6, 1, 1, T - 2, T - 2)
    for _ in range(scratches):
        x, y = rng.randint(3, 18), rng.randint(4, 19)
        for i in range(rng.randint(3, 6)):
            px(img, x + i, y + i // 2, "m1")
    for _ in range(dirt):
        x, y = rng.randint(2, 20), rng.randint(2, 20)
        for dx, dy in ((0, 0), (1, 0), (0, 1), (1, 1), (2, 1)):
            px(img, x + dx, y + dy, "d1")
    return img


def floor_grate():
    img = new()
    rect(img, 0, 0, T - 1, T - 1, "void")
    bevel(img, 0, 0, T - 1, T - 1, "void", "m1", "d1")
    for y in range(2, T - 2, 3):
        hline(img, 2, T - 3, y, "d2")
        hline(img, 2, T - 3, y + 1, "outline")
    vline(img, 11, 1, T - 2, "m1")
    vline(img, 12, 1, T - 2, "d1")
    # something glowing under the grate
    for x in (5, 6, 17):
        px(img, x, 8, "cyan_dd")
        px(img, x, 14, "cyan_dd")
    return img


def floor_hazard():
    img = new()
    rect(img, 0, 0, T - 1, T - 1, "amber_d")
    for y in range(T):
        for x in range(T):
            if ((x + y) // 4) % 2 == 0:
                px(img, x, y, "amber")
            else:
                px(img, x, y, "d1")
    hline(img, 0, T - 1, 0, "m2")
    hline(img, 0, T - 1, T - 1, "outline")
    rng = random.Random(7)
    noise(img, rng, ["amber_d", "d2"], 18)  # worn paint
    return img


def floor_vent():
    img = floor_plate(11)
    cx, cy, r = 11.5, 11.5, 8.5
    for y in range(T):
        for x in range(T):
            d = ((x - cx) ** 2 + (y - cy) ** 2) ** 0.5
            if d <= r:
                px(img, x, y, "d1")
            if r - 1 < d <= r:
                px(img, x, y, "l1" if y < cy else "outline")
    for y in range(5, 19, 2):
        for x in range(4, 20):
            if ((x - cx) ** 2 + (y - cy) ** 2) ** 0.5 < r - 1.5:
                px(img, x, y, "outline")
    rect(img, 10, 10, 13, 13, "m2")
    rect(img, 11, 11, 12, 12, "l1")
    return img


def floor_lightstrip():
    img = floor_plate(23)
    rect(img, 0, 10, T - 1, 13, "d1")
    hline(img, 0, T - 1, 10, "outline")
    hline(img, 0, T - 1, 11, "cyan_d")
    hline(img, 0, T - 1, 12, "cyan")
    hline(img, 0, T - 1, 13, "cyan_d")
    for x in range(0, T, 6):
        px(img, x, 12, (220, 255, 255, 255))
    return img


# ---- walls (3/4 top-down: "top" is the roof cap, "face" is the visible front)
def wall_top():
    img = new()
    rect(img, 0, 0, T - 1, T - 1, "void")
    rect(img, 0, 0, T - 1, T - 1, "d1")
    for y in range(0, T, 6):
        hline(img, 0, T - 1, y, "void")
    for x in range(0, T, 8):
        for y in range(0, T, 6):
            px(img, x + (4 if (y // 6) % 2 else 0), y + 3, "d2")
    return img


def wall_face(variant="plain"):
    img = new()
    # cap edge
    rect(img, 0, 0, T - 1, 3, "d2")
    hline(img, 0, T - 1, 0, "l1")
    hline(img, 0, T - 1, 3, "outline")
    # front panels
    bevel(img, 0, 4, 11, T - 3, "m1", "m2", "d1")
    bevel(img, 12, 4, T - 1, T - 3, "m1", "m2", "d1")
    # skirting / floor contact shadow
    rect(img, 0, T - 2, T - 1, T - 1, "outline")
    hline(img, 0, T - 1, T - 3, "d1")
    if variant == "plain":
        for ox in (0, 12):
            hline(img, ox + 3, ox + 8, 9, "d2")
            hline(img, ox + 3, ox + 8, 10, "m2")
            rivet(img, ox + 2, 17)
            rivet(img, ox + 8, 17)
    elif variant == "light":
        for ox in (0, 12):
            rect(img, ox + 3, 7, ox + 8, 10, "outline")
            rect(img, ox + 4, 8, ox + 7, 9, "cyan")
            px(img, ox + 4, 8, (220, 255, 255, 255))
            # glow spill
            hline(img, ox + 3, ox + 8, 11, "cyan_dd")
    elif variant == "pipe":
        rect(img, 0, 12, T - 1, 16, "rust")
        hline(img, 0, T - 1, 12, "outline")
        hline(img, 0, T - 1, 13, (170, 110, 70, 255))
        hline(img, 0, T - 1, 16, "outline")
        for x in (5, 17):
            rect(img, x, 11, x + 2, 17, "m2")
            vline(img, x + 2, 11, 17, "d1")
    elif variant == "warning":
        for ox in (0, 12):
            for y in range(7, 17):
                for x in range(ox + 3, ox + 9):
                    px(img, x, y, "red" if ((x + y) // 3) % 2 else "d1")
            rect(img, ox + 3, 7, ox + 8, 7, "outline")
    return img


def door(open_frac):
    """Horizontal sliding door on a wall-face tile. open_frac 0..1."""
    img = new()
    rect(img, 0, 0, T - 1, 3, "d2")
    hline(img, 0, T - 1, 0, "l1")
    hline(img, 0, T - 1, 3, "outline")
    # frame
    bevel(img, 0, 4, 2, T - 1, "m2", "l1", "d1")
    bevel(img, T - 3, 4, T - 1, T - 1, "m2", "l1", "d1")
    # doorway darkness
    rect(img, 3, 4, T - 4, T - 1, "outline")
    gap = int(round(open_frac * 9))
    left_end = 11 - gap
    right_start = 12 + gap
    if left_end >= 3:
        bevel(img, 3, 4, left_end, T - 1, "m1", "m2", "d1")
        for y in range(6, T - 1, 4):
            hline(img, 4, left_end - 1, y, "amber" if (y // 4) % 2 else "d1")
    if right_start <= T - 4:
        bevel(img, right_start, 4, T - 4, T - 1, "m1", "m2", "d1")
        for y in range(6, T - 1, 4):
            hline(img, right_start + 1, T - 5, y, "amber" if (y // 4) % 2 else "d1")
    # status light on the lintel
    px(img, 11, 2, "green" if open_frac > 0.5 else "red")
    px(img, 12, 2, "green" if open_frac > 0.5 else "red")
    return img


# ---- props -----------------------------------------------------------------
def crate():
    img = new()
    # drop shadow
    rect(img, 4, 20, 22, 22, (0, 0, 0, 90))
    bevel(img, 2, 3, 20, 20, "rust", (170, 110, 70, 255), (70, 40, 26, 255))
    rect(img, 1, 2, 21, 2, "outline")
    vline(img, 1, 2, 21, "outline")
    vline(img, 21, 2, 21, "outline")
    hline(img, 1, 21, 21, "outline")
    # X bracing
    for i in range(16):
        px(img, 3 + i, 4 + i, (90, 52, 32, 255))
        px(img, 19 - i, 4 + i, (90, 52, 32, 255))
    rect(img, 8, 9, 14, 14, "amber_d")
    rect(img, 9, 10, 13, 13, "amber")
    px(img, 11, 11, "outline")
    px(img, 11, 12, "outline")
    return img


def console():
    img = new()
    rect(img, 3, 20, 22, 22, (0, 0, 0, 90))
    bevel(img, 2, 6, 21, 20, "m1", "m2", "d1")
    rect(img, 2, 5, 21, 5, "outline")
    hline(img, 2, 21, 21, "outline")
    # screen
    rect(img, 4, 7, 19, 13, "outline")
    rect(img, 5, 8, 18, 12, "cyan_dd")
    for x in range(5, 19):
        y = 10 + int(2 * __import__("math").sin(x * 0.9))
        px(img, x, y, "cyan")
    hline(img, 5, 9, 8, "cyan_d")
    # buttons
    for i, c in enumerate(("red", "amber", "green", "cyan", "amber")):
        px(img, 5 + i * 3, 16, c)
        px(img, 5 + i * 3, 17, "d1")
    return img


def barrel(kind="toxic"):
    img = new()
    face, hi, lo = ("green_d", "green", (24, 60, 24, 255)) if kind == "toxic" else ("red_d", "red", (60, 16, 18, 255))
    rect(img, 6, 21, 20, 22, (0, 0, 0, 90))
    rect(img, 6, 4, 17, 20, face)
    vline(img, 7, 4, 20, hi)
    vline(img, 8, 4, 20, hi)
    vline(img, 16, 4, 20, lo)
    vline(img, 17, 4, 20, lo)
    vline(img, 5, 4, 20, "outline")
    vline(img, 18, 4, 20, "outline")
    hline(img, 6, 17, 3, "outline")
    hline(img, 6, 17, 21, "outline")
    for y in (8, 15):
        hline(img, 6, 17, y, "outline")
        hline(img, 7, 16, y + 1, lo)
    # lid
    rect(img, 7, 4, 16, 6, "m2")
    hline(img, 7, 16, 4, "l2")
    px(img, 13, 5, "outline")
    # symbol
    rect(img, 10, 11, 13, 12, "amber")
    return img


def locker():
    img = new()
    rect(img, 3, 21, 21, 22, (0, 0, 0, 90))
    rect(img, 2, 1, 21, 21, "outline")
    for ox in (3, 12):
        bevel(img, ox, 2, ox + 8, 20, "m2", "l1", "d2")
        for y in (4, 6, 8):
            hline(img, ox + 2, ox + 6, y, "d1")
        px(img, ox + 6, 13, "amber")
        px(img, ox + 6, 14, "amber_d")
    return img


def terminal_pillar():
    img = new()
    rect(img, 5, 20, 19, 22, (0, 0, 0, 90))
    bevel(img, 6, 2, 17, 20, "d2", "m1", "outline")
    rect(img, 8, 4, 15, 9, "outline")
    rect(img, 9, 5, 14, 8, "amber_d")
    hline(img, 9, 12, 6, "amber")
    hline(img, 9, 14, 7, "amber")
    for y in range(12, 19, 2):
        hline(img, 8, 15, y, "m1")
    px(img, 11, 1, "red")
    px(img, 12, 1, "red")
    return img


# ---- decals ---------------------------------------------------------------
def decal_acid():
    rng = random.Random(42)
    img = new()
    blobs = [(12, 12, 6), (6, 8, 3), (18, 17, 3), (16, 6, 2), (7, 18, 2)]
    for cx, cy, r in blobs:
        for y in range(T):
            for x in range(T):
                d = ((x - cx) ** 2 + (y - cy) ** 2) ** 0.5 + rng.random() * 1.2
                if d < r:
                    px(img, x, y, "acid_d")
                if d < r - 1.5:
                    px(img, x, y, "acid")
    noise(img, rng, [(230, 255, 160, 255)], 6, 8, 8, 16, 16)
    return img


def decal_scorch():
    rng = random.Random(9)
    img = new()
    for y in range(T):
        for x in range(T):
            d = ((x - 11.5) ** 2 + (y - 11.5) ** 2) ** 0.5 + rng.random() * 2.5
            if d < 10:
                a = int(255 * min(1, 1.4 * (1 - d / 10)))
                img.putpixel((x, y), (*P["soot"], max(a, 0)))
    for _ in range(6):
        ang = rng.random() * 6.283
        import math
        for i in range(4, 11):
            px(img, int(11.5 + math.cos(ang) * i), int(11.5 + math.sin(ang) * i), (*P["soot"], 180))
    return img


def decal_cables():
    img = new()
    import math
    for off, c in ((0, "d1"), (3, "red_d"), (6, "cyan_dd")):
        for x in range(T):
            y = int(10 + off + 3 * math.sin(x * 0.35 + off))
            px(img, x, y, "outline")
            px(img, x, y - 1, c)
    return img



# ---- auto-tiling wall caps -------------------------------------------------
# Mask bits say which sides of a cap touch open floor: N=1, E=2, S=4, W=8.
# Cap index in the sheet = WALL_CAP_BASE + mask.
N_, E_, S_, W_ = 1, 2, 4, 8


def wall_cap(mask):
    img = wall_top()
    if mask & N_:
        hline(img, 0, T - 1, 0, "outline")
        hline(img, 0, T - 1, 1, "l1")
        hline(img, 0, T - 1, 2, "m1")
    if mask & S_:
        hline(img, 0, T - 1, T - 1, "outline")
        hline(img, 0, T - 1, T - 2, "m1")
        hline(img, 0, T - 1, T - 3, "d2")
    if mask & W_:
        vline(img, 0, 0, T - 1, "outline")
        vline(img, 1, 0, T - 1, "m2")
        vline(img, 2, 0, T - 1, "d2")
    if mask & E_:
        vline(img, T - 1, 0, T - 1, "outline")
        vline(img, T - 2, 0, T - 1, "m1")
        vline(img, T - 3, 0, T - 1, "d2")
    # tidy outer corners
    for bit_a, bit_b, (cx, cy) in ((N_, W_, (0, 0)), (N_, E_, (T - 1, 0)),
                                    (S_, W_, (0, T - 1)), (S_, E_, (T - 1, T - 1))):
        if mask & bit_a and mask & bit_b:
            px(img, cx, cy, "void")
    return img


def wall_inner_corner(which):
    """Overlay for a cap whose two orthogonal neighbours are wall but the
    diagonal is floor. which in NE, NW, SE, SW."""
    img = new()
    x = T - 3 if "E" in which else 0
    y = 0 if "N" in which else T - 3
    rect(img, x, y, x + 2, y + 2, "m1")
    cx = T - 1 if "E" in which else 0
    cy = 0 if "N" in which else T - 1
    px(img, cx, cy, "outline")
    px(img, cx + (-1 if "E" in which else 1), cy, "outline")
    px(img, cx, cy + (1 if "N" in which else -1), "outline")
    return img


def build_tiles():
    tiles = [
        ("floor_plate_a", floor_plate(1)),
        ("floor_plate_b", floor_plate(2, scratches=3)),
        ("floor_plate_dirty", floor_plate(3, scratches=1, dirt=4)),
        ("floor_grate", floor_grate()),
        ("floor_hazard", floor_hazard()),
        ("floor_vent", floor_vent()),
        ("floor_lightstrip", floor_lightstrip()),
        ("wall_top", wall_top()),
        ("wall_face", wall_face("plain")),
        ("wall_face_light", wall_face("light")),
        ("wall_face_pipe", wall_face("pipe")),
        ("wall_face_warning", wall_face("warning")),
        ("door_closed", door(0.0)),
        ("door_half", door(0.5)),
        ("door_open", door(1.0)),
        ("crate", crate()),
        ("console", console()),
        ("barrel_toxic", barrel("toxic")),
        ("barrel_explosive", barrel("explosive")),
        ("locker", locker()),
        ("terminal", terminal_pillar()),
        ("decal_acid", decal_acid()),
        ("decal_scorch", decal_scorch()),
        ("decal_cables", decal_cables()),
    ]
    # rows 4-5: the 16 cap masks, row 6: inner corners
    for m in range(16):
        tiles.append((f"wall_cap_{m:02d}", wall_cap(m)))
    for c in ("NE", "NW", "SE", "SW"):
        tiles.append((f"wall_inner_{c}", wall_inner_corner(c)))
    return tiles


WALL_CHARS = set("#LP!D=")
FACE_VARIANT = {"#": "wall_face", "L": "wall_face_light", "P": "wall_face_pipe",
                "!": "wall_face_warning", "D": "door_closed", "=": "door_open"}
FLOOR_CHARS = {"g": "floor_grate", "h": "floor_hazard", "v": "floor_vent",
               "s": "floor_lightstrip"}
PROP_CHARS = {"b": "barrel_toxic", "B": "barrel_explosive", "c": "console", "x": "crate",
              "k": "locker", "t": "terminal"}
DECAL_CHARS = {"a": "decal_acid", "z": "decal_scorch", "~": "decal_cables"}


def is_wall(rows, x, y):
    if y < 0 or y >= len(rows) or x < 0 or x >= len(rows[0]):
        return True
    return rows[y][x] in WALL_CHARS


def wall_tile_for(rows, x, y):
    """The auto-tiling rule, also implemented the same way in the game:
    a wall cell with floor directly below shows its front face; any other
    wall cell is a roof cap chosen by which sides touch floor."""
    if not is_wall(rows, x, y + 1):
        return [FACE_VARIANT[rows[y][x]]]
    mask = 0
    if not is_wall(rows, x, y - 1): mask |= N_
    if not is_wall(rows, x + 1, y): mask |= E_
    if not is_wall(rows, x, y + 1): mask |= S_
    if not is_wall(rows, x - 1, y): mask |= W_
    out = [f"wall_cap_{mask:02d}"]
    for dx, dy, name in ((1, -1, "NE"), (-1, -1, "NW"), (1, 1, "SE"), (-1, 1, "SW")):
        if is_wall(rows, x + dx, y) and is_wall(rows, x, y + dy) and not is_wall(rows, x + dx, y + dy):
            out.append(f"wall_inner_{name}")
    return out


def render_room(rows, tiles_by_name, seed=5, with_props=True):
    """Floor + walls (+ props/decals). Returns an RGBA image at 1x."""
    H, W = len(rows), len(rows[0])
    room = Image.new("RGBA", (W * T, H * T), P["void"] + (255,))
    rng = random.Random(seed)
    floors = ["floor_plate_a"] * 5 + ["floor_plate_b"] * 2 + ["floor_plate_dirty"]
    for y, row in enumerate(rows):
        for x, ch in enumerate(row):
            pos = (x * T, y * T)
            if ch in WALL_CHARS:
                names = wall_tile_for(rows, x, y)
                if names[0].startswith("door") or names[0].startswith("wall_face"):
                    base = tiles_by_name["floor_plate_a"]
                    room.paste(base, pos, base)
                for n in names:
                    room.paste(tiles_by_name[n], pos, tiles_by_name[n])
                continue
            base = tiles_by_name[FLOOR_CHARS.get(ch) or rng.choice(floors)]
            room.paste(base, pos, base)
            if with_props:
                for table in (DECAL_CHARS, PROP_CHARS):
                    if ch in table:
                        img = tiles_by_name[table[ch]]
                        room.paste(img, pos, img)
    return room


# A room with an irregular shape to exercise the auto-tiling.
SAMPLE_MAP = [
    "####################",
    "#L#P#L#D##L#P#L#####",
    "#..b..c.....v..k.k##",
    "#..........a.....x.#",
    "#hhhh....s.........#",
    "#.......z..........#",
    "######=####...##...#",
    "#L#L##.#L##...##..g#",
    "#..........~......g#",
    "#.c..x.........B...#",
    "#......t.......xx..#",
    "####################",
]


def main():
    tiles = build_tiles()
    rows = (len(tiles) + COLS - 1) // COLS
    sheet = Image.new("RGBA", (COLS * T, rows * T), (0, 0, 0, 0))
    index = {}
    for i, (name, img) in enumerate(tiles):
        sheet.paste(img, ((i % COLS) * T, (i // COLS) * T), img)
        index[i] = name
    sheet.save("tileset.png")
    sheet.resize((sheet.width * 4, sheet.height * 4), Image.NEAREST).save("tileset_x4.png")
    names = [n for n, _ in tiles]
    with open("tiles.json", "w") as f:
        json.dump({
            "tile_size": T,
            "columns": COLS,
            "tiles": index,
            "autotile": {
                "rule": "wall cell with floor below -> wall_face*; else wall_cap_{mask} where mask bits mark sides touching floor (N=1,E=2,S=4,W=8); then overlay wall_inner_{NE,NW,SE,SW} where both orthogonal neighbours are wall but the diagonal is floor. Off-map counts as wall.",
                "wall_cap_base_index": names.index("wall_cap_00"),
                "wall_inner_index": {c: names.index(f"wall_inner_{c}") for c in ("NE", "NW", "SE", "SW")},
            },
        }, f, indent=2)
    by_name = dict(tiles)
    room = render_room(SAMPLE_MAP, by_name)
    room.resize((room.width * 3, room.height * 3), Image.NEAREST).save("preview_x3.png")
    print("tiles:", len(tiles), "sheet:", sheet.size)


if __name__ == "__main__":
    main()
