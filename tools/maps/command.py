#!/usr/bin/env python3
"""Lays out the Command map (deck 4) and writes it into internal/world/maps.go.

Same idea as reactor.py: rooms, openings, doors, props and spawns placed by
coordinate.

    python3 tools/maps/command.py          # rewrite the Command block in maps.go
    python3 tools/maps/command.py --print  # just print it with column numbers

The legend is in internal/world/level.go. After a change, run
go test ./internal/world (TestMapsAreConnected). The Overmind sits at Q and
grows its nodes about 3 tiles around it, so keep that ring free of props.
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


# The observation gallery (north), the command hall (centre, the Overmind's
# arena), crew quarters (west), the server room (east), and the entry
# (south) where the player starts.
fill(1, 2, 42, 5)
fill(10, 9, 33, 20)
fill(1, 9, 7, 20)
fill(36, 9, 42, 20)
fill(1, 23, 42, 26)
# openings
fill(2, 6, 4, 8); fill(39, 6, 41, 8)            # gallery <-> wings
fill(20, 6, 23, 8)                              # gallery <-> hall, wide
fill(8, 13, 9, 15); fill(34, 13, 35, 15)        # wings <-> hall
fill(13, 21, 15, 22); fill(28, 21, 30, 22)      # hall <-> entry
# inner doors: floor above and below
put(4, 21, '.'); put(4, 22, 'D')
put(39, 21, '.'); put(39, 22, 'D')
# outer doors (wave spawns): the top wall, and the band above the entry
for x in (7, 15, 28, 36): put(x, 1, 'D')
put(9, 22, 'D'); put(34, 22, 'D')
# wall faces
for x in (3, 11, 19, 24, 32, 40): put(x, 1, 'L')
for x in (13, 30): put(x, 1, '!')
for x in (12, 17, 26, 31): put(x, 8, 'P')
for x in (2, 6, 37, 41): put(x, 8, 'L')
for x in (19, 24): put(x, 22, '!')
# floors: a hazard ring around the Overmind's dais, grate walkways, light strips
fill(19, 12, 24, 17, 'h')
fill(20, 13, 23, 16, '.')
fill(10, 10, 33, 10, 's'); fill(10, 19, 33, 19, 's')
fill(1, 3, 42, 3, 'g')
for x, y in ((4, 11), (39, 11), (8, 25), (35, 25)): put(x, y, 'v')
fill(1, 26, 3, 26, 'h'); fill(40, 26, 42, 26, 'h')
# props: consoles along the gallery, bunks (lockers) west, servers (terminals) east
for x in (9, 10, 11, 32, 33, 34): put(x, 2, 'c')
for x, y in ((1, 9), (1, 10), (1, 17), (1, 18), (7, 9)): put(x, y, 'k')
for x, y in ((42, 9), (42, 10), (42, 11), (42, 16), (42, 17), (42, 18), (36, 9)): put(x, y, 't')
for x, y in ((12, 11), (12, 18), (31, 11), (31, 18), (16, 9), (27, 9),   # cover around the hall's edge
             (5, 15), (38, 14), (18, 24), (25, 24), (12, 25), (31, 25)):
    put(x, y, 'x')
for x, y in ((1, 5), (42, 5), (1, 20), (42, 20)): put(x, y, 'B')
for x, y in ((7, 20), (36, 20), (20, 26), (23, 26)): put(x, y, 'b')
# decals: the hive has crept in
for x, y in ((14, 4), (29, 12), (11, 16), (37, 17), (6, 12), (22, 25)): put(x, y, 'a')
for x, y in ((25, 3), (15, 14), (34, 24)): put(x, y, 'z')
for x, y in ((21, 18), (5, 3), (38, 3), (16, 23), (27, 23)): put(x, y, '~')
# spawns
put(4, 24, '@')
put(21, 14, 'Q')
for x, y in ((8, 4), (35, 4), (26, 13)): put(x, y, 'R')
for x, y in ((40, 13), (3, 12)): put(x, y, 'S')
for x, y in ((16, 4), (27, 4), (14, 12), (29, 17), (39, 18), (5, 18)): put(x, y, 'C')
for x, y in ((21, 3), (17, 16)): put(x, y, 'U')
for x, y in ((24, 10), (11, 14)): put(x, y, 'Y')
put(40, 15, 'N')
put(30, 14, 'G')
put(33, 4, 'E')
rows = ["".join(r) for r in g]


def main():
    if "--print" in sys.argv:
        for i, r in enumerate(rows):
            print(f"{i:2d} {r}")
        print("   " + "".join(str(i % 10) for i in range(W)))
        return
    src = MAPS.read_text()
    block = "var Command = []string{\n" + "".join(f'\t"{r}",\n' for r in rows) + "}\n"
    if "var Command = []string{" not in src:
        src += "\n// Command is the fourth deck: the observation gallery (north), the command\n" \
               "// hall where the Overmind sits (centre), crew quarters (west), the server\n" \
               "// room (east) and the entry (south). Laid out by tools/maps/command.py.\n" + block
    else:
        start = src.index("var Command = []string{")
        end = src.index("}\n", start) + 2
        src = src[:start] + block + src[end:]
    MAPS.write_text(src)
    print(f"wrote the Command map ({W}x{H}) into {MAPS.relative_to(MAPS.parents[2])}")


if __name__ == "__main__":
    main()
