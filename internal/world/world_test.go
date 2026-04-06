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

const worldWithUI = `
name: testworld
start_room: r0
narrator_model: test
ui:
  banner: "TEST BANNER"
  prompt: "#"
  tagline: "test tagline"
  theme:
    bg: "#000000"
    fg: "#ffffff"
    accent: "#ff0000"
    dim: "#333333"
    border: "#444444"
    error: "#ff5555"
    success: "#00ff00"
rooms:
  - id: r0
    name: "Start"
    desc: "Beginning."
    exits: {}
`

const worldNoUI = `
name: noui
start_room: r0
narrator_model: test
rooms:
  - id: r0
    name: "Start"
    desc: "."
    exits: {}
`

func TestWorldUIFullParse(t *testing.T) {
	var w World
	if err := yaml.Unmarshal([]byte(worldWithUI), &w); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if w.UI.Banner != "TEST BANNER" {
		t.Errorf("banner: got %q want %q", w.UI.Banner, "TEST BANNER")
	}
	if w.UI.Prompt != "#" {
		t.Errorf("prompt: got %q want %q", w.UI.Prompt, "#")
	}
	if w.UI.Tagline != "test tagline" {
		t.Errorf("tagline: got %q want %q", w.UI.Tagline, "test tagline")
	}
	if w.UI.Theme.BG != "#000000" {
		t.Errorf("theme.bg: got %q want %q", w.UI.Theme.BG, "#000000")
	}
	if w.UI.Theme.Accent != "#ff0000" {
		t.Errorf("theme.accent: got %q want %q", w.UI.Theme.Accent, "#ff0000")
	}
}

func TestWorldUIFallbacks(t *testing.T) {
	var w World
	if err := yaml.Unmarshal([]byte(worldNoUI), &w); err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := w.UIPrompt(); got != ">" {
		t.Errorf("UIPrompt: got %q want %q", got, ">")
	}
	if got := w.UIBanner(); got != "" {
		t.Errorf("UIBanner: got %q want empty", got)
	}
}

func TestListAvailable(t *testing.T) {
	metas := ListAvailable()
	if len(metas) == 0 {
		t.Fatal("ListAvailable returned empty slice")
	}
	found := false
	for _, m := range metas {
		if m.Name == "cyberspace" {
			found = true
			if m.Tagline == "" {
				t.Error("cyberspace should have a non-empty tagline after adding ui: block")
			}
			if m.Theme.Accent == "" {
				t.Error("cyberspace should have a non-empty theme accent")
			}
		}
	}
	if !found {
		t.Error("ListAvailable should always include cyberspace")
	}
}

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

