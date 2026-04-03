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
}

// InvItem is a carried item as sent to the client.
type InvItem struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Desc string `json:"desc"`
	Tier string `json:"tier"`
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
