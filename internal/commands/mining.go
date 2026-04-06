package commands

import (
	"context"
	"fmt"
	"math/rand"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/db/sqliteq"
	"github.com/adam-stokes/gl1tch-mud/internal/enchanting"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/quests"
	"github.com/adam-stokes/gl1tch-mud/internal/weather"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

func init() {
	Registry["mine"]    = Mine
	Registry["harvest"] = Harvest
	Registry["gather"]  = Gather
	Registry["smelt"]   = Smelt
	Registry["plant"]   = Plant
}

// bumpActions increments the player action counter.
func bumpActions(gdb *gamedb.GameDB) {
	q := sqliteq.New(gdb.SQLiteDB())
	q.BumpActions(context.Background()) //nolint:errcheck
}

// isResourceDepleted checks if a resource in a room is currently depleted.
func isResourceDepleted(gdb *gamedb.GameDB, roomID, resourceID string, respawnActions int) bool {
	q := sqliteq.New(gdb.SQLiteDB())
	ctx := context.Background()
	row, err := q.GetResourceState(ctx, sqliteq.GetResourceStateParams{
		RoomID:     roomID,
		ResourceID: resourceID,
	})
	if err != nil {
		return false // no record = not depleted
	}
	if row.Depleted == 0 {
		return false
	}
	current := actionCount(gdb)
	if current >= int(row.DepletedAtAction)+respawnActions {
		q.ClearResourceDepletion(ctx, sqliteq.ClearResourceDepletionParams{ //nolint:errcheck
			RoomID:     roomID,
			ResourceID: resourceID,
		})
		return false
	}
	return true
}

// depleteResource marks a resource as depleted.
func depleteResource(gdb *gamedb.GameDB, roomID, resourceID string) {
	current := actionCount(gdb)
	q := sqliteq.New(gdb.SQLiteDB())
	q.DepleteResource(context.Background(), sqliteq.DepleteResourceParams{ //nolint:errcheck
		RoomID:           roomID,
		ResourceID:       resourceID,
		DepletedAtAction: int64(current),
	})
}

// rollYield rolls loot from a resource's yields list, applying weather + fortune enchant bonuses.
func rollYield(gdb *gamedb.GameDB, yields []world.LootEntry, biome string) []world.LootEntry {
	var bonusCount int
	items, _ := player.Inventory(gdb)
	for _, it := range items {
		enchants, _ := enchanting.List(gdb, it.ID)
		for _, e := range enchants {
			if e.EnchantID == "fortune" {
				bonusCount += enchanting.YieldBonus("fortune", e.Level)
			}
		}
	}

	cond, _ := weather.Current(gdb, biome)
	weatherBonus := weather.YieldBonus(cond)

	var out []world.LootEntry
	for _, entry := range yields {
		if rand.Float64() >= entry.Probability*weatherBonus {
			continue
		}
		count := entry.CountMin + rand.Intn(entry.CountMax-entry.CountMin+1) + bonusCount
		out = append(out, world.LootEntry{
			ItemID:   entry.ItemID,
			Name:     entry.Name,
			Desc:     entry.Desc,
			CountMin: count,
			CountMax: count,
		})
	}
	return out
}

// Mine lists or mines a resource in the current room.
func Mine(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nowhere to mine."}
	}

	var mineResources []world.Resource
	for _, r := range room.Resources {
		if r.Type == "mine" {
			mineResources = append(mineResources, r)
		}
	}

	if len(args) == 0 {
		if len(mineResources) == 0 {
			return Result{Output: "nothing to mine here."}
		}
		var b strings.Builder
		b.WriteString("mineable resources:\n")
		for _, r := range mineResources {
			status := ""
			if isResourceDepleted(gdb, s.RoomID, r.ID, r.RespawnActions) {
				status = " (depleted)"
			}
			fmt.Fprintf(&b, "  %s%s\n", r.ID, status)
		}
		return Result{Output: strings.TrimRight(b.String(), "\n")}
	}

	target := strings.ToLower(args[0])
	var res *world.Resource
	for i := range mineResources {
		if mineResources[i].ID == target {
			res = &mineResources[i]
			break
		}
	}
	if res == nil {
		return Result{Output: fmt.Sprintf("no mineable resource %q here.", target)}
	}
	if isResourceDepleted(gdb, s.RoomID, res.ID, res.RespawnActions) {
		return Result{Output: fmt.Sprintf("the %s is exhausted. come back later.", res.ID)}
	}

	if res.ToolRequired != "" {
		invIDs := inventoryIDs(gdb)
		hasTool := false
		for _, id := range invIDs {
			if strings.Contains(id, res.ToolRequired) {
				hasTool = true
				break
			}
		}
		if !hasTool {
			return Result{Output: fmt.Sprintf("you need a %s to mine this.", res.ToolRequired)}
		}
	}

	bumpActions(gdb)
	depleteResource(gdb, s.RoomID, res.ID)
	enchanting.AddXP(gdb, 5) //nolint:errcheck

	yields := rollYield(gdb, res.Yields, room.Biome)
	if len(yields) == 0 {
		return Result{Output: fmt.Sprintf("you mine the %s but find nothing useful.", res.ID)}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "you mine the %s...\n", res.ID)
	for _, y := range yields {
		player.AddItem(gdb, y.ItemID, y.Name, y.Desc) //nolint:errcheck
		fmt.Fprintf(&b, "  + %dx %s\n", y.CountMin, y.Name)
	}
	out := strings.TrimRight(b.String(), "\n")
	readyQuests, _ := quests.CheckMine(gdb, res.ID)
	for _, q := range readyQuests {
		out += fmt.Sprintf("\nquest ready: [%s] — type 'quest complete %s'", q.Title, q.ID)
	}
	return Result{Output: out}
}

