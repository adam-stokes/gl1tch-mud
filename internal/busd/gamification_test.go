package busd_test

import (
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/busd"
)

func TestMapMudEvent_CombatWon(t *testing.T) {
	action, ok := busd.MapMudEvent("mud.combat.ended", map[string]any{"outcome": "won"})
	if !ok {
		t.Fatal("expected ok")
	}
	if action != "combat.won" {
		t.Errorf("got %q, want %q", action, "combat.won")
	}
}

func TestMapMudEvent_CombatLost(t *testing.T) {
	action, ok := busd.MapMudEvent("mud.combat.ended", map[string]any{"outcome": "lost"})
	if !ok {
		t.Fatal("expected ok")
	}
	if action != "combat.lost" {
		t.Errorf("got %q, want %q", action, "combat.lost")
	}
}

func TestMapMudEvent_RoomEnteredFirst(t *testing.T) {
	action, ok := busd.MapMudEvent("mud.room.entered", map[string]any{"first": true})
	if !ok {
		t.Fatal("expected ok=true for first room")
	}
	if action != "room.explored" {
		t.Errorf("got %q, want %q", action, "room.explored")
	}
}

func TestMapMudEvent_RoomEnteredNotFirst(t *testing.T) {
	_, ok := busd.MapMudEvent("mud.room.entered", map[string]any{"first": false})
	if ok {
		t.Error("expected ok=false for revisit")
	}
}

func TestMapMudEvent_Unknown(t *testing.T) {
	_, ok := busd.MapMudEvent("mud.session.started", nil)
	if ok {
		t.Error("expected ok=false for unmapped topic")
	}
}

func TestMapMudEvent_SimpleTopics(t *testing.T) {
	cases := []struct {
		topic  string
		action string
	}{
		{"mud.hack.success", "hack.success"},
		{"mud.trade.completed", "trade.completed"},
		{"mud.craft.completed", "craft.completed"},
		{"mud.lock.picked", "lock.picked"},
		{"mud.player.died", "player.died"},
	}
	for _, tc := range cases {
		got, ok := busd.MapMudEvent(tc.topic, nil)
		if !ok {
			t.Errorf("topic %q: expected ok=true", tc.topic)
			continue
		}
		if got != tc.action {
			t.Errorf("topic %q: got %q, want %q", tc.topic, got, tc.action)
		}
	}
}
