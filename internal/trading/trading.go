// Package trading implements NPC trade offers and execution.
package trading

import (
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// Reputation returns the player's reputation with a faction (0 if not found).
func Reputation(db *sql.DB, faction string) int {
	var value int
	db.QueryRow(`SELECT value FROM player_reputation WHERE faction=?`, faction).Scan(&value) //nolint:errcheck
	return value
}

// IncrementReputation adds 1 to a faction's reputation.
func IncrementReputation(db *sql.DB, faction string) error {
	_, err := db.Exec(
		`INSERT INTO player_reputation (faction, value) VALUES (?,1)
		 ON CONFLICT(faction) DO UPDATE SET value=value+1`,
		faction,
	)
	return err
}

// parseFactionReq splits "faction:minRep" → (faction, minRep).
func parseFactionReq(req string) (string, int) {
	if req == "" {
		return "", 0
	}
	parts := strings.SplitN(req, ":", 2)
	if len(parts) != 2 {
		return req, 0
	}
	n, _ := strconv.Atoi(parts[1])
	return parts[0], n
}

// ListOffers returns all trades the NPC will offer given player reputation.
func ListOffers(w *world.World, npc *world.NPC, db *sql.DB) []world.TradeOffer {
	var result []world.TradeOffer
	for _, trade := range npc.Trades {
		faction, minRep := parseFactionReq(trade.FactionReq)
		if faction != "" {
			rep := Reputation(db, faction)
			if rep < minRep {
				continue
			}
		}
		result = append(result, trade)
	}
	return result
}

// FindNPCInRoom finds an NPC by ID in the given room.
func FindNPCInRoom(room *world.Room, npcID string) *world.NPC {
	for i := range room.NPCs {
		if room.NPCs[i].ID == npcID {
			return &room.NPCs[i]
		}
	}
	return nil
}

// ExecuteResult describes the outcome of a trade execution.
type ExecuteResult struct {
	OK         bool
	Message    string
	GaveItems  []string
	GotItems   []world.TradeIngredient
	FactionInc string
}

// Execute performs a trade identified by tradeID with an NPC.
// inventory is a map[itemID]bool of items the player carries.
func Execute(db *sql.DB, npc *world.NPC, tradeID string, inventoryIDs []string) ExecuteResult {
	// Find trade
	var trade *world.TradeOffer
	for i := range npc.Trades {
		if npc.Trades[i].ID == tradeID {
			trade = &npc.Trades[i]
			break
		}
	}
	if trade == nil {
		return ExecuteResult{Message: fmt.Sprintf("no trade %q available from %s.", tradeID, npc.Name)}
	}

	// Check faction rep
	faction, minRep := parseFactionReq(trade.FactionReq)
	if faction != "" {
		rep := Reputation(db, faction)
		if rep < minRep {
			return ExecuteResult{
				Message: fmt.Sprintf(
					"%s: we don't do business with strangers. (need %s rep %d, have %d)",
					npc.Name, faction, minRep, rep,
				),
			}
		}
	}

	// Build inventory set
	invSet := make(map[string]int)
	for _, id := range inventoryIDs {
		invSet[id]++
	}

	// Check wanted items
	var missing []string
	for _, want := range trade.Wants {
		if invSet[want.ID] < want.Count {
			missing = append(missing, fmt.Sprintf("%s x%d", want.ID, want.Count))
		}
	}
	if len(missing) > 0 {
		return ExecuteResult{
			Message: fmt.Sprintf("you're missing: %s", strings.Join(missing, ", ")),
		}
	}

	// Remove wanted items from DB
	for _, want := range trade.Wants {
		for i := 0; i < want.Count; i++ {
			db.Exec(`DELETE FROM inventory WHERE item_id=?`, want.ID) //nolint:errcheck
		}
	}

	// Add offered items to DB
	for _, offer := range trade.Offers {
		name := offer.Name
		if name == "" {
			name = offer.ID
		}
		desc := offer.Desc
		if desc == "" {
			desc = fmt.Sprintf("received from %s", npc.Name)
		}
		db.Exec( //nolint:errcheck
			`INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?,?,?)`,
			offer.ID, name, desc,
		)
	}

	// Increment faction reputation
	if faction != "" {
		IncrementReputation(db, faction) //nolint:errcheck
	}

	var gaveIDs []string
	for _, want := range trade.Wants {
		gaveIDs = append(gaveIDs, want.ID)
	}

	return ExecuteResult{
		OK:         true,
		GaveItems:  gaveIDs,
		GotItems:   trade.Offers,
		FactionInc: faction,
		Message: fmt.Sprintf(
			"trade complete with %s. gave: %s.",
			npc.Name,
			strings.Join(gaveIDs, ", "),
		),
	}
}
