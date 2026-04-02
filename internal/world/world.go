package world

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

//go:embed defaults/cyberspace/world.yaml
var defaultWorldFS embed.FS

// NPC is an enemy or character in a room.
type NPC struct {
	ID     string `yaml:"id"`
	Name   string `yaml:"name"`
	HP     int    `yaml:"hp"`
	Attack int    `yaml:"attack"`
	Desc   string `yaml:"desc"`
}

// Item is a collectable object in a room.
type Item struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
	Desc string `yaml:"desc"`
}

// Room is a single location in the world.
type Room struct {
	ID    string            `yaml:"id"`
	Name  string            `yaml:"name"`
	Desc  string            `yaml:"desc"`
	Exits map[string]string `yaml:"exits"`
	NPCs  []NPC             `yaml:"npcs"`
	Items []Item            `yaml:"items"`
}

// World holds all rooms for a loaded world.
type World struct {
	Name          string `yaml:"name"`
	StartRoom     string `yaml:"start_room"`
	NarratorModel string `yaml:"narrator_model"`
	Rooms         []Room `yaml:"rooms"`
	index         map[string]*Room
}

// Load reads a world YAML from ~/.local/share/gl1tch-mud/worlds/<name>/world.yaml.
// Falls back to the embedded cyberspace world if not found.
func Load(name string) (*World, error) {
	path := worldPath(name)
	data, err := os.ReadFile(path)
	if err != nil {
		// fall back to embedded default
		data, err = defaultWorldFS.ReadFile("defaults/cyberspace/world.yaml")
		if err != nil {
			return nil, fmt.Errorf("world: load default: %w", err)
		}
	}
	var w World
	if err := yaml.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("world: parse %s: %w", name, err)
	}
	w.index = make(map[string]*Room, len(w.Rooms))
	for i := range w.Rooms {
		w.index[w.Rooms[i].ID] = &w.Rooms[i]
	}
	return &w, nil
}

// Room returns the room with the given ID, or nil.
func (w *World) Room(id string) *Room {
	return w.index[id]
}

// Render returns a formatted description of the room.
func (r *Room) Render(visitedBefore bool) string {
	var b strings.Builder
	b.WriteString("\n")
	b.WriteString("[ " + r.Name + " ]\n")
	b.WriteString(strings.TrimSpace(r.Desc) + "\n")

	if len(r.Exits) > 0 {
		dirs := make([]string, 0, len(r.Exits))
		for d := range r.Exits {
			dirs = append(dirs, d)
		}
		b.WriteString("\nexits: " + strings.Join(dirs, ", ") + "\n")
	}

	if len(r.NPCs) > 0 {
		for _, npc := range r.NPCs {
			b.WriteString("  ! " + npc.Name + " is here.\n")
		}
	}

	if len(r.Items) > 0 {
		for _, item := range r.Items {
			b.WriteString("  + " + item.Name + " is on the ground.\n")
		}
	}

	return b.String()
}

func worldPath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "share", "gl1tch-mud", "worlds", name, "world.yaml")
}
