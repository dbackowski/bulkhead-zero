# Bulkhead Zero

Top-down sci-fi shooter: a lone marine clears an infested space station.
WASD to move, mouse to aim (full 360°), click to shoot.
Go + **Ebitengine v2** (`github.com/hajimehoshi/ebiten/v2`), pixel art at 24×24 per tile/frame.

**Status:** playable prototype. Two station decks (Habitat, Reactor), each played as 4 waves
and a boss, the marine with 360° aiming and six weapons (three unlocked by clearing waves),
crawlers / spitters / drones / bombers / brutes, two bosses (the Brood Mother, the Warden),
magazines and reloading,
health and ammo pickups, sliding doors, explosive barrels, HUD, sounds, music, win/lose and restart. See "Prototype: what exists" and "Next steps" at the end.

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
                  bosses.png (48×48 frames), weapons.png (24×12 per weapon), font.png (HUD
                  font), sprites.json (all metadata)
assets/sounds/    <name>_<variant>.wav, 16-bit mono 44.1 kHz (the game picks a random variant)
assets/music/     music_<name>.wav loops, 16-bit mono 22.05 kHz
tools/artgen/     make_tiles.py, make_sprites.py, make_sfx.py, make_music.py, make_preview.py, build.py
docs/previews/    enlarged sheets + scene.gif / animations.gif / aiming.png for viewing only
```

Go code (module `github.com/dbackowski/bulkhead-zero`, Ebitengine **v2.10**, Go 1.25+):

```
main.go                 window setup, runs game.New()
assets/assets.go        //go:embed of tiles/, sprites/, sounds/ and music/
internal/geom           Vec (2D math)
internal/gfx            sprites.json/tiles.json parsing, Library (anims, tiles, weapons, font),
                        draw helpers, Font (bitmap text)
internal/world          map text format, autotile, doors, collision (MoveAndSlide), line of sight,
                        BFS flow field for enemy pathing; maps.go holds the Station and Reactor maps
internal/game           Game loop, input, player, enemies (AI + stats table), combat
                        (bullets, enemy shots, blasts), waves.go (waves, boss, doors),
                        drawing (y-sorted), HUD, sound (sfx.go), devtools
