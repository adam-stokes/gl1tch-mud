package world

import (
	"testing"

	"gopkg.in/yaml.v3"
)

// minimalYAML is the existing cyberspace world — no new fields.
const minimalYAML = `
name: cyberspace
start_room: net-0
narrator_model: llama3.2
rooms:
  - id: net-0
    name: "The Gibson"
    desc: "Entry node."
    exits:
      north: net-1
    npcs:
      - id: daemon-0
        name: "Daemon"
        hp: 30
        attack: 8
        desc: "Scary."
    items: []
`

// extendedYAML includes all new fields.
const extendedYAML = `
name: testworld
start_room: r0
narrator_model: llama3.2
crafting_recipes:
  - id: sniffer
    name: "Packet Sniffer"
    ingredients:
      - {id: silicon, count: 2}
      - {id: wire, count: 1}
    output: {id: sniffer, name: "Sniffer", desc: "Listens."}
    skill_req: 2
loot_tables:
  - id: table-a
    entries:
      - {item_id: credits, probability: 1.0, count_min: 10, count_max: 50}
      - {item_id: data-chip, probability: 0.5, count_min: 1, count_max: 2}
rooms:
  - id: r0
    name: "Room Zero"
    desc: "A test room."
    exits:
      north: r1
    systems:
      - {id: ice-wall, security_level: 4, reward_item: root-key, reward_text: "Access granted."}
    locks:
      - {id: vault-door, exit: north, difficulty: 6, keys: [root-key]}
    npcs:
      - id: npc-0
        name: "Runner"
        hp: 20
        attack: 5
        desc: "A netrunner."
        loot_table_id: table-a
        trades:
          - id: trade-1
            wants: [{id: data-chip, count: 1}]
            offers: [{id: enc-key, name: "Enc Key", desc: "Encrypts.", count: 1}]
            faction_req: "netrunners:5"
        dialogue:
          - {trigger: "always", text: "System's hot."}
          - {trigger: "has_item:root-key", text: "Where'd you get that?"}
    items:
      - {id: hazmat-suit, name: "Hazmat Suit", desc: "Protective.", is_disguise: true}
  - id: r1
    name: "Room One"
    desc: "Another room."
    exits:
      south: r0
    npcs: []
    items: []
`

func TestParseMinimal(t *testing.T) {
	var w World
	if err := yaml.Unmarshal([]byte(minimalYAML), &w); err != nil {
		t.Fatalf("parse minimal world: %v", err)
	}
	if w.Name != "cyberspace" {
		t.Errorf("name: got %q want %q", w.Name, "cyberspace")
	}
	if len(w.Rooms) != 1 {
		t.Errorf("rooms count: got %d want 1", len(w.Rooms))
	}
	if len(w.CraftingRecipes) != 0 {
		t.Errorf("crafting_recipes should be nil for minimal world")
	}
	if len(w.LootTables) != 0 {
		t.Errorf("loot_tables should be nil for minimal world")
	}
}

func TestParseExtended(t *testing.T) {
	var w World
	if err := yaml.Unmarshal([]byte(extendedYAML), &w); err != nil {
		t.Fatalf("parse extended world: %v", err)
	}

	// Crafting recipes
	if len(w.CraftingRecipes) != 1 {
		t.Fatalf("crafting_recipes: got %d want 1", len(w.CraftingRecipes))
	}
	r := w.CraftingRecipes[0]
	if r.ID != "sniffer" || r.SkillReq != 2 || len(r.Ingredients) != 2 {
		t.Errorf("recipe mismatch: %+v", r)
	}

	// Loot tables
	if len(w.LootTables) != 1 {
		t.Fatalf("loot_tables: got %d want 1", len(w.LootTables))
	}
	lt := w.LootTables[0]
	if lt.ID != "table-a" || len(lt.Entries) != 2 {
		t.Errorf("loot table mismatch: %+v", lt)
	}

	// Room systems & locks
	if len(w.Rooms) < 1 {
		t.Fatal("no rooms")
	}
	room := w.Rooms[0]
	if len(room.Systems) != 1 {
		t.Errorf("systems: got %d want 1", len(room.Systems))
	}
	if room.Systems[0].SecurityLevel != 4 {
		t.Errorf("security_level: got %d want 4", room.Systems[0].SecurityLevel)
	}
	if len(room.Locks) != 1 {
		t.Errorf("locks: got %d want 1", len(room.Locks))
	}
	if room.Locks[0].Difficulty != 6 {
		t.Errorf("difficulty: got %d want 6", room.Locks[0].Difficulty)
	}

	// NPC extensions
	npc := room.NPCs[0]
	if npc.LootTableID != "table-a" {
		t.Errorf("loot_table_id: got %q want %q", npc.LootTableID, "table-a")
	}
	if len(npc.Trades) != 1 {
		t.Errorf("trades: got %d want 1", len(npc.Trades))
	}
	if len(npc.Dialogue) != 2 {
		t.Errorf("dialogue: got %d want 2", len(npc.Dialogue))
	}

	// Item is_disguise
	if !room.Items[0].IsDisguise {
		t.Errorf("is_disguise: expected true for hazmat-suit")
	}
}

func TestFindLootTable(t *testing.T) {
	var w World
	yaml.Unmarshal([]byte(extendedYAML), &w) //nolint:errcheck
	w.index = make(map[string]*Room)

	lt := w.FindLootTable("table-a")
	if lt == nil {
		t.Fatal("FindLootTable returned nil for known table")
	}
	if w.FindLootTable("nonexistent") != nil {
		t.Error("FindLootTable should return nil for unknown table")
	}
}

func TestFindRecipe(t *testing.T) {
	var w World
	yaml.Unmarshal([]byte(extendedYAML), &w) //nolint:errcheck
	w.index = make(map[string]*Room)

	rc := w.FindRecipe("sniffer")
	if rc == nil {
		t.Fatal("FindRecipe returned nil for known recipe")
	}
	if w.FindRecipe("nonexistent") != nil {
		t.Error("FindRecipe should return nil for unknown recipe")
	}
}

func TestDefaultWorldLoads(t *testing.T) {
	w, err := Load("cyberspace")
	if err != nil {
		t.Fatalf("Load cyberspace: %v", err)
	}
	if w.StartRoom == "" {
		t.Error("start_room should not be empty")
	}
	if w.Room(w.StartRoom) == nil {
		t.Errorf("start room %q not found", w.StartRoom)
	}
}
