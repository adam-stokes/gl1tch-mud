package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// HashPassword hashes a plaintext password with bcrypt.
func HashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("auth: hash: %w", err)
	}
	return string(hash), nil
}

// CheckPassword verifies a password against a bcrypt hash.
func CheckPassword(password, hash string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// GenerateToken creates a cryptographically random 64-char hex token.
func GenerateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("auth: token: %w", err)
	}
	return hex.EncodeToString(b), nil
}

// ValidateNewAccount checks account creation parameters.
func ValidateNewAccount(username, password, role string) error {
	if username == "" {
		return fmt.Errorf("username required")
	}
	if len(password) < 3 {
		return fmt.Errorf("password must be at least 3 characters")
	}
	if role != "admin" && role != "player" {
		return fmt.Errorf("role must be 'admin' or 'player'")
	}
	return nil
}
