// Package credits manages the player's credit wallet.
package credits

import (
	"context"

	"github.com/adam-stokes/gl1tch-mud/internal/db/gamedb"
)

// Get returns the current credit balance. Returns 0 on any error.
func Get(gdb *gamedb.GameDB) int {
	return gdb.GetCredits(context.Background())
}

// Add adds amount credits (may be negative) and returns the new balance.
func Add(gdb *gamedb.GameDB, amount int) (int, error) {
	err := gdb.AddCredits(context.Background(), amount)
	if err != nil {
		return 0, err
	}
	return Get(gdb), nil
}

// Deduct subtracts amount from the balance. Returns an error if insufficient funds.
func Deduct(gdb *gamedb.GameDB, amount int) (int, error) {
	cur := Get(gdb)
	if cur < amount {
		return cur, &InsufficientFundsError{Have: cur, Need: amount}
	}
	return Add(gdb, -amount)
}

// InsufficientFundsError is returned when the player cannot afford a deduction.
type InsufficientFundsError struct {
	Have int
	Need int
}

func (e *InsufficientFundsError) Error() string {
	return "insufficient credits"
}
