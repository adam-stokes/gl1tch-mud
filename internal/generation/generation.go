// Package generation implements Ollama-driven room generation for the explore command.
package generation

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
	"gopkg.in/yaml.v3"
)

const (
	defaultOllamaURL = "http://localhost:11434/api/chat"
	defaultTimeout   = 5 * time.Second
)

// Generator handles Ollama-based room generation with SQLite caching.
type Generator struct {
	db         *sql.DB
	ollamaURL  string
	httpClient *http.Client
}

// New creates a new Generator using the given DB and default Ollama URL.
func New(db *sql.DB) *Generator {
	return &Generator{
		db:        db,
		ollamaURL: defaultOllamaURL,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// NewWithURL creates a Generator with a custom Ollama URL (for testing).
func NewWithURL(db *sql.DB, url string) *Generator {
	return &Generator{
		db:        db,
		ollamaURL: url,
		httpClient: &http.Client{Timeout: defaultTimeout},
	}
}

// promptHash returns the SHA256 of the composite prompt key.
func promptHash(roomID, direction, worldName string) string {
	h := sha256.Sum256([]byte(roomID + "|" + direction + "|" + worldName))
	return fmt.Sprintf("%x", h)
}

// oppositeDirection returns the reverse of a compass direction.
func oppositeDirection(dir string) string {
	switch strings.ToLower(dir) {
	case "north":
		return "south"
	case "south":
		return "north"
	case "east":
		return "west"
	case "west":
		return "east"
	case "up":
		return "down"
	case "down":
		return "up"
	}
	return "back"
}

// cachedRoom looks up a generated room from the cache. Returns nil if not found.
func (g *Generator) cachedRoom(hash string) *world.Room {
	var blob string
	err := g.db.QueryRow(
		`SELECT yaml_blob FROM generated_content WHERE prompt_hash=? AND type='room'`, hash,
	).Scan(&blob)
	if err != nil {
		return nil
	}
	var r world.Room
	if err := yaml.Unmarshal([]byte(blob), &r); err != nil {
		return nil
	}
	return &r
}

// persistRoom stores a generated room in the cache.
func (g *Generator) persistRoom(hash string, r *world.Room) error {
	blob, err := yaml.Marshal(r)
	if err != nil {
		return err
	}
	_, err = g.db.Exec(
		`INSERT OR IGNORE INTO generated_content (prompt_hash, type, yaml_blob, created_at) VALUES (?,?,?,?)`,
		hash, "room", string(blob), time.Now().Unix(),
	)
	return err
}

// ollamaRequest is the JSON body sent to the Ollama API.
type ollamaRequest struct {
	Model    string          `json:"model"`
	Messages []ollamaMessage `json:"messages"`
	Stream   bool            `json:"stream"`
}

type ollamaMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ollamaResponse is the JSON response from Ollama (non-streaming).
type ollamaResponse struct {
	Message ollamaMessage `json:"message"`
}

func buildPrompt(currentRoom *world.Room, direction string, w *world.World) string {
	tagline := strings.TrimSpace(w.UI.Tagline)
	if tagline == "" {
		tagline = "atmospheric and immersive"
	}
	return fmt.Sprintf(`You are generating a room for a text MUD called "%s" (%s).
The player is in "%s": %s
They explore to the %s and find a new area.

Return ONLY a YAML block (no prose, no markdown fences) with this exact structure:
id: gen-placeholder
name: "Room Name Here"
desc: |
  A short 2-3 sentence description matching the world's tone.
exits:
  %s: %s
npcs: []
items: []

Keep it atmospheric, consistent with the world theme, and true to the tagline. Output ONLY the YAML.`,
		w.Name, tagline,
		currentRoom.Name, strings.TrimSpace(currentRoom.Desc),
		direction,
		oppositeDirection(direction), currentRoom.ID,
	)
}

// callOllama sends the generation prompt to Ollama and returns the raw content.
func (g *Generator) callOllama(ctx context.Context, model, prompt string) (string, error) {
	reqBody := ollamaRequest{
		Model: model,
		Messages: []ollamaMessage{
			{Role: "user", Content: prompt},
		},
		Stream: false,
	}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.ollamaURL, bytes.NewReader(data))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(body, &ollamaResp); err != nil {
		return "", fmt.Errorf("ollama: parse response: %w", err)
	}
	return ollamaResp.Message.Content, nil
}

// parseRoomYAML extracts a Room struct from potentially noisy model output.
// It strips any markdown code fences and tries to parse the remainder.
func parseRoomYAML(content string) (*world.Room, error) {
	// Strip markdown fences if present
	content = strings.TrimSpace(content)
	content = strings.TrimPrefix(content, "```yaml")
	content = strings.TrimPrefix(content, "```")
	content = strings.TrimSuffix(content, "```")
	content = strings.TrimSpace(content)

	var r world.Room
	if err := yaml.Unmarshal([]byte(content), &r); err != nil {
		return nil, fmt.Errorf("parse room yaml: %w", err)
	}
	if r.Name == "" {
		return nil, fmt.Errorf("generated room has no name")
	}
	return &r, nil
}

// GenerateResult is the outcome of a Generate call.
type GenerateResult struct {
	Room      *world.Room
	FromCache bool
	Model     string
	Error     error
	ErrMsg    string // user-facing message on failure
}

// providerModel splits "provider/model" into its parts.
// A bare model name (no slash) is assumed to be ollama.
type providerModel struct {
	provider string
	model    string
}

func parseProviderModel(s string) providerModel {
	if parts := strings.SplitN(s, "/", 2); len(parts) == 2 {
		return providerModel{provider: parts[0], model: parts[1]}
	}
	return providerModel{provider: "ollama", model: s}
}

// callGlitch delegates generation to the glitch CLI for non-Ollama providers.
func (g *Generator) callGlitch(ctx context.Context, pm providerModel, prompt string) (string, error) {
	args := []string{"ask", "--route=false", "--brain=false"}
	if pm.provider != "" {
		args = append(args, "--provider="+pm.provider)
	}
	if pm.model != "" {
		args = append(args, "--model="+pm.model)
	}
	args = append(args, prompt)
	out, err := exec.CommandContext(ctx, "glitch", args...).Output()
	if err != nil {
		return "", fmt.Errorf("glitch ask: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// Generate attempts to create or load a room for the given direction.
// currentRoom is the room the player is currently in.
// w is the live world; on success, the generated room is added to the world graph.
func (g *Generator) Generate(ctx context.Context, w *world.World, currentRoom *world.Room, direction string) GenerateResult {
	pm := parseProviderModel(resolveNarratorModel(w.NarratorModel))
	modelLabel := pm.provider + "/" + pm.model

	hash := promptHash(currentRoom.ID, direction, w.Name)

	// Cache hit
	if cached := g.cachedRoom(hash); cached != nil {
		g.wireRooms(w, currentRoom, cached, direction)
		return GenerateResult{Room: cached, FromCache: true, Model: modelLabel}
	}

	prompt := buildPrompt(currentRoom, direction, w)
	var content string
	var err error
	if pm.provider == "ollama" {
		content, err = g.callOllama(ctx, pm.model, prompt)
	} else {
		content, err = g.callGlitch(ctx, pm, prompt)
	}
	if err != nil {
		return GenerateResult{
			Error:  err,
			ErrMsg: "static beyond the edge — signal lost.",
		}
	}

	room, err := parseRoomYAML(content)
	if err != nil {
		return GenerateResult{
			Error:  err,
			ErrMsg: "something's there but you can't make it out.",
		}
	}

	// Assign deterministic ID based on hash
	room.ID = "gen-" + hash[:8]

	// Wire exits
	g.wireRooms(w, currentRoom, room, direction)

	// Persist to cache
	g.persistRoom(hash, room) //nolint:errcheck

	return GenerateResult{Room: room, FromCache: false, Model: modelLabel}
}

// wireRooms adds the new room to the world graph and links exits bidirectionally.
func (g *Generator) wireRooms(w *world.World, current, generated *world.Room, direction string) {
	// Don't add if already in the world
	if w.Room(generated.ID) != nil {
		return
	}

	// Ensure generated room has the return exit
	if generated.Exits == nil {
		generated.Exits = make(map[string]string)
	}
	generated.Exits[oppositeDirection(direction)] = current.ID

	// Add to world graph
	w.AddRoom(generated)

	// Wire current room → generated room
	if current.Exits == nil {
		current.Exits = make(map[string]string)
	}
	current.Exits[direction] = generated.ID
}

// resolveNarratorModel returns "provider/model" for generation.
// Priority: explicit world config > glitch model --local > hardcoded fallback.
func resolveNarratorModel(worldModel string) string {
	if worldModel != "" {
		// Normalise bare model names (no slash) to ollama/<model>.
		if !strings.Contains(worldModel, "/") {
			return "ollama/" + worldModel
		}
		return worldModel
	}
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "glitch", "model", "--local").Output()
	if err == nil {
		s := strings.TrimSpace(string(out))
		if strings.Contains(s, "/") {
			return s // already "provider/model"
		}
	}
	return "ollama/llama3.2"
}
