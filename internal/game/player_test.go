package game

import (
	"math"
	"testing"
)

func TestFacingFor(t *testing.T) {
	f := []string{"right", "down_right", "down", "down_left", "left", "up_left", "up", "up_right"}
	cases := map[float64]string{
		0:                "right",
		math.Pi / 4:      "down_right",
		math.Pi / 2:      "down",
		math.Pi:          "left",
		-math.Pi:         "left",
		-math.Pi / 2:     "up",
		-math.Pi / 4:     "up_right",
		-3 * math.Pi / 4: "up_left",
		0.38:             "right",      // 21.8°, still inside the right sector
		0.42:             "down_right", // 24°, just past the boundary
	}
	for a, want := range cases {
		if got := facingFor(f, a); got != want {
			t.Errorf("facingFor(%.2f) = %s, want %s", a, got, want)
		}
	}
}
