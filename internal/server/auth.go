package server

import (
	"fmt"
	"regexp"
)

var validPlayerID = regexp.MustCompile(`^[a-zA-Z0-9-]{2,20}$`)

// ValidatePlayerID checks that a player name is 2–20 alphanumeric+hyphen chars.
func ValidatePlayerID(id string) error {
	if !validPlayerID.MatchString(id) {
		return fmt.Errorf("playerID must be 2-20 alphanumeric characters or hyphens")
	}
	return nil
}

// ValidatePassphrase returns true if given matches expected, or if expected is
// empty (no auth required).
func ValidatePassphrase(given, expected string) bool {
	if expected == "" {
		return true
	}
	return given == expected
}
