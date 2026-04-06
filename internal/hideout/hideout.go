// Package hideout manages the player's hideout upgrades.
package hideout

import (
	"context"
	"fmt"
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

// Upgrade is a single hideout upgrade definition.
type Upgrade struct {
	ID       string
	Name     string
	Desc     string
	Cost     int
	SkillReq int
}

// Catalog is the full list of available hideout upgrades.
var Catalog = []Upgrade{
	{ID: "workbench", Name: "Workbench", Desc: "Advanced crafting station. Unlocks tier-3 recipes.", Cost: 500, SkillReq: 0},
	{ID: "armory", Name: "Armory", Desc: "Weapon rack and maintenance tools.", Cost: 750, SkillReq: 0},
	{ID: "med-bay", Name: "Med Bay", Desc: "Restore full HP when you rest here.", Cost: 1000, SkillReq: 0},
	{ID: "signal-jammer", Name: "Signal Jammer", Desc: "Blocks tracking. Stealth resets to 80 on entry.", Cost: 1200, SkillReq: 2},
	{ID: "training-deck", Name: "Training Deck", Desc: "Practice runs. Earn 50 hacking XP per visit.", Cost: 1500, SkillReq: 0},
	{ID: "intel-hub", Name: "Intel Hub", Desc: "Reveals all active world events.", Cost: 2000, SkillReq: 3},
	{ID: "vault", Name: "Vault", Desc: "Secure storage. Expand inventory capacity.", Cost: 2500, SkillReq: 0},
}

// Installed returns the IDs of all installed upgrades.
func Installed(gdb *gamedb.GameDB) ([]string, error) {
	return gdb.ListHideoutUpgrades(context.Background())
}

// HasUpgrade reports whether a specific upgrade is installed.
func HasUpgrade(gdb *gamedb.GameDB, id string) (bool, error) {
	return gdb.HasHideoutUpgrade(context.Background(), id)
}

// Install records an upgrade as installed.
func Install(gdb *gamedb.GameDB, id string) error {
	return gdb.InstallHideoutUpgrade(context.Background(), id)
}

// Available returns all upgrades from the Catalog that are not yet installed.
func Available(gdb *gamedb.GameDB) ([]Upgrade, error) {
	installed, err := Installed(gdb)
	if err != nil {
		return nil, err
	}
	installedSet := make(map[string]bool, len(installed))
	for _, id := range installed {
		installedSet[id] = true
	}
	var result []Upgrade
	for _, u := range Catalog {
		if !installedSet[u.ID] {
			result = append(result, u)
		}
	}
	return result, nil
}

// FindUpgrade returns the upgrade definition for the given ID, or an error.
func FindUpgrade(id string) (*Upgrade, error) {
	for i := range Catalog {
		if Catalog[i].ID == id {
			return &Catalog[i], nil
		}
	}
	return nil, fmt.Errorf("unknown upgrade %q", id)
}

// GenerateHideout creates a new room representing the player's faction hideout.
func GenerateHideout(factionName string) world.Room {
	id := "hideout-" + strings.ToLower(strings.ReplaceAll(factionName, " ", "-"))
	return world.Room{
		ID:    id,
		Name:  factionName + " Safehouse",
		Desc:  "A signal-dead pocket of the net, scrubbed from every index. Bare infrastructure, no identifying markers. Yours.",
		Exits: map[string]string{},
		NPCs:  []world.NPC{},
		Items: []world.Item{},
	}
}