// Harvest lists or harvests a resource in the current room.
func Harvest(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "nowhere to harvest."}
	}

	var harvestResources []world.Resource
	for _, r := range room.Resources {
		if r.Type == "harvest" {
			harvestResources = append(harvestResources, r)
		}
	}

	current := actionCount(gdb)
	q := sqliteq.New(gdb.SQLiteDB())
	seedIDs, _ := q.ListReadyCrops(context.Background(), sqliteq.ListReadyCropsParams{
		RoomID:        s.RoomID,
		ReadyAtAction: int64(current),
	})
	for _, seedID := range seedIDs {
		harvestResources = append(harvestResources, world.Resource{ID: seedID + "-crop", Type: "harvest"})
	}

	if len(args) == 0 {
		if len(harvestResources) == 0 {
			return Result{Output: "nothing to harvest here."}
		}
		var b strings.Builder
		b.WriteString("harvestable resources:\n")
		for _, r := range harvestResources {
			status := ""
			if isResourceDepleted(gdb, s.RoomID, r.ID, r.RespawnActions) {
				status = " (depleted)"
			}
			fmt.Fprintf(&b, "  %s%s\n", r.ID, status)
		}
		return Result{Output: strings.TrimRight(b.String(), "\n")}
	}

	target := strings.ToLower(args[0])

	if strings.HasSuffix(target, "-crop") {
		seedID := strings.TrimSuffix(target, "-crop")
		cropCount, _ := q.CountReadyCrops(context.Background(), sqliteq.CountReadyCropsParams{
			RoomID:        s.RoomID,
			SeedID:        seedID,
			ReadyAtAction: int64(current),
		})
		if cropCount == 0 {
			return Result{Output: "no ready crops of that type here."}
		}
		q.HarvestCrops(context.Background(), sqliteq.HarvestCropsParams{ //nolint:errcheck
			RoomID:        s.RoomID,
			SeedID:        seedID,
			ReadyAtAction: int64(current),
		})
		player.AddItem(gdb, seedID+"-harvest", strings.Title(seedID), "A freshly harvested crop.") //nolint:errcheck
		return Result{Output: fmt.Sprintf("you harvest the %s.", seedID)}
	}

	var res *world.Resource
	for i := range harvestResources {
		if harvestResources[i].ID == target {
			res = &harvestResources[i]
			break
		}
	}
	if res == nil {
		return Result{Output: fmt.Sprintf("no harvestable resource %q here.", target)}
	}
	if isResourceDepleted(gdb, s.RoomID, res.ID, res.RespawnActions) {
		return Result{Output: fmt.Sprintf("the %s is exhausted. come back later.", res.ID)}
	}

	bumpActions(gdb)
	depleteResource(gdb, s.RoomID, res.ID)

	yields := rollYield(gdb, res.Yields, room.Biome)
	if len(yields) == 0 {
		return Result{Output: fmt.Sprintf("you harvest the %s but find nothing useful.", res.ID)}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "you harvest the %s...\n", res.ID)
	for _, y := range yields {
		player.AddItem(gdb, y.ItemID, y.Name, y.Desc) //nolint:errcheck
		fmt.Fprintf(&b, "  + %dx %s\n", y.CountMin, y.Name)
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

// Gather picks up ambient resources from the environment (no tool required).
func Gather(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	room := w.Room(s.RoomID)
	biome := "meadow"
	if room != nil {
		biome = room.Biome
	}

	const cooldown = 20
	if isResourceDepleted(gdb, s.RoomID+"-gather", "gather-cooldown", cooldown) {
		return Result{Output: "you need to rest before gathering again."}
	}

	bumpActions(gdb)
	depleteResource(gdb, s.RoomID+"-gather", "gather-cooldown")

	ambient := map[string][]world.LootEntry{
		"meadow": {
			{ItemID: "flint", Name: "Flint", Desc: "A sharp piece of flint.", Probability: 0.8, CountMin: 1, CountMax: 2},
			{ItemID: "stick", Name: "Stick", Desc: "A sturdy stick.", Probability: 0.9, CountMin: 1, CountMax: 3},
			{ItemID: "wildflower", Name: "Wildflower", Desc: "A cheerful wildflower.", Probability: 0.5, CountMin: 1, CountMax: 1},
		},
		"forest": {
			{ItemID: "stick", Name: "Stick", Desc: "A sturdy stick.", Probability: 0.9, CountMin: 1, CountMax: 3},
			{ItemID: "berry", Name: "Berry", Desc: "Wild berries.", Probability: 0.6, CountMin: 1, CountMax: 3},
			{ItemID: "leaf", Name: "Leaf", Desc: "A large leaf.", Probability: 0.7, CountMin: 1, CountMax: 2},
		},
		"desert": {
			{ItemID: "sand", Name: "Sand", Desc: "Fine desert sand.", Probability: 0.95, CountMin: 2, CountMax: 5},
			{ItemID: "flint", Name: "Flint", Desc: "A sharp piece of flint.", Probability: 0.6, CountMin: 1, CountMax: 2},
			{ItemID: "bone", Name: "Bone", Desc: "An old bone.", Probability: 0.4, CountMin: 1, CountMax: 1},
		},
		"snow": {
			{ItemID: "ice", Name: "Ice", Desc: "A chunk of ice.", Probability: 0.9, CountMin: 1, CountMax: 3},
			{ItemID: "pebble", Name: "Pebble", Desc: "A smooth pebble.", Probability: 0.7, CountMin: 1, CountMax: 2},
			{ItemID: "snowball", Name: "Snowball", Desc: "A perfectly packed snowball.", Probability: 0.8, CountMin: 1, CountMax: 2},
		},
		"caves": {
			{ItemID: "gravel", Name: "Gravel", Desc: "Loose gravel.", Probability: 0.9, CountMin: 1, CountMax: 3},
			{ItemID: "coal", Name: "Coal", Desc: "A lump of coal.", Probability: 0.5, CountMin: 1, CountMax: 2},
			{ItemID: "moss", Name: "Moss", Desc: "Damp cave moss.", Probability: 0.6, CountMin: 1, CountMax: 2},
		},
	}

	pool, ok := ambient[biome]
	if !ok {
		pool = ambient["meadow"]
	}

	yields := rollYield(gdb, pool, biome)
	if len(yields) == 0 {
		return Result{Output: "you search the area but find nothing useful."}
	}

	var b strings.Builder
	b.WriteString("you gather from the surroundings...\n")
	for _, y := range yields {
		player.AddItem(gdb, y.ItemID, y.Name, y.Desc) //nolint:errcheck
		fmt.Fprintf(&b, "  + %dx %s\n", y.CountMin, y.Name)
		readyQuests, _ := quests.CheckGather(gdb, y.ItemID)
		for _, q := range readyQuests {
			fmt.Fprintf(&b, "\nquest ready: [%s] — type 'quest complete %s'", q.Title, q.ID)
		}
	}
	return Result{Output: strings.TrimRight(b.String(), "\n")}
}

// Smelt converts ores to ingots using a furnace.
func Smelt(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "smelt <item-id> — requires a furnace and fuel (wood or coal)"}
	}

	hasFurnace := false
	if room := w.Room(s.RoomID); room != nil {
		for _, sys := range room.Systems {
			if sys.ID == "furnace" {
				hasFurnace = true
				break
			}
		}
	}
	if !hasFurnace {
		q := sqliteq.New(gdb.SQLiteDB())
		cnt, _ := q.CountBuildsByType(context.Background(), sqliteq.CountBuildsByTypeParams{
			RoomID:  s.RoomID,
			BuildID: "furnace",
		})
		hasFurnace = cnt > 0
	}
	if !hasFurnace {
		return Result{Output: "you need a furnace to smelt. build one with 'build furnace' or find one."}
	}

	invIDs := inventoryIDs(gdb)
	fuel := ""
	for _, id := range invIDs {
		if id == "coal" || id == "wood-log" || id == "charcoal" {
			fuel = id
			break
		}
	}
	if fuel == "" {
		return Result{Output: "you need fuel (coal, wood-log, or charcoal) to smelt."}
	}

	itemID := strings.ToLower(args[0])
	smeltMap := map[string][2]string{
		"iron-ore": {"iron-ingot", "Iron Ingot"},
		"gold-ore": {"gold-ingot", "Gold Ingot"},
		"sand":     {"glass", "Glass"},
		"clay":     {"brick", "Brick"},
		"coal-ore": {"coal", "Coal"},
		"wood-log": {"charcoal", "Charcoal"},
	}

	result, ok := smeltMap[itemID]
	if !ok {
		return Result{Output: fmt.Sprintf("%s cannot be smelted.", itemID)}
	}

	hasItem := false
	for _, id := range invIDs {
		if id == itemID {
			hasItem = true
			break
		}
	}
	if !hasItem {
		return Result{Output: fmt.Sprintf("you don't have %s.", itemID)}
	}

	if err := player.RemoveItem(gdb, itemID); err != nil {
		return Result{Output: fmt.Sprintf("you don't have %s.", itemID)}
	}
	if fuel != itemID {
		if err := player.RemoveItem(gdb, fuel); err != nil {
			return Result{Output: fmt.Sprintf("you don't have %s for fuel.", fuel)}
		}
	}
	player.AddItem(gdb, result[0], result[1], fmt.Sprintf("Smelted from %s.", itemID)) //nolint:errcheck
	bumpActions(gdb)

	out := fmt.Sprintf("you feed the furnace with %s and smelt the %s.\nyou receive: 1x %s.", fuel, itemID, result[1])
	readyQuests, _ := quests.CheckSmelt(gdb, result[0])
	for _, rq := range readyQuests {
		out += fmt.Sprintf("\nquest ready: [%s] — type 'quest complete %s'", rq.Title, rq.ID)
	}
	return Result{Output: out}
}