func TestRoomBiomeAndResources(t *testing.T) {
	raw := `
name: test
start_room: r1
narrator_model: test
rooms:
  - id: r1
    name: Test Room
    desc: A test room.
    exits: {}
    biome: forest
    resources:
      - id: oak-tree
        type: harvest
        yields:
          - item_id: wood-log
            probability: 1.0
            count_min: 1
            count_max: 3
        tool_required: ""
        respawn_actions: 10
        grow_actions: 0
weather_table:
  - biome: forest
    possible: [clear, rainy]
`
	var w World
	if err := yaml.Unmarshal([]byte(raw), &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	r := w.Rooms[0]
	if r.Biome != "forest" {
		t.Errorf("biome: want forest, got %q", r.Biome)
	}
	if len(r.Resources) != 1 || r.Resources[0].ID != "oak-tree" {
		t.Errorf("resources: got %+v", r.Resources)
	}
	if len(w.WeatherTable) != 1 || w.WeatherTable[0].Biome != "forest" {
		t.Errorf("weather_table: got %+v", w.WeatherTable)
	}
}

func TestAvailable(t *testing.T) {
	names := Available()
	found := false
	for _, n := range names {
		if n == "cyberspace" {
			found = true
		}
	}
	if !found {
		t.Error("Available() should always include cyberspace")
	}
}

// buildTestIndex wires w.index so Room() lookups work in tests that bypass Load().
func buildTestIndex(w *World) {
	w.index = make(map[string]*Room, len(w.Rooms))
	for i := range w.Rooms {
		w.index[w.Rooms[i].ID] = &w.Rooms[i]
	}
}

func TestComputeGridLayoutLinear(t *testing.T) {
	w := &World{
		StartRoom: "a",
		Rooms: []Room{
			{ID: "a", Exits: map[string]string{"east": "b"}},
			{ID: "b", Exits: map[string]string{"west": "a", "east": "c"}},
			{ID: "c", Exits: map[string]string{"west": "b"}},
		},
	}
	buildTestIndex(w)
	w.computeGridLayout()

	want := map[string][2]int{"a": {0, 0}, "b": {1, 0}, "c": {2, 0}}
	for _, r := range w.Rooms {
		if exp, ok := want[r.ID]; ok {
			if r.GridX != exp[0] || r.GridY != exp[1] {
				t.Errorf("room %s: got (%d,%d) want (%d,%d)", r.ID, r.GridX, r.GridY, exp[0], exp[1])
			}
		}
	}
}

func TestComputeGridLayoutCardinals(t *testing.T) {
	w := &World{
		StartRoom: "center",
		Rooms: []Room{
			{ID: "center",   Exits: map[string]string{"north": "north-rm", "east": "east-rm", "south": "south-rm", "west": "west-rm"}},
			{ID: "north-rm", Exits: map[string]string{"south": "center"}},
			{ID: "east-rm",  Exits: map[string]string{"west":  "center"}},
			{ID: "south-rm", Exits: map[string]string{"north": "center"}},
			{ID: "west-rm",  Exits: map[string]string{"east":  "center"}},
		},
	}
	buildTestIndex(w)
	w.computeGridLayout()

	want := map[string][2]int{
		"center":   {0, 0},
		"north-rm": {0, -1},
		"east-rm":  {1, 0},
		"south-rm": {0, 1},
		"west-rm":  {-1, 0},
	}
	for _, r := range w.Rooms {
		if exp, ok := want[r.ID]; ok {
			if r.GridX != exp[0] || r.GridY != exp[1] {
				t.Errorf("room %s: got (%d,%d) want (%d,%d)", r.ID, r.GridX, r.GridY, exp[0], exp[1])
			}
		}
	}
}

func TestComputeGridLayoutStartRoomAtOrigin(t *testing.T) {
	w := &World{
		StartRoom: "s",
		Rooms:     []Room{{ID: "s", Exits: map[string]string{}}},
	}
	buildTestIndex(w)
	w.computeGridLayout()
	for _, r := range w.Rooms {
		if r.ID == "s" && (r.GridX != 0 || r.GridY != 0) {
			t.Errorf("start room should be at (0,0) got (%d,%d)", r.GridX, r.GridY)
		}
	}
}

func TestComputeGridLayoutNoCardinalExits(t *testing.T) {
	w := &World{
		StartRoom: "only",
		Rooms:     []Room{{ID: "only", Exits: map[string]string{}}},
	}
	buildTestIndex(w)
	// Must not panic
	w.computeGridLayout()
}

func TestWorldUIProfileParsedFromYAML(t *testing.T) {
	raw := []byte(`
name: testworld
start_room: r1
rooms: []
ui:
  profile: kids
  prompt: "$"
  tagline: "test"
  theme:
    bg: "#000"
`)
	var w World
	if err := yaml.Unmarshal(raw, &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if w.UI.Profile != "kids" {
		t.Errorf("Profile: got %q want %q", w.UI.Profile, "kids")
	}
}

func TestFindItemAnywhere(t *testing.T) {
	w := &World{
		Rooms: []Room{
			{
				ID: "room-0",
				Items: []Item{
					{ID: "bread", Name: "Bread", Desc: "Food."},
				},
			},
		},
		CraftingRecipes: []CraftingRecipe{
			{
				ID:     "leather-armor",
				Name:   "Leather Armor",
				Output: Item{ID: "leather-armor", Name: "Leather Armor", Tags: []string{"armor"}, Stats: map[string]int{"damage_resist": 2}},
			},
		},
	}
	// Build the index manually (same package — unexported field is accessible)
	w.index = map[string]*Room{"room-0": &w.Rooms[0]}

	// Finds room items
	if item := w.FindItemAnywhere("bread"); item == nil {
		t.Error("FindItemAnywhere: expected to find 'bread' in room items")
	}
	// Finds recipe outputs
	item := w.FindItemAnywhere("leather-armor")
	if item == nil {
		t.Fatal("FindItemAnywhere: expected to find 'leather-armor' in recipe outputs")
	}
	if item.Stats["damage_resist"] != 2 {
		t.Errorf("FindItemAnywhere: damage_resist: got %d want 2", item.Stats["damage_resist"])
	}
	// Returns nil for unknown
	if item := w.FindItemAnywhere("no-such-item"); item != nil {
		t.Error("FindItemAnywhere: expected nil for unknown item")
	}
}

func TestMudoutWorldLoads(t *testing.T) {
	w, err := Load("mudout")
	if err != nil {
		t.Fatalf("Load(mudout): %v", err)
	}
	if w.Name != "mudout" {
		t.Errorf("name: got %q want %q", w.Name, "mudout")
	}
	if w.StartRoom != "wakeup-camp" {
		t.Errorf("start_room: got %q want %q", w.StartRoom, "wakeup-camp")
	}
	if len(w.Rooms) != 14 {
		t.Errorf("rooms: got %d want 14", len(w.Rooms))
	}
	if len(w.Factions) != 4 {
		t.Errorf("factions: got %d want 4", len(w.Factions))
	}
	if w.UI.Profile != "wasteland" {
		t.Errorf("ui.profile: got %q want %q", w.UI.Profile, "wasteland")
	}
	if w.UI.Theme.BG != "#0d0d00" {
		t.Errorf("theme.bg: got %q want %q", w.UI.Theme.BG, "#0d0d00")
	}
	if r := w.Room("dusthaven-0"); r == nil {
		t.Error("start room dusthaven-0 not found in index")
	}
	if len(w.CraftingRecipes) < 2 {
		t.Errorf("crafting_recipes: got %d want >=2", len(w.CraftingRecipes))
	}
	if len(w.LootTables) != 4 {
		t.Errorf("loot_tables: got %d want 4", len(w.LootTables))
	}
}
