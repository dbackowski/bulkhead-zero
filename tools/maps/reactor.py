#!/usr/bin/env python3
"""Lays out the Reactor map (deck 2) and writes it into internal/world/maps.go.

Typing a 44x28 map by hand invites off-by-one columns, so the rooms,
openings, doors, props and spawns are placed here by coordinate instead:

    python3 tools/maps/reactor.py          # rewrite the Reactor block in maps.go
    python3 tools/maps/reactor.py --print  # just print it with column numbers

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


# A north turbine hall, B reactor chamber, C west control, D east coolant, E south cargo bay
fill(1, 2, 42, 7)
fill(8, 10, 35, 19)
fill(1, 10, 6, 19)
fill(37, 10, 42, 19)
fill(1, 22, 42, 26)
# openings
fill(2, 8, 4, 9); fill(39, 8, 41, 9)            # wide gaps north hall <-> wings
fill(7, 14, 7, 15); fill(36, 14, 36, 15)        # wings <-> chamber
fill(13, 20, 15, 21); fill(28, 20, 30, 21)      # chamber <-> cargo bay
# inner doors: corridor cell above, door at the bottom of the wall band
put(21, 8, '.'); put(21, 9, 'D')
put(4, 20, '.'); put(4, 21, 'D')
put(39, 20, '.'); put(39, 21, 'D')
# reactor core: solid block, hazard ring around it
fill(17, 12, 26, 17, 'h')
fill(18, 13, 25, 16, '#')
for x in (19, 21, 22, 24): put(x, 16, '!')
put(20, 16, 'L'); put(23, 16, 'L')
# outer doors (wave spawns): top wall, and the band above the cargo bay
for x in (6, 21, 37): put(x, 1, 'D')
put(9, 21, 'D'); put(34, 21, 'D')
# wall faces
for x in (2, 11, 15, 27, 31, 41): put(x, 1, 'L')
for x in (13, 29): put(x, 1, 'P')
for x in (17, 25): put(x, 1, '!')
for x in (10, 18, 25, 33): put(x, 9, 'P')
for x in (7, 23): put(x, 21, 'L')
# floors: grate walkways, light strips, vents
fill(9, 4, 34, 4, 'g')
fill(8, 18, 16, 18, 's'); fill(27, 18, 35, 18, 's')
for x, y in ((3, 12), (40, 12), (12, 24), (31, 24)): put(x, y, 'v')
fill(1, 26, 4, 26, 'h'); fill(39, 26, 42, 26, 'h')
# props
for x, y in ((9, 2), (10, 2)): put(x, y, "k")              # lockers
for x, y in ((1, 12), (1, 13), (2, 10), (3, 10)): put(x, y, 'c')  # control consoles
put(42, 10, 't'); put(41, 10, 't')
for x, y in ((12, 5), (13, 5), (30, 5), (31, 5), (11, 12), (11, 13), (32, 16), (32, 17),
             (19, 24), (20, 24), (24, 23), (25, 23), (8, 25), (35, 25), (36, 25)):
    put(x, y, 'x')
for x, y in ((1, 7), (42, 7), (35, 19), (38, 18), (21, 25)): put(x, y, 'B')
for x, y in ((40, 16), (41, 16), (42, 19), (1, 24)): put(x, y, 'b')
# decals
for x, y in ((10, 7), (33, 11), (9, 17), (26, 22), (40, 25)): put(x, y, 'a')
for x, y in ((22, 3), (15, 11), (5, 23)): put(x, y, 'z')
for x, y in ((3, 6), (36, 3), (17, 22)): put(x, y, '~')
# spawns
put(4, 24, '@')
put(21, 19, 'Q')
for x, y in ((8, 3), (35, 6), (30, 14)): put(x, y, 'R')
for x, y in ((40, 13), (34, 10), (40, 3)): put(x, y, 'S')
for x, y in ((15, 3), (26, 5), (39, 4), (20, 11), (27, 19), (4, 13), (38, 15), (32, 23)): put(x, y, 'C')
put(23, 6, 'G')
rows = ["".join(r) for r in g]


def main():
    if "--print" in sys.argv:
        for i, r in enumerate(rows):
            print(f"{i:2d} {r}")
        print("   " + "".join(str(i % 10) for i in range(W)))
        return
    src = MAPS.read_text()
    start = src.index("var Reactor = []string{")
    end = src.index("}\n", start) + 2
    block = "var Reactor = []string{\n" + "".join(f'\t"{r}",\n' for r in rows) + "}\n"
    MAPS.write_text(src[:start] + block + src[end:])
    print(f"wrote the Reactor map ({W}x{H}) into {MAPS.relative_to(MAPS.parents[2])}")


if __name__ == "__main__":
    main()
