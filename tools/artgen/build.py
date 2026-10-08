#!/usr/bin/env python3
"""Regenerate every art and sound asset, and the previews, for Bulkhead Zero.

Usage (from anywhere):
    python3 tools/artgen/build.py            # sheets + JSON + previews
    python3 tools/artgen/build.py --no-previews

The three generator scripts write into the current directory, so this runs
them in a scratch folder and moves each output to its place in the repo:
    assets/tiles/     tileset.png, tiles.json
    assets/sprites/   player.png, weapons.png, enemies.png, fx.png, sprites.json
    assets/sounds/    <name>_<variant>.wav (make_sfx.py, standard library only)
    docs/previews/    *_x3/x4 sheets, aiming.png, scene.gif, animations.gif
"""
import os
import shutil
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
ROOT = HERE.parents[1]

DEST = {
    "tileset.png": "assets/tiles",
    "tiles.json": "assets/tiles",
    "player.png": "assets/sprites",
    "weapons.png": "assets/sprites",
    "enemies.png": "assets/sprites",
    "fx.png": "assets/sprites",
    "sprites.json": "assets/sprites",
    "tileset_x4.png": "docs/previews",
    "preview_x3.png": "docs/previews",
    "sprites_x4.png": "docs/previews",
    "aiming.png": "docs/previews",
    "scene.gif": "docs/previews",
    "animations.gif": "docs/previews",
}


def main():
    previews = "--no-previews" not in sys.argv
    sys.path.insert(0, str(HERE))
    for old in (ROOT / "assets/sounds").glob("*.wav"):  # renamed sounds mustn't linger
        old.unlink()
    with tempfile.TemporaryDirectory() as tmp:
        os.chdir(tmp)
        import make_sfx
        import make_tiles
        import make_sprites
        make_sfx.main()
        make_tiles.main()
        make_sprites.main()
        if previews:
            import make_preview
            make_preview.aiming_sheet()
            make_preview.scene()
            make_preview.showcase()
        for name in sorted(os.listdir(tmp)):
            dest = DEST.get(name) or ("assets/sounds" if name.endswith(".wav") else None)
            if dest is None:
                continue
            target = ROOT / dest
            target.mkdir(parents=True, exist_ok=True)
            shutil.move(os.path.join(tmp, name), target / name)
            print(f"  {dest}/{name}")


if __name__ == "__main__":
    main()
