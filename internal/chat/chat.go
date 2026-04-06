// Package chat implements say, shout, and whisper commands with proper message routing.
package chat

import (
	"strings"

	"github.com/adam-stokes/gl1tch-mud/internal/commands"
)

// Say broadcasts a message to all players in the same room.
func Say(sender string, args []string) commands.Result {
	text := strings.Join(args, " ")
	if text == "" {
		return commands.Result{Output: "say <message>"}
	}
	return commands.Result{
		ChatMessages: []commands.ChatMessage{{
			Type: "say", Sender: sender, Body: text,
		}},
	}
}

// Shout broadcasts a message to all players in the same world.
func Shout(sender string, args []string) commands.Result {
	text := strings.Join(args, " ")
	if text == "" {
		return commands.Result{Output: "shout <message>"}
	}
	return commands.Result{
		ChatMessages: []commands.ChatMessage{{
			Type: "shout", Sender: sender, Body: text,
		}},
	}
}

// Whisper sends a private message to a specific player.
func Whisper(sender string, args []string) commands.Result {
	if len(args) < 2 {
		return commands.Result{Output: "whisper <player> <message>"}
	}
	target := args[0]
	text := strings.Join(args[1:], " ")
	return commands.Result{
		Output: "[to " + target + "] " + text,
		ChatMessages: []commands.ChatMessage{{
			Type: "whisper", Sender: sender, Target: target, Body: text,
		}},
	}
}
