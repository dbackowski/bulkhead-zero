// Package gfx loads the generated sprite sheets and draws frames.
package gfx

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"strconv"
)

// AnimMeta is one row of a sprite sheet, as written by tools/artgen.
type AnimMeta struct {
	Row     int   `json:"row"`
	Frames  int   `json:"frames"`
	FrameMS int   `json:"frame_ms"`
	Loop    bool  `json:"loop"`
	Bob     []int `json:"bob"`
}

// WeaponMeta describes a weapon sprite (drawn pointing right) and its stats.
type WeaponMeta struct {
	Rect        [4]int  `json:"rect"` // x, y, w, h in weapons.png
	Pivot       [2]int  `json:"pivot"`
	Muzzle      [2]int  `json:"muzzle"`
	FireMS      int     `json:"fire_ms"`
	SpreadDeg   float64 `json:"spread_deg"`
	Pellets     int     `json:"pellets"`
	RecoilPx    float64 `json:"recoil_px"`
	BulletSpeed float64 `json:"bullet_speed"`
	Damage      int     `json:"damage"`
	SemiAuto    bool    `json:"semi_auto"` // fires once per click
}

// PlayerMeta holds the detached-gun aiming data.
type PlayerMeta struct {
	Facings       []string          `json:"facings"`
	HandAnchor    map[string][2]int `json:"hand_anchor"`
	GunBehindBody []string          `json:"gun_behind_body"`
}

// SpriteMeta mirrors assets/sprites/sprites.json.
type SpriteMeta struct {
	FrameSize int                            `json:"frame_size"`
	Player    PlayerMeta                     `json:"player"`
	Sheets    map[string]map[string]AnimMeta `json:"sheets"`
	Weapons   map[string]WeaponMeta          `json:"weapons.png"`
}

// TileMeta mirrors assets/tiles/tiles.json.
type TileMeta struct {
	TileSize int               `json:"tile_size"`
	Columns  int               `json:"columns"`
	Tiles    map[string]string `json:"tiles"` // index (as string) -> name
	Autotile struct {
		WallCapBaseIndex int            `json:"wall_cap_base_index"`
		WallInnerIndex   map[string]int `json:"wall_inner_index"`
	} `json:"autotile"`
}

// TileIndex returns name -> index.
func (m *TileMeta) TileIndex() (map[string]int, error) {
	out := make(map[string]int, len(m.Tiles))
	for k, name := range m.Tiles {
		i, err := strconv.Atoi(k)
		if err != nil {
			return nil, fmt.Errorf("tiles.json: bad index %q", k)
		}
		out[name] = i
	}
	return out, nil
}

func readJSON(fsys fs.FS, path string, v any) error {
	b, err := fs.ReadFile(fsys, path)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(b, v); err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}
	return nil
}

// LoadSpriteMeta reads sprites/sprites.json.
func LoadSpriteMeta(fsys fs.FS) (*SpriteMeta, error) {
	var m SpriteMeta
	if err := readJSON(fsys, "sprites/sprites.json", &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// LoadTileMeta reads tiles/tiles.json.
func LoadTileMeta(fsys fs.FS) (*TileMeta, error) {
	var m TileMeta
	if err := readJSON(fsys, "tiles/tiles.json", &m); err != nil {
		return nil, err
	}
	return &m, nil
}