// Plant plants a seed in the current room.
func Plant(gdb *gamedb.GameDB, s *player.State, w *world.World, args []string) Result {
	if len(args) == 0 {
		return Result{Output: "plant <seed-id>"}
	}

	room := w.Room(s.RoomID)
	if room == nil {
		return Result{Output: "you can't plant here."}
	}

	canPlant := room.Biome == "meadow"
	if !canPlant {
		q := sqliteq.New(gdb.SQLiteDB())
		cnt, _ := q.CountBuildsByType(context.Background(), sqliteq.CountBuildsByTypeParams{
			RoomID:  s.RoomID,
			BuildID: "garden-plot",
		})
		canPlant = cnt > 0
	}
	if !canPlant {
		return Result{Output: "you need farmland to plant. try a meadow room or build a garden-plot."}
	}

	seedID := strings.ToLower(args[0])
	invIDs := inventoryIDs(gdb)
	hasSeed := false
	for _, id := range invIDs {
		if id == seedID {
			hasSeed = true
			break
		}
	}
	if !hasSeed {
		return Result{Output: fmt.Sprintf("you don't have %s.", seedID)}
	}

	growActions := 15
	for _, r := range room.Resources {
		if r.Type == "plant" && r.ID == seedID {
			if r.GrowActions > 0 {
				growActions = r.GrowActions
			}
			break
		}
	}

	current := actionCount(gdb)

	// Find the lowest available slot (0-3) not occupied by an active crop.
	q := sqliteq.New(gdb.SQLiteDB())
	usedSlotsList, _ := q.ListActiveCropSlots(context.Background(), s.RoomID)
	usedSlots := map[int]bool{}
	for _, used := range usedSlotsList {
		usedSlots[int(used)] = true
	}
	slot := -1
	for i := 0; i < 4; i++ {
		if !usedSlots[i] {
			slot = i
			break
		}
	}
	if slot == -1 {
		return Result{Output: "the farmland is full. harvest some crops first."}
	}

	player.RemoveItem(gdb, seedID) //nolint:errcheck
	q.InsertCrop(context.Background(), sqliteq.InsertCropParams{ //nolint:errcheck
		RoomID:          s.RoomID,
		Slot:            int64(slot),
		SeedID:          seedID,
		PlantedAtAction: int64(current),
		ReadyAtAction:   int64(current + growActions),
	})
	bumpActions(gdb)

	return Result{Output: fmt.Sprintf(
		"you plant the %s in the soil. it will be ready to harvest in about %d actions.",
		seedID, growActions,
	)}
}
