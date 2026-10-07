package game

import "testing"

// Plays the level with the autopilot, without a window, and checks the round
// flows: enemies die, the deck can be cleared or lost, and restart works.
// On Linux this needs a display (xvfb-run go test ./...); macOS is fine as is.
func TestAutopilotRound(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	g.autopilot = newAutopilot()
	start := g.aliveFoes()
	for i := 0; i < 60*120 && g.state == statePlaying; i++ { // up to two minutes of game time
		if err := g.Update(); err != nil {
			t.Fatal(err)
		}
	}
	killed := start - g.aliveFoes()
	t.Logf("after %.1fs: state=%d killed=%d/%d hp=%d", g.t, g.state, killed, start, g.player.HP)
	if killed < start/2 {
		t.Errorf("autopilot only killed %d of %d enemies", killed, start)
	}
	if g.state == statePlaying {
		t.Error("round never ended")
	}
}

func TestDeathAndRestart(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for g.player.HP > 0 {
		g.player.Invuln = 0
		g.hurtPlayer(1, g.player.Pos)
	}
	for i := 0; i < 120 && g.state == statePlaying; i++ {
		if err := g.Update(); err != nil {
			t.Fatal(err)
		}
	}
	if g.state != stateDead {
		t.Fatalf("expected the dead state, got %d", g.state)
	}
	if err := g.reset(); err != nil {
		t.Fatal(err)
	}
	if g.player.HP != playerMaxHP || g.aliveFoes() != g.totalFoes || g.state != statePlaying {
		t.Fatal("restart did not restore the round")
	}
}

func TestBarrelChainReaction(t *testing.T) {
	g, err := New()
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range g.lvl.Props {
		if p.Explosive {
			g.explode(p)
			break
		}
	}
	for i := 0; i < 30; i++ {
		g.updateBlasts()
	}
	n := 0
	for _, p := range g.lvl.Props {
		if p.Explosive && !p.Alive {
			n++
		}
	}
	if n < 1 || len(g.scorches) != n {
		t.Fatalf("exploded %d barrels, %d scorch marks", n, len(g.scorches))
	}
}
