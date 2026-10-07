# Bulkhead Zero

Top-down sci-fi shooter: a lone marine clears an infested space station.
WASD to move, mouse to aim (full 360°), click to shoot.
Go + **Ebitengine v2** (`github.com/hajimehoshi/ebiten/v2`), pixel art at 24×24 per tile/frame.

**Status:** playable prototype. One station level (5 rooms, 17 enemies), the marine with
360° aiming and three weapons, crawlers / spitters / drones, explosive barrels, HUD, win/lose
and restart. See "Prototype: what exists" and "Next steps" at the end.

```
go run .                 # play (macOS: needs Xcode Command Line Tools for cgo)
go test ./...            # Linux without a display: xvfb-run go test ./...
python3 tools/artgen/build.py   # regenerate art (see "Art pipeline")
```

## Decisions so far (and why)

- **Real-time top-down shooter**, not turn-based tactics. A turn-based XCOM-style game was the
  other option; it's still a possible later mode, and the grid tiles would support it.
- **Own procedurally generated art** instead of the Oryx "16-Bit Sci-Fi" pack. The Oryx pack is
  also 24px and suits tactics, but it has no walk cycles, which matters for a real-time shooter.
  Our art is drawn by Python scripts (`tools/artgen/`), so it's ours with no licence questions
  and can be restyled by changing code.
- **Detached gun for aiming.** The player body has 8 facings; the weapon is a separate sprite
  rotated freely around the hand toward the cursor. Bullets use the exact aim angle. This is the
  Enter the Gungeon / Nuclear Throne approach: precise aim with little art, and swappable weapons.
- **3/4 top-down view** (SNES style): walls show a front face when floor is below them.

## Repo layout

```
assets/tiles/     tileset.png (8 cols × 24px), tiles.json (names + autotile rule)
assets/sprites/   player.png, enemies.png, fx.png (one animation per row, 24×24 frames)
                  weapons.png (24×12 per weapon), sprites.json (all metadata)
tools/artgen/     make_tiles.py, make_sprites.py, make_preview.py, build.py
docs/previews/    enlarged sheets + scene.gif / animations.gif / aiming.png for viewing only
```

Go code (module `github.com/dbackowski/bulkhead-zero`, Ebitengine **v2.10**, Go 1.25+):

```
main.go                 window setup, runs game.New()
assets/assets.go        //go:embed of tiles/ and sprites/
internal/geom           Vec (2D math)
internal/gfx            sprites.json/tiles.json parsing, Library (anims, tiles, weapons), draw helpers
internal/world          map text format, autotile, collision (MoveAndSlide), line of sight,
                        BFS flow field for enemy pathing; maps.go holds the Station level
internal/game           Game loop, input, player, enemies (AI + stats table), combat
                        (bullets, enemy shots, barrels), drawing (y-sorted), HUD, devtools
```

## Art pipeline: never hand-edit the PNGs

All art comes from code. Change the scripts, then regenerate:

```
pip3 install -r tools/artgen/requirements.txt   # Pillow
python3 tools/artgen/build.py                   # sheets + JSON + previews (~10 s)
python3 tools/artgen/build.py --no-previews     # just the game assets
```

