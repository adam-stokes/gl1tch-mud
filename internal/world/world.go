package world

import (
	"database/sql"
	"embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed defaults/cyberspace/world.yaml defaults/blockhaven/world.yaml
var defaultWorldFS embed.FS

// System is a hackable terminal or node inside a room.
type System struct {
	ID            string `yaml:"id"`
	SecurityLevel int    `yaml:"security_level"`
	RewardItem    string `yaml:"reward_item,omitempty"`
	RewardText    string `yaml:"reward_text,omitempty"`
	ICE           string `yaml:"ice,omitempty"`
}

// Lock is a locked exit or container in a room.
type Lock struct {
	ID         string   `yaml:"id"`
	Exit       string   `yaml:"exit"`
	Difficulty int      `yaml:"difficulty"`
	Keys       []string `yaml:"keys,omitempty"`
}

// DialogueLine is one line of NPC dialogue with a trigger condition.
type DialogueLine struct {
	Trigger string `yaml:"trigger"`
	Text    string `yaml:"text"`
	QuestID string `yaml:"quest_id,omitempty"`
}

// TradeIngredient is an item required or offered in a trade.
type TradeIngredient struct {
	ID    string `yaml:"id"`
	Name  string `yaml:"name,omitempty"`
	Desc  string `yaml:"desc,omitempty"`
	Count int    `yaml:"count"`
}

// TradeOffer is a single trade an NPC can make.
type TradeOffer struct {
	ID         string            `yaml:"id"`
	Wants      []TradeIngredient `yaml:"wants"`
	Offers     []TradeIngredient `yaml:"offers"`
	FactionReq string            `yaml:"faction_req,omitempty"`
}

// CraftingIngredient is an item required for a crafting recipe.
type CraftingIngredient struct {
	ID    string `yaml:"id"`
	Count int    `yaml:"count"`
}

// CraftingRecipeType controls which crafting path is used.
type CraftingRecipeType string

const (
	RecipeTypeIngredient CraftingRecipeType = "ingredient"
	RecipeTypeAssembly   CraftingRecipeType = "assembly"
)

// CraftingSlot is a named slot in an assembly recipe.
type CraftingSlot struct {
	ID         string         `yaml:"id"`
	Name       string         `yaml:"name"`
	Required   bool           `yaml:"required"`
	AcceptsTag string         `yaml:"accepts_tag"`
	StatMods   map[string]int `yaml:"stat_mods,omitempty"`
}

// CraftingRecipe defines how to craft an item.
type CraftingRecipe struct {
	ID             string               `yaml:"id"`
	Name           string               `yaml:"name"`
	Ingredients    []CraftingIngredient `yaml:"ingredients"`
	Output         Item                 `yaml:"output"`
	SkillReq       int                  `yaml:"skill_req,omitempty"`
	Workbench      string               `yaml:"workbench,omitempty"`
	TierThresholds []int                `yaml:"tier_thresholds,omitempty"`
	TierNames      []string             `yaml:"tier_names,omitempty"`
	Type           CraftingRecipeType   `yaml:"type,omitempty"`
	Slots          []CraftingSlot       `yaml:"slots,omitempty"`
}

// LootEntry is a single item in a loot table.
type LootEntry struct {
	ItemID      string         `yaml:"item_id"`
	Name        string         `yaml:"name,omitempty"`
	Desc        string         `yaml:"desc,omitempty"`
	Probability float64        `yaml:"probability"`
	CountMin    int            `yaml:"count_min"`
	CountMax    int            `yaml:"count_max"`
	Faction     string         `yaml:"faction,omitempty"`
	Tags        []string       `yaml:"tags,omitempty"`
	StatMods    map[string]int `yaml:"stat_mods,omitempty"`
	Quality     string         `yaml:"quality,omitempty"`
	Weight      int            `yaml:"weight,omitempty"`
}

// Resource is a mineable or harvestable node inside a room.
type Resource struct {
	ID             string      `yaml:"id"`
	Type           string      `yaml:"type"` // "mine" | "harvest" | "plant"
	Yields         []LootEntry `yaml:"yields"`
	ToolRequired   string      `yaml:"tool_required,omitempty"`
	RespawnActions int         `yaml:"respawn_actions,omitempty"`
	GrowActions    int         `yaml:"grow_actions,omitempty"` // for plant seeds
}

// WeatherEntry lists possible weather conditions for one biome.
type WeatherEntry struct {
	Biome    string   `yaml:"biome"`
	Possible []string `yaml:"possible"`
}

// WorldTheme holds the color palette for a world's UI.
type WorldTheme struct {
	BG      string `yaml:"bg"      json:"bg,omitempty"`
	FG      string `yaml:"fg"      json:"fg,omitempty"`
	Accent  string `yaml:"accent"  json:"accent,omitempty"`
	Dim     string `yaml:"dim"     json:"dim,omitempty"`
	Border  string `yaml:"border"  json:"border,omitempty"`
	Error   string `yaml:"error"   json:"error,omitempty"`
	Success string `yaml:"success" json:"success,omitempty"`
}

// WorldUI holds the presentation config for a world, read from the ui: block.
type WorldUI struct {
	Profile string     `yaml:"profile,omitempty"`
	Banner  string     `yaml:"banner"`
	Prompt  string     `yaml:"prompt"`
	Tagline string     `yaml:"tagline"`
	Theme   WorldTheme `yaml:"theme"`
}

// WorldMeta is a lightweight summary of a world used in lobby listings.
type WorldMeta struct {
	Name    string     `json:"name"`
	Tagline string     `json:"tagline"`
	Theme   WorldTheme `json:"theme"`
}

// LootTable holds a named set of loot entries.
type LootTable struct {
	ID      string      `yaml:"id"`
	Entries []LootEntry `yaml:"entries"`
}

// NPC is an enemy or character in a room.
type NPC struct {
	ID          string       `yaml:"id"`
	Name        string       `yaml:"name"`
	HP          int          `yaml:"hp"`
	Attack      int          `yaml:"attack"`
	Desc        string       `yaml:"desc"`
	LootTableID string       `yaml:"loot_table_id,omitempty"`
	Trades      []TradeOffer `yaml:"trades,omitempty"`
	Dialogue    []DialogueLine `yaml:"dialogue,omitempty"`
}

// Faction is a political or criminal organisation in the world.
type Faction struct {
	ID        string   `yaml:"id"`
	Name      string   `yaml:"name"`
	Desc      string   `yaml:"desc"`
	Agenda    string   `yaml:"agenda"`
	Territory []string `yaml:"territory,omitempty"`
	Allies    []string `yaml:"allies,omitempty"`
	Enemies   []string `yaml:"enemies,omitempty"`
}

// Item is a collectable object in a room.
// SignalTier encodes rarity in cyberspace lingo:
//
//	noise     — junk, scrapmetal, barely functional
//	signal    — usable, standard underground gear
//	ghost     — clean, rare, off-grid provenance
//	zero-day  — exploit-grade, one-of-a-kind
//	flatline  — legendary, world-altering
type Item struct {
	ID            string `yaml:"id"`
	Name          string `yaml:"name"`
	Desc          string `yaml:"desc"`
	SignalTier    string `yaml:"signal_tier,omitempty"`
	IsDisguise    bool   `yaml:"is_disguise,omitempty"`
	Readable      bool   `yaml:"readable,omitempty"`
	Content       string `yaml:"content,omitempty"`
	IsBlueprint   bool   `yaml:"is_blueprint,omitempty"`
	UnlocksRecipe string `yaml:"unlocks_recipe,omitempty"`
	IsContainer   bool   `yaml:"is_container,omitempty"`
	Capacity      int    `yaml:"capacity,omitempty"`
	IsExploit     bool   `yaml:"is_exploit,omitempty"`
	TargetsSystem string `yaml:"targets_system,omitempty"`
	IsAugment     bool   `yaml:"is_augment,omitempty"`
	AugmentSkill  string `yaml:"augment_skill,omitempty"`
	AugmentBonus  int    `yaml:"augment_bonus,omitempty"`
	ModSlots      int            `yaml:"mod_slots,omitempty"`
	IsMod         bool           `yaml:"is_mod,omitempty"`
	Tags          []string       `yaml:"tags,omitempty"`
	Stats         map[string]int `yaml:"stats,omitempty"`
	StatMods      map[string]int `yaml:"stat_mods,omitempty"`
	Quality       string         `yaml:"quality,omitempty"`
	UnlocksFlag   string         `yaml:"unlocks_flag,omitempty"`
}

// Room is a single location in the world.
type Room struct {
	ID      string            `yaml:"id"`
	Name    string            `yaml:"name"`
	Desc    string            `yaml:"desc"`
	Exits   map[string]string `yaml:"exits"`
	NPCs    []NPC             `yaml:"npcs"`
	Items   []Item            `yaml:"items"`
	Systems   []System   `yaml:"systems,omitempty"`
	Locks     []Lock     `yaml:"locks,omitempty"`
	Biome     string     `yaml:"biome,omitempty"`
	Resources      []Resource `yaml:"resources,omitempty"`
	WorkbenchTypes []string   `yaml:"workbench_types,omitempty"`
	// GridX and GridY are computed at load time via BFS from StartRoom.
	// They are not stored in YAML.
	GridX int `yaml:"-"`
	GridY int `yaml:"-"`
}

// WorldQuest is a pre-defined quest loaded from world YAML.
type WorldQuest struct {
	ID             string `yaml:"id"`
	Title          string `yaml:"title"`
	Description    string `yaml:"description"`
	GiverNPCID     string `yaml:"giver_npc_id"`
	ObjType        string `yaml:"obj_type"`
	ObjTarget      string `yaml:"obj_target"`
	ObjRoom        string `yaml:"obj_room,omitempty"`
	ObjCount       int    `yaml:"obj_count"`
	RewardCredits  int    `yaml:"reward_credits"`
	RewardXPSkill  string `yaml:"reward_xp_skill,omitempty"`
	RewardXPAmount int    `yaml:"reward_xp_amount,omitempty"`
	RewardItemID   string `yaml:"reward_item_id,omitempty"`
	RewardItemName string `yaml:"reward_item_name,omitempty"`
	RewardItemDesc string `yaml:"reward_item_desc,omitempty"`
	NextQuestID    string `yaml:"next_quest_id,omitempty"`
}

// World holds all rooms for a loaded world.
type World struct {
	Name            string           `yaml:"name"`
	StartRoom       string           `yaml:"start_room"`
	NarratorModel   string           `yaml:"narrator_model"`
	Rooms           []Room           `yaml:"rooms"`
	CraftingRecipes []CraftingRecipe `yaml:"crafting_recipes,omitempty"`
	LootTables      []LootTable      `yaml:"loot_tables,omitempty"`
	Factions        []Faction        `yaml:"factions,omitempty"`
	Quests          []WorldQuest     `yaml:"quests,omitempty"`
	WeatherTable    []WeatherEntry   `yaml:"weather_table,omitempty"`
	UI              WorldUI          `yaml:"ui"`
	index           map[string]*Room
}

// Load reads a world YAML from ~/.local/share/gl1tch-mud/worlds/<name>/world.yaml.
// Falls back to the embedded cyberspace world if not found.
func Load(name string) (*World, error) {
	path := worldPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		// fall back to embedded world if available
		embedded := "defaults/" + name + "/world.yaml"
		data, err = defaultWorldFS.ReadFile(embedded)
		if err != nil {
			// last resort: embedded cyberspace
			data, err = defaultWorldFS.ReadFile("defaults/cyberspace/world.yaml")
			if err != nil {
				return nil, fmt.Errorf("world: load default: %w", err)
			}
		}
	}
	var w World
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("world: parse %s: %w", name, err)
	}
	w.index = make(map[string]*Room, len(w.Rooms))
	for i := range w.Rooms {
		w.index[w.Rooms[i].ID] = &w.Rooms[i]
	}
	w.computeGridLayout()
	return &w, nil
}