```

## Art pipeline: never hand-edit the PNGs or WAVs

All art and sound comes from code. Change the scripts, then regenerate:

```
pip3 install -r tools/artgen/requirements.txt   # Pillow
python3 tools/artgen/build.py                   # sheets + JSON + sounds + music + previews (~50 s)
python3 tools/artgen/build.py --no-previews     # just the game assets
```

`build.py` is deterministic (seeded RNG), so an unchanged script reproduces identical files.
Check new art by reading the PNGs in `docs/previews/` (look at them, don't just trust the run).
`make_sfx.py` needs only the standard library; listen to the WAVs, or at least plot them.
`make_music.py` (standard library, reuses make_sfx's synth helpers) composes the loops in code:
each track is a `Track` with drum, bass, pad and lead parts, mixed by target loudness per
part (`mixdown` levels) then gently limited; the render's tail is folded onto its start so
it loops without a seam. It renders at 44.1 kHz and writes 22.05 kHz to keep files small
(~1.2 MB each). It takes most of the build time (~30 s). When changing it, plot the result:
a solid wall of waveform means the mix is clipping.
`build.py` deletes old WAVs only after everything generated, so a failed run leaves the
assets alone.
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
  idle/walk, `size` when frames aren't 24). Frame `n` is at `(n*size, row*size)`.
- Every 24×24 frame has its **feet / ground contact at about (12, 21)**: anchor entities there and
  y-sort by feet. Bigger frames follow the same rule, `(size/2, size-3)` (the boss: (24, 45)).
- Only `*_right` facings are drawn; **left = the right frames flipped horizontally**
  (player diagonals, crawler, bomber and brute too). Spitter, drone and boss face the camera.
- `loop: false` animations play once, then hold the last frame (death) or return to idle.
- Enemies: `crawler.walk_right/attack_right/death` (fast melee), `spitter.idle/attack/death`
  (stationary, lobs acid; the glob should leave on attack frame 2), `drone.idle/attack/death`
  (hovering, shoots a laser; it fires on attack frame 2), `bomber.walk_right/attack_right/death`
  (attack = swelling before it bursts), `brute.walk_right/attack_right/death` (attack frames
  0-1 wind-up, 2-3 charge), `brood.idle/attack/death` and `warden.idle/attack/death` (48×48, in bosses.png).
- FX: `bullet`, `laser`, `plasma`, `grenade`, `spine` (all point right: rotate to the heading),
  `hit_spark`, `acid_glob`, `acid_splash`, `explosion`, `muzzle_flash`, `crosshair`.
- Font (`font.png`, metadata under `"font.png"`): 5×7 uppercase glyphs (`GLYPHS` in
  make_sprites.py), white with a baked 1px dark outline, one 7×9 cell each in `chars` order,
  6 px advance. `Font.Draw` tints them and scales by whole numbers (titles are 2×); lower case
  is drawn as upper case. A new character is one `GLYPHS` entry.

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
   click), `mag` (rounds per magazine; a shotgun blast is one round), `reload_ms`, `ammo`
   (most spare rounds carried; 0 = unlimited, the pistol), and optionally `projectile` (fx anim, default `bullet`), `pierce` (enemies a shot
   passes through: plasma) and `blast` (explosion radius instead of a hit: launcher, which
   also bursts at the end of its range and can hurt the player). The shotgun fans 5 pellets
   evenly across its spread.

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

- **Controls:** WASD/arrows move, mouse aims, LMB fires (pistol and launcher semi-auto, the
  rest automatic), R reloads, 1-6, Q or wheel switch between unlocked weapons, Esc/P pause,
  F fullscreen, M mute. Restart is R in the pause menu, or R / click after dying or winning.
- **Ammo** (`player.go`): each weapon slot has its own loaded and spare rounds, starting full.
  An empty gun reloads by itself (also when you switch back to it); switching weapons
  drops a reload in progress. Out of spare rounds the trigger only clicks (`dry_fire`); the
  pistol never runs out.
- **Pickups** (`pickups.go`): dying enemies may drop one (`dropChance`: crawler 12%,
  spitter/drone 40%, brute always; bombers and the boss's brood never), 40% health (+2 HP)
  and 60% ammo (half a full load of spare rounds for every unlocked gun). Walk over to
  collect; one you can't use stays on the floor. They vanish after 25 s, blinking for the
  last 4.
- **Decks** (`levels` in `internal/game/waves.go`): each has a name, a map, its wave list,
  its boss (map letter `M` Brood Mother, `W` Warden) and how many weapons it starts with
  (3, then 6). Beating a deck's boss starts the next deck after 2.5 s with full HP and ammo;
  dying (or R in the pause menu) retries the current deck from its start; after the last
  deck the station is clear and R / click starts again from deck 1.
- **Waves**: wave 1 is the enemies placed in the map; then each string in the deck's
  `waves` (map letters, one per enemy) trickles in through the outer doors away from the
  player; then the boss appears at the map's `Q`. Between waves: a 4 s break, +3 HP (full
  HP before the boss) and the next weapon unlocks (smg, plasma, launcher). Killing a boss
  kills its minions.
- **Doors** (`D` in the map, `=` starts open; need floor below): inner doors (floor above too)
  slide open (`door_closed/half/open`, 0.25 s) while the player or an enemy is within 40 px;
  outer doors (wall above) open only for enemies and are where waves come in. A closed door
  blocks movement, bullets and sight like a wall; path finding treats every door as open.
- **HUD** (`hud.go`): all text uses the generated pixel font. Wave and hostiles top-left,
  boss name and health bar top-centre, health pips, weapon and ammo (loaded/spare, RELOAD
  with a progress line, or EMPTY) bottom-left; banners and
  overlays (`printCentered`) draw their first line at 2× in a title colour.
- **Rendering:** 480×270 canvas, window ×3, nearest scaling. The level is pre-rendered once
  (`renderLevel`); props, enemies and the player are y-sorted by feet each frame; corpses
  draw underneath; projectiles and FX on top. Hit flash uses `colorm`.
- **Sound** (`internal/game/sfx.go`): `g.sound(name, pos)` plays a random variant, quieter
  with distance from the player (floor 0.15 at 340+ px); repeats of one sound within 40 ms are
  dropped. Wired so far: weapon shots (named like the weapon) and bullet hits: `hit_flesh`
  (crawler, spitter), `hit_metal` (drone, explosive barrel), `hit_wall` (walls, other props),
  `hurt` / `player_death`, `<kind>_death` per enemy (`deathSound` in `enemy.go`), `explosion`
  (barrels, bombers, grenades), `door`, `boss_roar`, `pickup`, `reload`, `dry_fire`, and
  enemy attacks: `laser` (drone fires),
  `spit` and `acid_splash` (spitter glob), `spine` (boss ring).
  `hurt` and `player_death` are a voice made by source-filter synthesis (`voice()` in make_sfx.py). `BZ_MUTE=1`
  or running under `go test` skips audio entirely (no audio context is created).
- **Music** (`sfx.go`, `musicTrack`/`updateMusic`): three loops, `calm` (exploring, between
  fights, death and win screens), `combat` (while a wave is coming in or any enemy is awake,
  lingering 5 s after, longer than a wave break so it carries through) and `boss` (while a
  boss lives). Switching crossfades over 1.5 s and a loop that fades in starts from its
  beginning. Music sits at 0.45 volume under the effects, drops to 40% while paused, and
  M mutes it with everything else.
- **Projectiles** keep a *ground* position plus a height `H` they're drawn at. Collisions use
  the ground position (walls, props, enemy radius), which keeps the 3/4 view simple.
  Bullets move in 4 sub-steps per tick.
- **Enemies** (tuning table `stats` in `internal/game/enemy.go`) sleep until they see the
  player within their aggro range, then wake neighbours within 110 px. Crawlers follow a BFS
  flow field around walls (rebuilt every 0.25 s) and go straight at the player when close;
  spitters lob acid that leads the player (a growing ground marker shows where it lands);
  drones keep 85–140 px away, strafe, fly over props and fire lasers; bombers run in, swell
  for 0.5 s and burst (shooting one sets it off, hurting enemies nearby too); brutes (20 HP)
  wind up and charge in a straight line, stopping at walls. Bosses (`updateBoss`) hurt on
  contact and get faster with bigger attacks below half HP. The Brood Mother (120 HP)
  crawls after the player and cycles a ring of slow spines, an acid barrage and summoning
  crawlers. The Warden (80 HP, 2.4 s between attacks) hovers 60–100 px away and cycles a
  slow fan of 3 lasers, mortar shells (an orange ground marker shows the blast; they hurt
  enemies too) and deploying drones. Crates block bullets, lasers and spines in both
  directions (cover).
- **Explosive barrels** take 3 hits, damage everything within ~44 px and chain to nearby barrels
  (`blast` in combat.go, shared with bombers and grenades).
- **Dev tools** (`internal/game/devtools.go`): `BZ_AUTOPILOT=1` lets a bot play
  (`BZ_AUTOPILOT=human` gives it shaky aim and 0.35 s reactions, a rough stand-in for a person);
  `BZ_RECORD=dir BZ_RECORD_SECONDS=30` saves every 3rd frame as PNG and then quits.
  `docs/previews/gameplay.gif` was made that way.
- **Tests:** world parsing/connectivity (every spawn reachable), autotile vs the Python
  golden data, collision, line of sight, flow-field following, metadata completeness,
  facing sectors, doors, the wave → boss → clear flow, reloading, pickups, music choice, and full rounds
  played by the autopilot
  (clear, death, restart, barrel chains), and walking through every wave of both decks.
  `TestDifficulty` plays 20 seeded games of each deck (all waves and the boss) with the
  handicapped bot and fails if it beats the boss in under half of them or keeps over 75% HP
  on average; run it with `-v` while tuning. `TestMapsAreConnected` checks every map:
  everything reachable, a boss spot, outer doors that open onto floor.

## Next steps

- **Difficulty: play-test it.** Tuned against the bots only: player 8 HP and 0.45 s
  invulnerability; crawlers 4 HP at 70 px/s; drones 7 HP; ranged cooldowns 2.0 s; shots lead
  the player (`globLead`, `laserLead`, `laserSpeed` in `enemy.go`). With waves and the boss
  and then reloading and pickups (November 2026) the handicapped bot beats deck 1 in 11/20
  games (~4.6/8 HP left) and deck 2 in 10/20 (~4.0/8), in ~150 s each; most deaths are to
  the bosses. Reload time was the biggest lever (1.2 s rifle reloads dropped deck 1 to
  2/20); for the Warden it was its attack rate (2.0 s → 3/20, 2.4 s → 10/20). Explosive
  barrels on the bot's routes and enemies covering an entrance also cost a lot: keep them
  off the main paths when editing maps. The bot uses rifle/shotgun/pistol, the smg once
  unlocked and plasma against the Warden, finishes reloads before switching, and picks up
  drops when nothing is in sight. Levers: each deck's `waves`, boss `hp` and `cooldown`,
  `waveHeal`, `reload_ms`, `dropChance`.
- Destructible crates that drop pickups; ammo is plentiful for the bot, so `dropChance`
  and `ammo` could tighten if it never feels scarce.
- More sounds are one `SOUNDS` entry in `make_sfx.py` plus one `g.sound` call. Music per
  deck (e.g. the Reactor transposed or with its own lead) would be new `TRACKS` entries
  plus a deck field choosing them; stingers for wave start, death and victory.
- More decks: a map in `maps.go` (keep `TestMapsAreConnected` passing) plus an entry in
  `levels`; a new boss is a kind, a stats entry, an attack function and a bosses.png row.
  The Reactor map was laid out by a small script (rooms, openings and props as
  coordinates) rather than typed by hand, which avoids off-by-one columns.
- `docs/previews/gameplay.gif` predates waves and doors; re-record it.
- Possible later mode: turn-based tactics reusing the same tiles and sprites.
