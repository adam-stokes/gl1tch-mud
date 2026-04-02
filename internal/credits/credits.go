// Package credits manages the player's credit wallet backed by SQLite.
package credits

import "database/sql"

// Get returns the current credit balance. Returns 0 on any error.
func Get(db *sql.DB) int {
	var c int
	db.QueryRow(`SELECT credits FROM player_credits WHERE id=1`).Scan(&c) //nolint:errcheck
	return c
}

// Add adds amount credits (may be negative) and returns the new balance.
func Add(db *sql.DB, amount int) (int, error) {
	_, err := db.Exec(
		`INSERT INTO player_credits (id, credits) VALUES (1, ?)
		 ON CONFLICT(id) DO UPDATE SET credits = credits + excluded.credits`,
		amount,
	)
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