// Available returns the names of all installed worlds plus all embedded defaults.
// Always includes "cyberspace".
func Available() []string {
	seen := map[string]bool{}
	var names []string

	// Start with embedded worlds.
	entries, _ := defaultWorldFS.ReadDir("defaults")
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		n := e.Name()
		p := "defaults/" + n + "/world.yaml"
		if _, err := defaultWorldFS.Open(p); err == nil {
			names = append(names, n)
			seen[n] = true
		}
	}

	// Also scan user-installed worlds.
	home, err := os.UserHomeDir()
	if err != nil {
		return names
	}
	dir := filepath.Join(home, ".local", "share", "gl1tch-mud", "worlds")
	dirEntries, err := os.ReadDir(dir)
	if err != nil {
		return names
	}
	for _, e := range dirEntries {
		if !e.IsDir() {
			continue
		}
		n := e.Name()
		if seen[n] {
			continue // already included via embedded
		}
		// Only include if world.yaml exists
		p := filepath.Join(dir, n, "world.yaml")
		if _, err := os.Stat(p); err == nil {
			names = append(names, n)
			seen[n] = true
		}
	}
	return names
}

// UIPrompt returns the world's prompt string, falling back to ">".
func (w *World) UIPrompt() string {
	if w.UI.Prompt != "" {
		return w.UI.Prompt
	}
	return ">"
}

