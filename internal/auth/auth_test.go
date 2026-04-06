package auth_test

import (
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/auth"
)

func TestHashAndVerify(t *testing.T) {
	hash, err := auth.HashPassword("hunter2")
	if err != nil {
		t.Fatal(err)
	}
	if !auth.CheckPassword("hunter2", hash) {
		t.Error("expected password to match")
	}
	if auth.CheckPassword("wrong", hash) {
		t.Error("expected wrong password to not match")
	}
}

func TestGenerateToken(t *testing.T) {
	tok, err := auth.GenerateToken()
	if err != nil {
		t.Fatal(err)
	}
	if len(tok) != 64 {
		t.Errorf("expected 64 char token, got %d", len(tok))
	}
}

func TestValidateNewAccount(t *testing.T) {
	tests := []struct {
		name     string
		username string
		password string
		role     string
		wantErr  bool
	}{
		{"valid player", "kai", "hunter2", "player", false},
		{"valid admin", "stokes", "pass123", "admin", false},
		{"empty username", "", "hunter2", "player", true},
		{"short password", "kai", "ab", "player", true},
		{"invalid role", "kai", "hunter2", "superadmin", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := auth.ValidateNewAccount(tt.username, tt.password, tt.role)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateNewAccount() err = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
