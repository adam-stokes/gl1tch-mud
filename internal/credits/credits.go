// Package credits manages the player's credit wallet backed by SQLite.
package credits

import (
	"context"
	"database/sql"

	"github.com/adam-stokes/gl1tch-mud/internal/db/sqliteq"
)

// Get returns the current credit balance. Returns 0 on any error.
func Get(db *sql.DB) int {
	q := sqliteq.New(db)
	c, err := q.GetCredits(context.Background())
	if err != nil {
		return 0
	}
	return int(c)
}

// Add adds amount credits (may be negative) and returns the new balance.
func Add(db *sql.DB, amount int) (int, error) {
	q := sqliteq.New(db)
	err := q.UpsertCredits(context.Background(), int64(amount))
	if err != nil {
		return 0, err
	}
	return Get(db), nil
}

// Deduct subtracts amount from the balance. Returns an error if insufficient funds.
func Deduct(db *sql.DB, amount int) (int, error) {
	cur := Get(db)
	if cur < amount {
		return cur, &InsufficientFundsError{Have: cur, Need: amount}
	}
	return Add(db, -amount)
}

// InsufficientFundsError is returned when the player cannot afford a deduction.
type InsufficientFundsError struct {
	Have int
	Need int
}

func (e *InsufficientFundsError) Error() string {
	return "insufficient credits"
}