// UIBanner returns the world's banner string (may be empty).
func (w *World) UIBanner() string {
	return w.UI.Banner
}

// ListAvailable returns WorldMeta for all installed worlds plus embedded defaults.
// Ordering matches Available() — embedded defaults first, then user-installed.
func ListAvailable() []WorldMeta {
	names := Available()
	metas := make([]WorldMeta, 0, len(names))
	for _, name := range names {
		w, err := Load(name)
		if err != nil {
			continue
		}
		metas = append(metas, WorldMeta{
			Name:    w.Name,
			Tagline: w.UI.Tagline,
			Theme:   w.UI.Theme,
		})
	}
	return metas
}

// computeGridLayout assigns GridX/GridY to each room by BFS from StartRoom,
// following cardinal exits. Non-cardinal exits are ignored. If two rooms
// would land on the same cell, the second is nudged right until a free cell
// is found; a warning is logged.
func (w *World) computeGridLayout() {
	type pos struct{ x, y int }
	offsets := map[string]pos{
		"north": {0, -1}, "south": {0, 1},
		"east": {1, 0}, "west": {-1, 0},
	}
	occupied := map[pos]string{}
	coords := map[string]pos{}

	start := pos{0, 0}
	coords[w.StartRoom] = start
	occupied[start] = w.StartRoom
	queue := []string{w.StartRoom}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		room := w.Room(cur)
		if room == nil {
			continue
		}
		curPos := coords[cur]
		for dir, neighborID := range room.Exits {
			if _, seen := coords[neighborID]; seen {
				continue
			}
			off, ok := offsets[dir]
			if !ok {
				continue
			}
			candidate := pos{curPos.x + off.x, curPos.y + off.y}
			for {
				if existing, taken := occupied[candidate]; !taken || existing == neighborID {
					break
				}
				log.Printf("gl1tch-mud: grid collision at (%d,%d) for room %q, nudging\n", candidate.x, candidate.y, neighborID)
				candidate.x++
			}
			coords[neighborID] = candidate
			occupied[candidate] = neighborID
			queue = append(queue, neighborID)
		}
	}

	for i := range w.Rooms {
		if p, ok := coords[w.Rooms[i].ID]; ok {
			w.Rooms[i].GridX = p.x
			w.Rooms[i].GridY = p.y
		}
	}
}