`build.py` is deterministic (seeded RNG), so an unchanged script reproduces identical files.
Check new art by reading the PNGs in `docs/previews/` (look at them, don't just trust the run).
`make_preview.py` also contains a working Python reference of the player compositing and a tiny
fight simulation; port its logic rather than reinventing it.

## Asset reference

### Tiles (`assets/tiles/tiles.json`)
- Index `i` sits at column `i % 8`, row `i / 8`, 24×24 each. Names are in `tiles`.
- Floors: `floor_plate_a/b/dirty`, `floor_grate`, `floor_hazard`, `floor_vent`, `floor_lightstrip`.
- Wall faces: `wall_face`, `wall_face_light`, `wall_face_pipe`, `wall_face_warning`.
- Doors (wall-face tiles, also a 3-frame slide animation): `door_closed`, `door_half`, `door_open`.
- Props, drawn over a floor tile, with their own drop shadow: `crate`, `console`, `barrel_toxic`,
  `barrel_explosive`, `locker`, `terminal`. Decals (transparent overlays): `decal_acid`,
  `decal_scorch`, `decal_cables`.
- **Wall autotiling** (same rule as `make_tiles.wall_tile_for`): a wall cell with floor directly
  below draws its face variant; any other wall cell draws `wall_cap_{mask}` (index
  `wall_cap_base_index + mask`), where mask bits mark which sides touch floor
  N=1, E=2, S=4, W=8. Then overlay `wall_inner_{NE,NW,SE,SW}` when both orthogonal neighbours
  are wall but that diagonal is floor. Off-map counts as wall.
- The sample map format in `make_tiles.SAMPLE_MAP` (`#`/`L`/`P`/`!` walls, `D`/`=` doors,
  letters for props) is a reasonable starting point for a level format.

### Sprites (`assets/sprites/sprites.json`)
- `sheets.<file>.<anim>` gives `row`, `frames`, `frame_ms`, `loop` (and `bob` for player
  idle/walk). Frame `n` of an animation is at `(n*24, row*24)`.
- Every 24×24 frame has its **feet / ground contact at about (12, 21)**: anchor entities there and
  y-sort by feet.
- Only `*_right` facings are drawn; **left = the right frames flipped horizontally**
  (player diagonals and the crawler too). Spitter and drone face the camera and are not flipped.
- `loop: false` animations play once, then hold the last frame (death) or return to idle.
- Enemies: `crawler.walk_right/attack_right/death` (fast melee), `spitter.idle/attack/death`
  (stationary, lobs acid; the glob should leave on attack frame 2), `drone.idle/attack/death`
  (hovering, shoots a laser; it fires on attack frame 2).
- FX: `bullet`, `laser` (both point right: rotate to the heading), `hit_spark`, `acid_glob`,
  `acid_splash`, `explosion`, `muzzle_flash`, `crosshair`.

### Player compositing (the important part)
Facings: `right, down_right, down, down_left, left, up_left, up, up_right`.

1. `aim = atan2(mouse.y - hand.y, mouse.x - hand.x)` (screen space, +y down).
2. `facing = facings[round(deg(aim) / 45) mod 8]`; pick `idle_*`/`walk_*`/`hurt_*`, flipping
   `*_right` frames for left facings.
3. `hand = feet - (12, 21) + player.hand_anchor[facing] + (0, bob[frame])`.
4. Weapon (`weapons.png.<name>.rect`, drawn pointing right) with Ebitengine `GeoM`:
   `Translate(-pivot)` → if `cos(aim) < 0` then `Scale(1, -1)` (never upside down) →
   `Rotate(aim)` → `Translate(hand)`. After a shot, pull back by `recoil_px` along the aim for
   ~60 ms.
5. Draw order: gun **behind** the body when facing is in `gun_behind_body` (the up facings),
   otherwise in front.
6. Bullet spawn = `hand + rotate(muzzle - pivot, aim)`, with the offset's y negated when aiming
   left. Show `muzzle_flash` there.
7. Weapon stats live next to each weapon in `sprites.json`: `fire_ms`, `spread_deg`, `pellets`,
   `recoil_px`, `bullet_speed` (px/s), `damage` (per bullet/pellet), `semi_auto` (one shot per
   click). The shotgun fans 5 pellets evenly across its spread.

### Gameplay numbers used in the preview sim (starting values, tune freely)
- Crawler: 3 HP, chases at ~36 px/s, bites at ~15 px range every 0.7 s.
- Spitter: 6 HP, lobs acid every 2.3 s; the glob arcs ~36 px high over 0.7 s, then splashes.
- Drone: 7 HP, fires a laser every 2.6 s (bolt travels ~0.26 s).
- Hit feedback: white flash (`tint` ~0.8 toward white) for 60 ms on a hit; player hurt anim 160 ms.

## Conventions and gotchas

- Render to a low internal resolution (e.g. **480×270**) from `Layout`, scale up by an integer,
  nearest-neighbour only. Camera follows the player and snaps to whole pixels.
- **Move bullets in sub-steps** (we use 4 per frame) or test segment-vs-circle. Bullets travel
  ~20 px per frame, more than an enemy's width, and skipped straight past targets in the sim.
- Rotated pixel art gets slightly jagged at odd angles. That's acceptable; don't try to fix it
  with smoothing filters.
- Keep game data (weapon stats, timings) read from the JSON rather than hard-coded, so art
  regeneration and tuning stay in one place.
- Ebitengine on Linux needs a display even to run tests (`xvfb-run`), plus the X11/GL dev
  packages to build. macOS needs nothing extra beyond Xcode Command Line Tools.

## Prototype: what exists

- **Controls:** WASD/arrows move, mouse aims, LMB fires (rifle and shotgun automatic, pistol
  semi-auto), 1/2/3, Q or wheel switch weapons, Esc/P pause, R restart, F fullscreen.
- **Rendering:** 480×270 canvas, window ×3, nearest scaling. The level is pre-rendered once
  (`renderLevel`); props, enemies and the player are y-sorted by feet each frame; corpses
  draw underneath; projectiles and FX on top. Hit flash uses `colorm`.
- **Projectiles** keep a *ground* position plus a height `H` they're drawn at. Collisions use
  the ground position (walls, props, enemy radius), which keeps the 3/4 view simple.
  Bullets move in 4 sub-steps per tick.
- **Enemies** (tuning table `stats` in `internal/game/enemy.go`) sleep until they see the
  player within their aggro range, then wake neighbours within 110 px. Crawlers follow a BFS
  flow field around walls (rebuilt every 0.25 s) and go straight at the player when close;
  spitters lob acid that leads the player (a growing ground marker shows where it lands);
  drones keep 85–140 px away, strafe, fly over props and fire lasers. Crates block bullets and lasers in both directions (cover).
- **Explosive barrels** take 3 hits, damage everything within ~44 px and chain to nearby barrels.
- **Dev tools** (`internal/game/devtools.go`): `BZ_AUTOPILOT=1` lets a bot play
  (`BZ_AUTOPILOT=human` gives it shaky aim and 0.35 s reactions, a rough stand-in for a person);
  `BZ_RECORD=dir BZ_RECORD_SECONDS=30` saves every 3rd frame as PNG and then quits.
  `docs/previews/gameplay.gif` was made that way.
- **Tests:** world parsing/connectivity (every spawn reachable), autotile vs the Python
  golden data, collision, line of sight, flow-field following, metadata completeness,
  facing sectors, and full rounds played by the autopilot (clear, death, restart, barrel chains).
  `TestDifficulty` plays 20 seeded rounds with the handicapped bot and fails if it clears under
  half of them or keeps over 75% HP on average; run it with `-v` while tuning.
  Keep `TestStationIsConnected` passing when editing the map.

## Next steps

- **Difficulty: play-test it.** Tuned against the bots only (October 2026): player 8 HP and
  0.45 s invulnerability; crawlers 4 HP at 70 px/s; drones 7 HP; ranged cooldowns 2.0 s;
  shots lead the player more (`globLead`, `laserLead`, `laserSpeed` in `enemy.go`). The
  handicapped bot now clears in ~40 s with about 4.5/8 HP left and dies in a few % of rounds.
  Most damage comes late (no healing), so pickups are the next lever if it feels harsh.
- Real font for the HUD (currently `ebitenutil.DebugPrint`), ammo and reloading, pickups.
- Sound (Ebitengine `audio` package), muzzle and impact sounds first.
- Closed doors that open when approached (`door_half` frame exists), more levels, a level
  format loaded from files, waves or spawners.
- Possible later mode: turn-based tactics reusing the same tiles and sprites.
