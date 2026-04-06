package classes

import "testing"

func TestAllClassesHaveUniqueIDs(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range All() {
		if seen[c.ID] {
			t.Errorf("duplicate class id: %s", c.ID)
		}
		seen[c.ID] = true
	}
	if len(All()) != 5 {
		t.Errorf("expected 5 classes, got %d", len(All()))
	}
}

func TestEveryClassHasFreeHookItem(t *testing.T) {
	for _, c := range All() {
		var has bool
		for _, k := range c.Kit {
			if k.Cost == 0 {
				has = true
				break
			}
		}
		if !has {
			t.Errorf("class %s has no free (cost=0) hook item", c.ID)
		}
	}
}

func TestKitFitsInBudget(t *testing.T) {
	// Greedy: pick cheapest items until the budget is exhausted. We should
	// always be able to fit at least 3 items including the free hook —
	// guards against accidentally over-pricing the kit.
	for _, c := range All() {
		items := append([]KitItem{}, c.Kit...)
		for i := 0; i < len(items); i++ {
			for j := i + 1; j < len(items); j++ {
				if items[j].Cost < items[i].Cost {
					items[i], items[j] = items[j], items[i]
				}
			}
		}
		spent := 0
		picks := 0
		for _, it := range items {
			if spent+it.Cost > KitBudget {
				continue
			}
			spent += it.Cost
			picks++
		}
		if picks < 3 {
			t.Errorf("class %s: only %d items fit in budget %d", c.ID, picks, KitBudget)
		}
	}
}

func TestByID(t *testing.T) {
	for _, c := range All() {
		got := ByID(c.ID)
		if got == nil || got.ID != c.ID {
			t.Errorf("ByID(%q) failed", c.ID)
		}
	}
	if ByID("nonexistent") != nil {
		t.Error("ByID for unknown id should return nil")
	}
}

func TestGhoulHasFactionRep(t *testing.T) {
	g := ByID("ghoul")
	if g == nil {
		t.Fatal("ghoul class missing")
	}
	if len(g.StartingRep) == 0 {
		t.Error("ghoul should have starting rep deltas")
	}
}

func TestEverySignatureVerbHasGatingSkill(t *testing.T) {
	for _, c := range All() {
		if c.SignatureVerb == "" {
			t.Errorf("class %s missing signature verb", c.ID)
		}
		if c.GatingSkill == "" {
			t.Errorf("class %s missing gating skill", c.ID)
		}
	}
}