// Room returns the room with the given ID, or nil.
func (w *World) Room(id string) *Room {
	return w.index[id]
}

// AddRoom adds a generated room to the world graph at runtime.
func (w *World) AddRoom(r *Room) {
	if w.index == nil {
		w.index = make(map[string]*Room)
	}
	w.Rooms = append(w.Rooms, *r)
	w.index[r.ID] = &w.Rooms[len(w.Rooms)-1]
}

// FindFaction returns the faction with the given ID, or nil.
func (w *World) FindFaction(id string) *Faction {
	for i := range w.Factions {
		if w.Factions[i].ID == id {
			return &w.Factions[i]
		}
	}
	return nil
}

// FindLootTable returns the loot table with the given ID, or nil.
func (w *World) FindLootTable(id string) *LootTable {
	for i := range w.LootTables {
		if w.LootTables[i].ID == id {
			return &w.LootTables[i]
		}
	}
	return nil
}

// FindRecipe returns the crafting recipe with the given ID, or nil.
func (w *World) FindRecipe(id string) *CraftingRecipe {
	for i := range w.CraftingRecipes {
		if w.CraftingRecipes[i].ID == id {
			return &w.CraftingRecipes[i]
		}
	}
	return nil
}

// FindQuest returns the pre-defined quest with the given ID, or nil.
func (w *World) FindQuest(id string) *WorldQuest {
	for i := range w.Quests {
		if w.Quests[i].ID == id {
			return &w.Quests[i]
		}
	}
	return nil
}

// FindItem searches all rooms for an item with the given ID and returns a pointer to it, or nil.
func (w *World) FindItem(id string) *Item {
	for i := range w.Rooms {
		for j := range w.Rooms[i].Items {
			if w.Rooms[i].Items[j].ID == id {
				return &w.Rooms[i].Items[j]
			}
		}
	}
	return nil
}

