// Package analytics emits structured gameplay events as JSON log lines so we
// can mine player behaviour from container stdout (Docker/Loki/etc.) without
// adding a database dependency.
package analytics

import (
	"log/slog"
	"os"
	"time"
)

var logger *slog.Logger

func init() {
	logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
}

// Event logs a gameplay event with structured fields.
func Event(eventType string, fields map[string]any) {
	args := []any{"event", eventType, "ts", time.Now().UnixMilli()}
	for k, v := range fields {
		args = append(args, k, v)
	}
	logger.Info("gameplay", args...)
}

// Command logs a command execution.
func Command(accountID, username, world, room, verb string, args []string, output string, success bool) {
	Event("command", map[string]any{
		"account": accountID,
		"user":    username,
		"world":   world,
		"room":    room,
		"verb":    verb,
		"args":    args,
		"success": success,
		"out_len": len(output),
	})
}

// NoResult logs when an action button produced no result (player frustration signal).
func NoResult(accountID, username, world, action, reason string) {
	Event("no_result", map[string]any{
		"account": accountID,
		"user":    username,
		"world":   world,
		"action":  action,
		"reason":  reason,
	})
}

// RoomEnter logs when a player enters a room.
func RoomEnter(accountID, username, world, roomID string) {
	Event("room_enter", map[string]any{
		"account": accountID,
		"user":    username,
		"world":   world,
		"room":    roomID,
	})
}

// RoomExit logs when a player leaves a room, including how long they spent there.
func RoomExit(accountID, username, world, roomID string, durationMs int64) {
	Event("room_exit", map[string]any{
		"account":     accountID,
		"user":        username,
		"world":       world,
		"room":        roomID,
		"duration_ms": durationMs,
	})
}

// Login logs a login event.
func Login(accountID, username, world string) {
	Event("login", map[string]any{
		"account": accountID,
		"user":    username,
		"world":   world,
	})
}

// Logout logs a session end.
func Logout(accountID, username, world string, durationSec int) {
	Event("logout", map[string]any{
		"account":      accountID,
		"user":         username,
		"world":        world,
		"duration_sec": durationSec,
	})
}

// WorldSwitch logs a live world switch.
func WorldSwitch(accountID, username, fromWorld, toWorld string) {
	Event("world_switch", map[string]any{
		"account": accountID,
		"user":    username,
		"from":    fromWorld,
		"to":      toWorld,
	})
}

// Error logs an error encountered during gameplay.
func Error(accountID, username, world, where string, err error) {
	if err == nil {
		return
	}
	Event("error", map[string]any{
		"account": accountID,
		"user":    username,
		"world":   world,
		"where":   where,
		"err":     err.Error(),
	})
}
