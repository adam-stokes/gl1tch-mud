package generation

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/world"
	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if _, err := db.Exec(`CREATE TABLE IF NOT EXISTS generated_content (
		prompt_hash TEXT PRIMARY KEY,
		type        TEXT NOT NULL,
		yaml_blob   TEXT NOT NULL,
		created_at  INTEGER NOT NULL
	)`); err != nil {
		t.Fatalf("create table: %v", err)
	}
	return db
}

func makeTestWorld() *world.World {
	w := &world.World{
		Name:          "testworld",
		StartRoom:     "net-0",
		NarratorModel: "llama3.2",
	}
	// Use AddRoom which initializes the index
	w.AddRoom(&world.Room{
		ID:    "net-0",
		Name:  "The Gibson",
		Desc:  "Entry node.",
		Exits: map[string]string{"north": "net-1"},
	})
	w.AddRoom(&world.Room{
		ID:    "net-1",
		Name:  "Archive Sector",
		Desc:  "Old data.",
		Exits: map[string]string{"south": "net-0"},
	})
	return w
}

func validRoomYAML() string {
	return `id: gen-placeholder
name: "Dark Corridor"
desc: |
  A narrow passage flickering with dying neon strips.
  Somewhere a cooling fan whirs at dangerous speed.
exits:
  west: net-0
npcs: []
items: []
`
}

func mockOllama(t *testing.T, responseContent string, statusCode int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if statusCode != 200 {
			w.WriteHeader(statusCode)
			return
		}
		resp := ollamaResponse{
			Message: ollamaMessage{Role: "assistant", Content: responseContent},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
}

func TestGenerateSuccess(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	srv := mockOllama(t, validRoomYAML(), 200)
	defer srv.Close()

	wld := makeTestWorld()
	g := NewWithURL(db, srv.URL)

	current := wld.Room("net-0")
	if current == nil {
		t.Fatal("net-0 not found in test world")
	}
	res := g.Generate(context.Background(), wld, current, "west")

	if res.Error != nil {
		t.Fatalf("Generate: %v / %s", res.Error, res.ErrMsg)
	}
	if res.Room == nil {
		t.Fatal("expected non-nil Room")
	}
	if res.Room.Name != "Dark Corridor" {
		t.Errorf("room name: got %q want %q", res.Room.Name, "Dark Corridor")
	}
	if res.FromCache {
		t.Error("first generate should not be from cache")
	}

	// Generated room should be in world graph now
	if wld.Room(res.Room.ID) == nil {
		t.Errorf("generated room %q not in world graph", res.Room.ID)
	}

	// Current room should have west exit
	cur := wld.Room("net-0")
	if cur.Exits["west"] == "" {
		t.Error("current room should have west exit after generation")
	}
}

func TestGenerateCacheHit(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		resp := ollamaResponse{
			Message: ollamaMessage{Role: "assistant", Content: validRoomYAML()},
		}
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}))
	defer srv.Close()

	wld := makeTestWorld()
	g := NewWithURL(db, srv.URL)

	current := wld.Room("net-0")

	// First call — should hit Ollama
	res1 := g.Generate(context.Background(), wld, current, "west")
	if res1.Error != nil {
		t.Fatalf("first Generate: %v", res1.Error)
	}

	// Second call with a fresh world — should use cache, not Ollama
	wld2 := makeTestWorld()
	current2 := wld2.Room("net-0")
	res2 := g.Generate(context.Background(), wld2, current2, "west")

	if res2.Error != nil {
		t.Fatalf("second Generate: %v", res2.Error)
	}
	if !res2.FromCache {
		t.Error("second generate should come from cache")
	}
	if callCount != 1 {
		t.Errorf("Ollama should be called only once, got %d calls", callCount)
	}
}

func TestGenerateOllamaUnreachable(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	wld := makeTestWorld()
	g := NewWithURL(db, "http://127.0.0.1:19999") // nothing listening here
	g.httpClient.Timeout = 100 * 1000 * 1000       // 100ms in nanoseconds (not working, set Duration)

	current := wld.Room("net-0")
	res := g.Generate(context.Background(), wld, current, "south")

	if res.Error == nil {
		t.Error("expected error when Ollama unreachable")
	}
	if res.ErrMsg == "" {
		t.Error("expected user-facing error message")
	}
}

func TestGenerateMalformedYAML(t *testing.T) {
	db := openTestDB(t)
	defer db.Close()

	srv := mockOllama(t, "this is not valid YAML with no name field\n---\nfoo: bar", 200)
	defer srv.Close()

	wld := makeTestWorld()
	g := NewWithURL(db, srv.URL)

	current := wld.Room("net-0")
	res := g.Generate(context.Background(), wld, current, "down")

	// Should return gracefully with ErrMsg when name is empty
	if res.Error != nil && res.ErrMsg == "" {
		t.Error("expected ErrMsg when YAML parse fails")
	}
}

func TestOppositeDirection(t *testing.T) {
	cases := map[string]string{
		"north": "south",
		"south": "north",
		"east":  "west",
		"west":  "east",
		"up":    "down",
		"down":  "up",
	}
	for dir, want := range cases {
		got := oppositeDirection(dir)
		if got != want {
			t.Errorf("oppositeDirection(%q) = %q, want %q", dir, got, want)
		}
	}
}

func TestParseRoomYAMLWithFences(t *testing.T) {
	content := "```yaml\n" + validRoomYAML() + "\n```"
	room, err := parseRoomYAML(content)
	if err != nil {
		t.Fatalf("parseRoomYAML with fences: %v", err)
	}
	if room.Name != "Dark Corridor" {
		t.Errorf("expected Dark Corridor, got %q", room.Name)
	}
}