// FindLock returns the lock for the given exit direction in the room, or nil.
func (r *Room) FindLock(exitDir string) *Lock {
	for i := range r.Locks {
		if r.Locks[i].Exit == exitDir {
			return &r.Locks[i]
		}
	}
	return nil
}

// FindSystem returns the system with the given ID in the room, or nil.
func (r *Room) FindSystem(systemID string) *System {
	for i := range r.Systems {
		if r.Systems[i].ID == systemID {
			return &r.Systems[i]
		}
	}
	return nil
}

// Render returns a formatted description of the room.
func (r *Room) Render(visitedBefore bool) string {
	const (
		reset  = "\x1b[0m"
		bold   = "\x1b[1m"
		cyan   = "\x1b[36m"
		green  = "\x1b[32m"
		yellow = "\x1b[33m"
		purple = "\x1b[34m"
		orange = "\x1b[33m" // bold yellow for high-tier items
		red    = "\x1b[31m"
		dim    = "\x1b[37m"
	)

	var b strings.Builder
	b.WriteString("\n")
	b.WriteString(bold + cyan + "[ " + r.Name + " ]" + reset + "\n")
	b.WriteString(dim + strings.TrimSpace(r.Desc) + reset + "\n")

	if len(r.Exits) > 0 {
		dirs := make([]string, 0, len(r.Exits))
		for d := range r.Exits {
			dirs = append(dirs, d)
		}
		b.WriteString("\n" + green + "exits: " + strings.Join(dirs, ", ") + reset + "\n")
	}

	if len(r.NPCs) > 0 {
		for _, npc := range r.NPCs {
			b.WriteString(yellow + "  ! " + npc.Name + " is here." + reset + "\n")
		}
	}

	if len(r.Items) > 0 {
		for _, item := range r.Items {
			var color string
			switch item.SignalTier {
			case "ghost", "zero-day":
				color = bold + orange
			case "flatline":
				color = bold + red
			default:
				color = purple
			}
			prefix := "  + "
			if item.SignalTier != "" && item.SignalTier != "noise" && item.SignalTier != "signal" {
				prefix = "  + [" + strings.ToUpper(item.SignalTier) + "] "
			}
			b.WriteString(color + prefix + item.Name + " is on the ground." + reset + "\n")
		}
	}

	return b.String()
}

func worldPath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "gl1tch-mud", "worlds", name, "world.yaml")
}

// SeedCrystalShards inserts the five Crystal Shard rows for Blockhaven if they don't exist.
// Safe to call on any world — only acts when world name is "blockhaven".
func SeedCrystalShards(db *sql.DB, worldName string) error {
	if worldName != "blockhaven" {
		return nil
	}
	shards := []struct{ id, biome string }{
		{"meadow-shard", "meadow"},
		{"forest-shard", "forest"},
		{"desert-shard", "desert"},
		{"mountain-shard", "snow"},
		{"cave-shard", "caves"},
	}
	for _, s := range shards {
		if _, err := db.Exec(
			`INSERT OR IGNORE INTO crystal_shards (shard_id, biome, collected, collected_at) VALUES (?,?,0,0)`,
			s.id, s.biome,
		); err != nil {
			return err
		}
	}
	return nil
}

// SeedStartingItems adds starting items for the blockhaven world if inventory is empty.
func SeedStartingItems(db *sql.DB, worldName string) error {
	if worldName != "blockhaven" {
		return nil
	}
	// Check if starting items already seeded by probing for one specific item.
	var cnt int
	db.QueryRow(`SELECT COUNT(*) FROM inventory WHERE item_id='wooden-pickaxe'`).Scan(&cnt) //nolint:errcheck
	if cnt > 0 {
		return nil
	}
	items := []struct{ id, name, desc string }{
		{"wooden-pickaxe", "Wooden Pickaxe", "A basic pickaxe. Required for mining stone and ore."},
		{"wooden-sword", "Wooden Sword", "A basic sword. 5 attack."},
		{"bread", "Bread", "Restores 20 HP when eaten."},
		{"builders-map", "Builder's Map", "A hand-drawn map of Blockhaven."},
	}
	for _, it := range items {
		db.Exec(`INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?,?,?)`, //nolint:errcheck
			it.id, it.name, it.desc)
	}
	return nil
}
