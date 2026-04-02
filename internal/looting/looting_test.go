package looting

import (
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func makeWorld(entries []world.LootEntry) *world.World {
	w := &world.World{
		LootTables: []world.LootTable{
			{ID: "t1", Entries: entries},
		},
		Rooms: []world.Room{
			{
				ID: "r0",
				NPCs: []world.NPC{
					{ID: "npc-0", Name: "Enemy", LootTableID: "t1"},
				},
			},
		},
	}
	return w
}

func TestNoProbabilityNeverDrops(t *testing.T) {
	w := makeWorld([]world.LootEntry{
		{ItemID: "item-a", Probability: 0.0, CountMin: 1, CountMax: 1},
	})
	for i := 0; i < 100; i++ {
		items := Roll(w, "npc-0", "")
		if len(items) != 0 {
			t.Fatalf("probability 0.0 item dropped on iteration %d", i)
		}
	}
}

func TestAlwaysDrops(t *testing.T) {
	w := makeWorld([]world.LootEntry{
		{ItemID: "credits", Name: "Credits", Probability: 1.0, CountMin: 5, CountMax: 5},
	})
	for i := 0; i < 10; i++ {
		items := Roll(w, "npc-0", "")
		if len(items) != 5 {
			t.Fatalf("probability 1.0 item did not always drop: got %d items", len(items))
		}
	}
}

func TestCountRange(t *testing.T) {
	w := makeWorld([]world.LootEntry{
		{ItemID: "coin", Name: "Coin", Probability: 1.0, CountMin: 2, CountMax: 4},
	})
	for i := 0; i < 50; i++ {
		items := Roll(w, "npc-0", "")
		if len(items) < 2 || len(items) > 4 {
			t.Errorf("count out of range [2,4]: got %d", len(items))
		}
	}
}

func TestNPCWithNoLootTable(t *testing.T) {
	w := &world.World{
		Rooms: []world.Room{
			{
				ID: "r0",
				NPCs: []world.NPC{
					{ID: "npc-1", Name: "Thug"},
				},
			},
		},
	}
	items := Roll(w, "npc-1", "")
	if len(items) != 0 {
		t.Errorf("expected no loot for NPC with no loot table, got %d items", len(items))
	}
}

func TestUnknownNPC(t *testing.T) {
	w := makeWorld([]world.LootEntry{
		{ItemID: "item", Probability: 1.0, CountMin: 1, CountMax: 1},
	})
	items := Roll(w, "unknown-npc", "")
	if len(items) != 0 {
		t.Errorf("expected no loot for unknown NPC, got %d items", len(items))
	}
}
