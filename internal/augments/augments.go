package augments

import (
	"context"
	"fmt"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
)

const maxAugments = 3

func Install(gdb *gamedb.GameDB, skill string, bonus int) error {
	ctx := context.Background()
	count := gdb.AugmentCount(ctx)
	if count >= maxAugments {
		return fmt.Errorf("max augments reached")
	}
	return gdb.InstallAugment(ctx, skill, bonus)
}

func TotalBonus(gdb *gamedb.GameDB, skill string) int {
	return gdb.AugmentTotalBonus(context.Background(), skill)
}

func List(gdb *gamedb.GameDB) ([]struct {
	Skill string
	Bonus int
}, error) {
	ctx := context.Background()
	records, err := gdb.ListAugments(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]struct {
		Skill string
		Bonus int
	}, len(records))
	for i, r := range records {
		out[i].Skill = r.Skill
		out[i].Bonus = r.Bonus
	}
	return out, nil
}
