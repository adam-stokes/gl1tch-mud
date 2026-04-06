package mods

import (
	"context"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
)

func Apply(gdb *gamedb.GameDB, itemInstance, modID string) error {
	return gdb.ApplyItemMod(context.Background(), itemInstance, modID)
}

func GetMod(gdb *gamedb.GameDB, itemInstance string) (string, bool) {
	return gdb.GetItemMod(context.Background(), itemInstance)
}
