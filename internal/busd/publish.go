// Package busd provides a minimal client for publishing and subscribing to
// events on gl1tch's Unix socket event bus. If the socket is not available
// all operations are silent no-ops.
package busd

import (
	"bufio"
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

// Connect opens a connection to the gl1tch BUSD socket (publish-only).
// Returns a no-op client if the socket is not available.
func Connect() *Client {
	return ConnectWithSubscriptions(nil)
}

// ConnectWithSubscriptions opens a connection and subscribes to the given
// topic patterns (glob-style, e.g. "mud.chat.reply"). Returns a no-op client
// if the socket is not available.
func ConnectWithSubscriptions(topics []string) *Client {
	if topics == nil {
		topics = []string{}
	}
	path := socketPath()
	conn, err := net.DialTimeout("unix", path, 500*time.Millisecond)
	if err != nil {
		return &Client{}
	}
	reg, _ := json.Marshal(map[string]any{
		"name":      "gl1tch-mud",
		"subscribe": topics,
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

// Event is an incoming event frame from the BUSD daemon.
type Event struct {
	Topic   string          `json:"event"`
	Payload json.RawMessage `json:"payload"`
}

// Listen reads incoming event frames and calls fn for each one.
// Blocks until the connection is closed. Run in a goroutine.
// No-op if not connected.
func (c *Client) Listen(fn func(ev Event)) {
	if c.conn == nil {
		return
	}
	scanner := bufio.NewScanner(c.conn)
	for scanner.Scan() {
		var ev Event
		if err := json.Unmarshal(scanner.Bytes(), &ev); err != nil {
			continue
		}
		if ev.Topic != "" {
			fn(ev)
		}
	}
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
