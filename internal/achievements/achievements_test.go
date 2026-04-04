package achievements_test

import (
	"os"
	"testing"

	"github.com/adam-stokes/gl1tch-mud/internal/achievements"
)

func TestLoad(t *testing.T) {
	yaml := `
source: gl1tch-mud
version: "1.0.0"
achievements:
  - id: first_blood
    name: "First Blood"
    description: "Win your first combat"
    trigger:
      action: combat.won
      count: 1
    xp: 50
`
	f, err := os.CreateTemp(t.TempDir(), "achievements*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f.WriteString(yaml) //nolint:errcheck
	f.Close()

	cf, err := achievements.Load(f.Name())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cf.Source != "gl1tch-mud" {
		t.Errorf("source = %q, want gl1tch-mud", cf.Source)
	}
	if len(cf.Achievements) != 1 {
		t.Fatalf("got %d achievements, want 1", len(cf.Achievements))
	}
	if cf.Achievements[0].ID != "first_blood" {
		t.Errorf("id = %q, want first_blood", cf.Achievements[0].ID)
	}
}

func TestLoad_Missing(t *testing.T) {
	_, err := achievements.Load("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}
