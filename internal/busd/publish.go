// Package busd provides a minimal client for publishing events to gl1tch's
// Unix socket event bus. If the socket is not available the publish is a no-op.
package busd

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

// Client holds an open connection to the gl1tch bus.
type Client struct {
	conn net.Conn
}

// Connect opens a connection to the gl1tch BUSD socket.
// Returns nil (no-op client) if the socket is not available.
func Connect() *Client {
	path := socketPath()
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return &Client{}
	}
	// Send registration frame.
	reg, _ := json.Marshal(map[string]any{
		"name":      "gl1tch-mud",
		"subscribe": []string{},
	})
	conn.Write(append(reg, '\n')) //nolint:errcheck
	return &Client{conn: conn}
}

// Publish sends an event to gl1tch. Silent no-op if not connected.
func (c *Client) Publish(topic string, payload any) {
	if c.conn == nil {
		return
	}
	frame, err := json.Marshal(map[string]any{
		"action":  "publish",
		"event":   topic,
		"payload": payload,
	})
	if err != nil {
		return
	}
	c.conn.SetWriteDeadline(time.Now().Add(100 * time.Millisecond)) //nolint:errcheck
	fmt.Fprintf(c.conn, "%s\n", frame)
}

// Close tears down the connection.
func (c *Client) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func socketPath() string {
	if xdg := os.Getenv("XDG_RUNTIME_DIR"); xdg != "" {
		return filepath.Join(xdg, "glitch", "bus.sock")
	}
	cache, _ := os.UserCacheDir()
	return filepath.Join(cache, "glitch", "bus.sock")
}
