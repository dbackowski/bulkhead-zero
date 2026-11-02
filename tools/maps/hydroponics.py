#!/usr/bin/env python3
"""Lays out the Hydroponics map (deck 3) and writes it into internal/world/maps.go.

Same idea as reactor.py: rooms, openings, doors, props and spawns placed by
coordinate.

    python3 tools/maps/hydroponics.py          # rewrite the Hydroponics block in maps.go
    python3 tools/maps/hydroponics.py --print  # just print it with column numbers

The legend is in internal/world/level.go. After a change, run
go test ./internal/world (TestMapsAreConnected) and keep explosive barrels
and enemies off the main routes (see "Difficulty" in CLAUDE.md).
"""
import sys
from pathlib import Path

MAPS = Path(__file__).resolve().parents[2] / "internal/world/maps.go"

W, H = 44, 28
g = [["#"] * W for _ in range(H)]


def fill(x0, y0, x1, y1, c="."):
    for y in range(y0, y1 + 1):
        for x in range(x0, x1 + 1):
            g[y][x] = c


def put(x, y, c):
    g[y][x] = c


# NW and NE greenhouses joined by a north corridor, the grow hall in the
# middle (the Maw's arena), side corridors west and east, and the entry (SW)
# and pump room (SE) joined by a south corridor.
fill(1, 2, 14, 9)
fill(29, 2, 42, 9)
fill(15, 4, 28, 6)
fill(8, 12, 35, 19)
fill(4, 10, 6, 21)
fill(37, 10, 39, 21)
fill(1, 22, 14, 26)
fill(29, 22, 42, 26)
fill(15, 23, 28, 24)
# openings into the hall: west, east, north (wide), south (an inner door)
fill(7, 14, 7, 17); fill(36, 14, 36, 17)
fill(20, 7, 23, 11)
put(21, 20, '.'); put(21, 21, 'D'); put(21, 22, '.')
# outer doors (wave spawns): the greenhouses, the north corridor, the pump room
for x in (8, 35): put(x, 1, 'D')
put(21, 3, 'D')
put(33, 21, 'D')
# wall faces
for x in (3, 12, 31, 40): put(x, 1, 'L')
for x in (6, 37): put(x, 1, '!')
for x in (17, 25): put(x, 3, 'L')
for x in (9, 15, 28, 34): put(x, 11, 'P')
for x in (31, 41): put(x, 21, 'P')
for x in (2, 11): put(x, 21, 'L')
# floors: grow beds in the greenhouses and the hall, a grate walkway, vents
for y in (4, 7):
    fill(3, y, 12, y, 'o'); fill(31, y, 40, y, 'o')
for y in (14, 17):
    fill(10, y, 17, y, 'o'); fill(26, y, 33, y, 'o')
fill(19, 15, 24, 16, 'o')
fill(15, 5, 28, 5, 'g')
fill(21, 12, 22, 13, 'g')
for x, y in ((5, 12), (38, 12), (8, 24), (36, 24)): put(x, y, 'v')
fill(15, 24, 28, 24, 's')
fill(39, 26, 42, 26, 'h')
# props
for x, y in ((1, 22), (2, 22)): put(x, y, 'k')              # lockers at the entry
for x, y in ((41, 22), (42, 22), (42, 23)): put(x, y, 'c')  # pump controls
put(29, 22, 't'); put(1, 2, 't'); put(42, 2, 't')
for x, y in ((13, 13), (13, 18), (30, 13), (30, 18), (6, 3), (37, 8), (12, 25), (31, 25)):
    put(x, y, 'x')
for x, y in ((14, 2), (29, 9), (40, 25), (41, 25)): put(x, y, 'b')  # fertiliser
for x, y in ((1, 9), (42, 9)): put(x, y, 'B')
# decals: the infestation is thick here
for x, y in ((9, 3), (26, 4), (16, 13), (27, 18), (33, 6), (5, 17), (38, 19), (24, 12), (33, 23)):
    put(x, y, 'a')
for x, y in ((11, 8), (25, 19)): put(x, y, 'z')
for x, y in ((18, 6), (6, 25), (34, 26)): put(x, y, '~')
# spawns
put(4, 24, '@')
put(21, 15, 'Q')
for x, y in ((10, 15), (32, 16)): put(x, y, 'R')
for x, y in ((40, 3), (34, 9), (2, 6), (39, 23)): put(x, y, 'S')
for x, y in ((5, 5), (11, 3), (24, 6), (33, 5), (38, 6), (5, 14), (38, 16), (17, 16), (27, 13), (35, 25)):
    put(x, y, 'C')
put(13, 6, 'E'); put(36, 5, 'E')
put(9, 8, 'G')
rows = ["".join(r) for r in g]


def main():
    if "--print" in sys.argv:
        for i, r in enumerate(rows):
            print(f"{i:2d} {r}")
        print("   " + "".join(str(i % 10) for i in range(W)))
        return
    src = MAPS.read_text()
    block = "var Hydroponics = []string{\n" + "".join(f'\t"{r}",\n' for r in rows) + "}\n"
    if "var Hydroponics = []string{" not in src:
        src += "\n// Hydroponics is the third deck: two greenhouses (north), the grow hall\n" \
               "// where the Maw lives (centre), side corridors, the entry (south-west)\n" \
               "// and the pump room (south-east). Laid out by tools/maps/hydroponics.py.\n" + block
    else:
        start = src.index("var Hydroponics = []string{")
        end = src.index("}\n", start) + 2
        src = src[:start] + block + src[end:]
    MAPS.write_text(src)
    print(f"wrote the Hydroponics map ({W}x{H}) into {MAPS.relative_to(MAPS.parents[2])}")


if __name__ == "__main__":
    main()
