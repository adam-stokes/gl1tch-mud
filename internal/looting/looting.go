// Package looting implements loot table rolling when NPCs die.
package looting

import (
	"fmt"
	"math/rand"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// Roll looks up the NPC's loot table and rolls for each entry.
// Returns a slice of world.Item that were dropped (possibly empty).
// count > 1 entries generate one Item per copy (same ID, unique names won't clash in the room).
// disguiseFaction doubles the drop probability for entries whose Faction matches.
func Roll(w *world.World, npcID string, disguiseFaction string) []world.Item {
	// Find the NPC in any room to get its loot_table_id.
	var lootTableID string
	for _, room := range w.Rooms {
		for _, npc := range room.NPCs {
			if npc.ID == npcID {
				lootTableID = npc.LootTableID
				break
			}
		}
		if lootTableID != "" {
			break
		}
	}
	if lootTableID == "" {
		return nil
	}

	table := w.FindLootTable(lootTableID)
	if table == nil {
		return nil
	}

	var dropped []world.Item
	for _, entry := range table.Entries {
		if entry.Probability <= 0 {
			continue
		}
		prob := entry.Probability
		if entry.Faction != "" && entry.Faction == disguiseFaction {
			prob = min(1.0, prob*2.0)
		}
		roll := rand.Float64()
		if roll <= prob {
			count := entry.CountMin
			if entry.CountMax > entry.CountMin {
				count = entry.CountMin + rand.Intn(entry.CountMax-entry.CountMin+1)
			}
			for i := 0; i < count; i++ {
				name := entry.Name
				if name == "" {
					name = entry.ItemID
				}
				desc := entry.Desc
				if desc == "" {
					desc = fmt.Sprintf("dropped by %s", npcID)
				}
				dropped = append(dropped, world.Item{
					ID:   entry.ItemID,
					Name: name,
					Desc: desc,
				})
			}
		}
	}
	return dropped
}
