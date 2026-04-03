package server

import (
	"context"
	"encoding/json"
	"fmt"

	"nhooyr.io/websocket"
)

// ClientMsg is a message sent from the browser client to the server.
type ClientMsg struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

// ServerMsg is a message sent from the server to the browser client.
type ServerMsg struct {
	Type    string `json:"type"`
	Payload any    `json:"payload,omitempty"`
}

// Payload types for ServerMsg.

type AuthOKPayload struct {
	PlayerID string `json:"playerID"`
	Level    int    `json:"level"`
	Title    string `json:"title"`
	XP       int    `json:"xp"`
}

type AuthFailPayload struct {
	Reason string `json:"reason"`
}

type OutputTokenPayload struct {
	Token string `json:"token"`
}

type ErrorPayload struct {
	Message string `json:"message"`
}

// StateUpdatePayload is sent after output.done with structured player state.
type StateUpdatePayload struct {
	HP        int       `json:"hp"`
	MaxHP     int       `json:"maxHp"`
	RoomName  string    `json:"roomName"`
	Exits     []string  `json:"exits"`
	Inventory []InvItem `json:"inventory"`
	Credits   int       `json:"credits"`
	Recipes   []Recipe  `json:"recipes,omitempty"`
}

// RecipeIngredient is a single crafting ingredient as sent to the client.
type RecipeIngredient struct {
	ID    string `json:"id"`
	Count int    `json:"count"`
}

// Recipe is a crafting recipe as sent to the client.
type Recipe struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Ingredients []RecipeIngredient `json:"ingredients"`
	OutputID    string             `json:"outputId"`
	OutputName  string             `json:"outputName"`
	SkillReq    int                `json:"skillReq,omitempty"`
}

// InvItem is a carried item as sent to the client.
type InvItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`
	Tier string `json:"tier"`
}

// PlayersUpdatePayload is sent to all clients when the player roster changes.
type PlayersUpdatePayload struct {
	HostOnline bool         `json:"hostOnline"`
	Players    []PlayerInfo `json:"players"`
}

// PlayerInfo is a single connected player entry in PlayersUpdatePayload.
type PlayerInfo struct {
	Name     string `json:"name"`
	RoomName string `json:"roomName,omitempty"`
}

// ChatPayload is the payload of an incoming "chat" ClientMsg.
type ChatPayload struct {
	Text string `json:"text"`
}

// ChatMessagePayload is broadcast to all clients when someone chats.
type ChatMessagePayload struct {
	From string `json:"from"`
	Text string `json:"text"`
}

// AuthPayload is the payload of an incoming "auth" ClientMsg.
type AuthPayload struct {
	PlayerID   string `json:"playerID"`
	Passphrase string `json:"passphrase"`
}

// InputPayload is the payload of an incoming "input" ClientMsg.
type InputPayload struct {
	Text string `json:"text"`
}

// writeMsg marshals msg to JSON and sends it as a text WebSocket frame.
func writeMsg(ctx context.Context, conn *websocket.Conn, msg ServerMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("writeMsg: marshal: %w", err)
	}
	return conn.Write(ctx, websocket.MessageText, data)
}
