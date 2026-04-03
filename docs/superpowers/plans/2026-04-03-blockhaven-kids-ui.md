# Block Haven Kids UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a `ui_profile: kids` mode to Block Haven that shows context-sensitive action buttons with inline target pickers, a room presence panel, clickable exits, collapsed text input, onboarding hints, and a visual quest tracker — all isolated to Block Haven via a world YAML flag.

**Architecture:** A `profile` field on `WorldUI` is read by the server and sent in `world_meta`. The frontend sets `data-ui="kids"` on `<body>` and switches to kid-friendly rendering. `state.update` is expanded with structured room NPCs/items/resources and active quests so the frontend never needs to parse text output.

**Tech Stack:** Go (backend structs, WebSocket protocol), TypeScript (Astro frontend), YAML (world config), CSS (scoped to `[data-ui="kids"]`), Vitest (TS tests), Go testing (backend tests)

---

### Task 1: Add `profile` field to `WorldUI`

**Files:**
- Modify: `internal/world/world.go` (WorldUI struct)
- Test: `internal/server/server_test.go`

- [ ] **Step 1: Write the failing test**

Add to `internal/server/server_test.go` (also add `"gopkg.in/yaml.v3"` to the import block):

```go
func TestWorldUIProfileParsedFromYAML(t *testing.T) {
	raw := []byte(`
name: testworld
start_room: r1
rooms: []
ui:
  profile: kids
  prompt: "$"
  tagline: "test"
  theme:
    bg: "#000"
`)
	var w world.World
	if err := yaml.Unmarshal(raw, &w); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if w.UI.Profile != "kids" {
		t.Errorf("Profile: got %q want %q", w.UI.Profile, "kids")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/server/ -run TestWorldUIProfileParsedFromYAML -v
```

Expected: FAIL — `w.UI.Profile` is empty because the field doesn't exist yet.

- [ ] **Step 3: Add `Profile` to `WorldUI` in `world.go`**

In `internal/world/world.go`, change:

```go
type WorldUI struct {
	Banner  string     `yaml:"banner"`
	Prompt  string     `yaml:"prompt"`
	Tagline string     `yaml:"tagline"`
	Theme   WorldTheme `yaml:"theme"`
}
```

to:

```go
type WorldUI struct {
	Profile string     `yaml:"profile,omitempty"`
	Banner  string     `yaml:"banner"`
	Prompt  string     `yaml:"prompt"`
	Tagline string     `yaml:"tagline"`
	Theme   WorldTheme `yaml:"theme"`
}
```

- [ ] **Step 4: Run test to verify it passes**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/server/ -run TestWorldUIProfileParsedFromYAML -v
```

Expected: PASS

- [ ] **Step 5: Verify all existing Go tests still pass**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./...
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add internal/world/world.go internal/server/server_test.go && git commit -m "feat(world): add profile field to WorldUI for ui mode selection"
```

---

### Task 2: Add room presence + quest types to protocol

**Files:**
- Modify: `internal/server/protocol.go`
- Test: `internal/server/server_test.go`

- [ ] **Step 1: Write the failing tests**

Add to `internal/server/server_test.go`:

```go
func TestStateUpdatePayloadRoomFieldsJSON(t *testing.T) {
	p := StateUpdatePayload{
		HP:       10,
		MaxHP:    10,
		RoomName: "Town Square",
		Exits:    []string{"north"},
		RoomNPCs: []RoomNPCInfo{
			{ID: "elder-mason", Name: "Elder Mason", CanTalk: true, CanTrade: false, Attackable: false},
		},
		RoomResources: []RoomResourceInfo{
			{ID: "limestone-vein", Name: "limestone-vein", Action: "mine"},
		},
		Quests: []QuestInfo{
			{ID: "q1", Title: "Find the Map", ObjCount: 1, ObjProgress: 0},
		},
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := out["room_npcs"]; !ok {
		t.Error("expected room_npcs key in JSON")
	}
	if _, ok := out["room_resources"]; !ok {
		t.Error("expected room_resources key in JSON")
	}
	if _, ok := out["quests"]; !ok {
		t.Error("expected quests key in JSON")
	}
}

func TestWorldMetaPayloadUIProfileJSON(t *testing.T) {
	p := WorldMetaPayload{
		Name:      "blockhaven",
		Tagline:   "the ruins remember everything.",
		UIProfile: "kids",
	}
	data, err := json.Marshal(p)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out["ui_profile"] != "kids" {
		t.Errorf("ui_profile: got %v want %q", out["ui_profile"], "kids")
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/server/ -run "TestStateUpdatePayload|TestWorldMetaPayload" -v
```

Expected: FAIL — types `RoomNPCInfo`, `RoomResourceInfo`, `QuestInfo` don't exist yet.

- [ ] **Step 3: Add new types and expand payloads in `protocol.go`**

In `internal/server/protocol.go`, after the `InvItem` type, add:

```go
// RoomNPCInfo describes an NPC present in the player's current room.
type RoomNPCInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	CanTalk    bool   `json:"can_talk"`
	CanTrade   bool   `json:"can_trade"`
	Attackable bool   `json:"attackable"`
}

// RoomItemInfo describes an item on the ground in the player's current room.
type RoomItemInfo struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Takeable bool   `json:"takeable"`
}

// RoomResourceInfo describes a minable/harvestable resource in the player's current room.
type RoomResourceInfo struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Action string `json:"action"`
}

// QuestInfo is a summary of an active quest sent in state.update.
type QuestInfo struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	ObjCount    int    `json:"obj_count"`
	ObjProgress int    `json:"obj_progress"`
}
```

Replace `StateUpdatePayload`:

```go
// StateUpdatePayload is sent after output.done with structured player state.
type StateUpdatePayload struct {
	HP            int                `json:"hp"`
	MaxHP         int                `json:"maxHp"`
	RoomName      string             `json:"roomName"`
	Exits         []string           `json:"exits"`
	Inventory     []InvItem          `json:"inventory"`
	Credits       int                `json:"credits"`
	Recipes       []Recipe           `json:"recipes,omitempty"`
	RoomNPCs      []RoomNPCInfo      `json:"room_npcs,omitempty"`
	RoomItems     []RoomItemInfo     `json:"room_items,omitempty"`
	RoomResources []RoomResourceInfo `json:"room_resources,omitempty"`
	Quests        []QuestInfo        `json:"quests,omitempty"`
}
```

Replace `WorldMetaPayload`:

```go
// WorldMetaPayload is sent to the client on WebSocket connect, before the first state.update.
type WorldMetaPayload struct {
	Name      string           `json:"name"`
	Tagline   string           `json:"tagline"`
	Theme     world.WorldTheme `json:"theme"`
	UIProfile string           `json:"ui_profile,omitempty"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/server/ -run "TestStateUpdatePayload|TestWorldMetaPayload" -v
```

Expected: both PASS

- [ ] **Step 5: Verify all tests still pass**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./...
```

Expected: all PASS

- [ ] **Step 6: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add internal/server/protocol.go internal/server/server_test.go && git commit -m "feat(protocol): add room presence types and quest info to state.update payload"
```

---

### Task 3: Populate room presence + quests in sendStateUpdate; send UIProfile in world_meta

**Files:**
- Modify: `internal/server/session.go`
- Modify: `internal/server/server.go`

- [ ] **Step 1: Add `quests` import to `session.go`**

In `internal/server/session.go`, add to the import block:

```go
"github.com/adam-stokes/gl1tch-mud/internal/quests"
```

- [ ] **Step 2: Replace the room lookup block in `sendStateUpdate`**

In `internal/server/session.go`, in `sendStateUpdate`, replace:

```go
	// Room name and exits.
	var roomName string
	var exits []string
	if room := s.world.Room(s.state.RoomID); room != nil {
		roomName = room.Name
		for dir := range room.Exits {
			exits = append(exits, dir)
		}
	}
```

with:

```go
	// Room name, exits, and kids-mode presence data.
	var roomName string
	var exits []string
	var roomNPCs []RoomNPCInfo
	var roomItems []RoomItemInfo
	var roomResources []RoomResourceInfo
	if room := s.world.Room(s.state.RoomID); room != nil {
		roomName = room.Name
		for dir := range room.Exits {
			exits = append(exits, dir)
		}
		for _, npc := range room.NPCs {
			roomNPCs = append(roomNPCs, RoomNPCInfo{
				ID:         npc.ID,
				Name:       npc.Name,
				CanTalk:    len(npc.Dialogue) > 0,
				CanTrade:   len(npc.Trades) > 0,
				Attackable: npc.Attack > 0,
			})
		}
		for _, item := range room.Items {
			roomItems = append(roomItems, RoomItemInfo{
				ID:       item.ID,
				Name:     item.Name,
				Takeable: true,
			})
		}
		for _, res := range room.Resources {
			roomResources = append(roomResources, RoomResourceInfo{
				ID:     res.ID,
				Name:   res.ID,
				Action: res.Type,
			})
		}
	}

	// Active quests for kids quest tracker.
	activeQuests, _ := quests.Active(s.database)
	questInfos := make([]QuestInfo, 0, len(activeQuests))
	for _, q := range activeQuests {
		questInfos = append(questInfos, QuestInfo{
			ID:          q.ID,
			Title:       q.Title,
			ObjCount:    q.ObjCount,
			ObjProgress: q.ObjProgress,
		})
	}
```

- [ ] **Step 3: Add new fields to the `payload` struct literal**

Find `payload := StateUpdatePayload{...}` and replace it:

```go
	payload := StateUpdatePayload{
		HP:            s.state.HP,
		MaxHP:         s.state.MaxHP,
		RoomName:      roomName,
		Exits:         exits,
		Inventory:     hudInv,
		Credits:       credits.Get(s.database),
		Recipes:       recipes,
		RoomNPCs:      roomNPCs,
		RoomItems:     roomItems,
		RoomResources: roomResources,
		Quests:        questInfos,
	}
```

- [ ] **Step 4: Add `UIProfile` to the `world_meta` send in `server.go`**

In `internal/server/server.go`, find:

```go
	_ = writeMsg(ctx, conn, ServerMsg{
		Type: "world_meta",
		Payload: WorldMetaPayload{
			Name:    selectedWorld.Name,
			Tagline: selectedWorld.UI.Tagline,
			Theme:   selectedWorld.UI.Theme,
		},
	})
```

Replace with:

```go
	_ = writeMsg(ctx, conn, ServerMsg{
		Type: "world_meta",
		Payload: WorldMetaPayload{
			Name:      selectedWorld.Name,
			Tagline:   selectedWorld.UI.Tagline,
			Theme:     selectedWorld.UI.Theme,
			UIProfile: selectedWorld.UI.Profile,
		},
	})
```

- [ ] **Step 5: Build and test**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go build ./... && go test ./...
```

Expected: clean build, all PASS

- [ ] **Step 6: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add internal/server/session.go internal/server/server.go && git commit -m "feat(server): populate room presence and quests in state.update; send ui_profile in world_meta"
```

---

### Task 4: Add `profile: kids` to both Block Haven world.yaml files

**Files:**
- Modify: `worlds/blockhaven/world.yaml`
- Modify: `internal/world/defaults/blockhaven/world.yaml`

- [ ] **Step 1: Edit `worlds/blockhaven/world.yaml`**

Find the `ui:` section. After `prompt: "$"`, add `profile: kids`:

```yaml
  prompt: "$"
  profile: kids
  tagline: "the ruins remember everything."
```

- [ ] **Step 2: Apply the same edit to the embedded default**

In `internal/world/defaults/blockhaven/world.yaml`, make the same change — add `profile: kids` after `prompt: "$"` and before `tagline:`.

- [ ] **Step 3: Verify the world still loads cleanly**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./internal/world/... -v
```

Expected: PASS

- [ ] **Step 4: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add worlds/blockhaven/world.yaml internal/world/defaults/blockhaven/world.yaml && git commit -m "feat(blockhaven): set ui_profile=kids for age-appropriate web UI"
```

---

### Task 5: Add kids-mode HTML to `game.astro`

**Files:**
- Modify: `web/src/pages/game.astro`

- [ ] **Step 1: Add `#room-context` panel to sidebar**

Inside `<div id="sidebar">`, after the closing `</div>` of the compass panel, add:

```html
        <!-- Room Context (kids mode only) -->
        <div class="panel" id="room-context">
          <div class="panel-title">who's here</div>
          <div id="room-npcs-list"></div>
          <div class="panel-title" style="margin-top:0.5rem">exits</div>
          <div id="kids-exits"></div>
        </div>
```

- [ ] **Step 2: Add `#target-picker` inside the Actions panel**

Find `<!-- Actions -->` panel. Replace:

```html
        <!-- Actions -->
        <div class="panel">
          <div class="panel-title">actions</div>
          <div class="action-grid" id="action-grid">
            <!-- populated by JS based on world -->
          </div>
        </div>
```

with:

```html
        <!-- Actions -->
        <div class="panel">
          <div class="panel-title">actions</div>
          <div id="target-picker">
            <div class="picker-label" id="target-picker-label">choose a target</div>
            <div id="target-picker-btns"></div>
          </div>
          <div class="action-grid" id="action-grid">
            <!-- populated by JS based on world -->
          </div>
        </div>
```

- [ ] **Step 3: Add `#kids-input-toggle` inside `#input-bar`**

Find `<div id="input-bar">`. Add a toggle button as the first child, before `<span class="prompt">`:

```html
      <div id="input-bar">
        <button id="kids-input-toggle" title="Show/hide keyboard input">⌨️</button>
        <span class="prompt">&gt;</span>
        <input id="cmd-input" ...
```

- [ ] **Step 4: Add `#hint-banner` before `#input-bar`**

Insert just before `<!-- Input bar -->`:

```html
      <!-- Onboarding hint banner (kids mode only) -->
      <div id="hint-banner"></div>
```

- [ ] **Step 5: Add `#quest-kids-modal` after the craft modal closing tag**

```html
    <!-- Kids quest modal -->
    <div class="modal-overlay" id="quest-kids-modal">
      <div class="modal-box" style="max-width:340px;width:90%">
        <button class="modal-close" id="quest-kids-modal-close">✕</button>
        <div class="modal-title">your quests</div>
        <div id="quest-kids-list"></div>
      </div>
    </div>
```

- [ ] **Step 6: Build to verify no HTML errors**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun run build 2>&1 | tail -20
```

Expected: clean build

- [ ] **Step 7: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/src/pages/game.astro && git commit -m "feat(web): add kids-mode HTML elements to game layout"
```

---

### Task 6: Add kids-mode CSS to `game.astro`

**Files:**
- Modify: `web/src/pages/game.astro`

- [ ] **Step 1: Append kids-mode CSS to the `<style is:global>` block**

Add all of the following before the closing `</style>` tag in `game.astro`:

```css
      /* ── Kids mode (data-ui="kids") ──────────────────────── */

      /* Hide compass + network; show room-context */
      #room-context                           { display: none; }
      [data-ui="kids"] #room-context          { display: block; }
      [data-ui="kids"] .panel:has(#btn-n)     { display: none; }
      [data-ui="kids"] .panel:has(#player-list) { display: none; }

      /* NPC list */
      [data-ui="kids"] #room-npcs-list {
        display: flex;
        flex-direction: column;
        gap: 4px;
        margin-bottom: 4px;
      }

      .room-npc-row {
        display: flex;
        align-items: center;
        gap: 6px;
        font-size: 0.78rem;
        color: var(--fg);
      }

      .room-npc-badges { display: flex; gap: 3px; font-size: 0.8rem; }

      /* Exit buttons */
      [data-ui="kids"] #kids-exits {
        display: flex;
        flex-wrap: wrap;
        gap: 4px;
      }

      .kids-exit-btn {
        background: var(--bg-dark);
        border: 1px solid var(--border);
        color: var(--fg);
        font-family: inherit;
        font-size: 0.72rem;
        padding: 3px 8px;
        border-radius: 4px;
        cursor: pointer;
        transition: border-color 0.1s, color 0.1s;
      }

      .kids-exit-btn:hover {
        border-color: var(--accent, #c9a84c);
        color: var(--accent, #c9a84c);
      }

      /* Kids inventory: chip row */
      [data-ui="kids"] .inv-grid {
        display: flex;
        flex-wrap: wrap;
        gap: 4px;
      }

      [data-ui="kids"] .inv-slot {
        aspect-ratio: unset;
        width: auto;
        height: auto;
        flex-direction: row;
        padding: 3px 8px;
        gap: 4px;
        border-radius: 12px;
      }

      [data-ui="kids"] .inv-slot .slot-icon  { font-size: 0.9rem; margin-bottom: 0; }
      [data-ui="kids"] .inv-slot .slot-label { font-size: 0.7rem; white-space: nowrap; text-overflow: unset; }

      /* Target picker */
      #target-picker        { display: none; margin-bottom: 6px; }
      #target-picker.open   { display: block; }

      .picker-label {
        font-size: 0.65rem;
        color: var(--comment);
        text-transform: uppercase;
        letter-spacing: 0.1em;
        margin-bottom: 5px;
      }

      #target-picker-btns { display: flex; flex-wrap: wrap; gap: 4px; }

      .target-btn {
        background: var(--bg);
        border: 1px solid var(--border);
        color: var(--fg);
        font-family: inherit;
        font-size: 0.75rem;
        padding: 4px 10px;
        border-radius: 4px;
        cursor: pointer;
        transition: border-color 0.1s, color 0.1s;
      }

      .target-btn:hover {
        border-color: var(--accent, #c9a84c);
        color: var(--accent, #c9a84c);
      }

      /* Kids action buttons: slightly larger */
      [data-ui="kids"] .action-btn {
        font-size: 0.8rem;
        padding: 0.45rem 0.5rem;
      }

      /* Hint banner */
      #hint-banner {
        display: none;
        background: var(--bg-dark);
        border-top: 1px solid var(--border);
        padding: 0.35rem 0.75rem;
        font-size: 0.75rem;
        color: var(--fg);
        align-items: center;
        justify-content: space-between;
        gap: 0.5rem;
        grid-column: 1 / -1;
      }

      #hint-banner.visible { display: flex; }

      .hint-close {
        background: none;
        border: none;
        color: var(--comment);
        font-size: 0.8rem;
        cursor: pointer;
        padding: 0;
        flex-shrink: 0;
      }

      /* Kids input toggle */
      #kids-input-toggle {
        display: none;
        background: none;
        border: 1px solid var(--border);
        color: var(--comment);
        font-size: 1rem;
        padding: 2px 6px;
        border-radius: 4px;
        cursor: pointer;
        flex-shrink: 0;
        transition: border-color 0.1s, color 0.1s;
        align-items: center;
      }

      [data-ui="kids"] #kids-input-toggle              { display: flex; }
      [data-ui="kids"] #cmd-input                      { display: none; }
      [data-ui="kids"] #send-btn                       { display: none; }
      [data-ui="kids"] .prompt                         { display: none; }
      [data-ui="kids"] #cmd-input.kids-visible         { display: block; }
      [data-ui="kids"] #send-btn.kids-visible          { display: block; }
      [data-ui="kids"] .prompt.kids-visible            { display: block; }

      /* Quest kids modal */
      #quest-kids-list {
        display: flex;
        flex-direction: column;
        gap: 10px;
        margin-top: 0.75rem;
        max-height: 60vh;
        overflow-y: auto;
      }

      .quest-kids-card {
        background: var(--bg);
        border: 1px solid var(--border);
        border-radius: 6px;
        padding: 8px 10px;
      }

      .quest-kids-title {
        font-size: 0.8rem;
        color: var(--fg);
        margin-bottom: 5px;
      }

      .quest-progress-bar-wrap {
        background: var(--border);
        border-radius: 4px;
        height: 6px;
        overflow: hidden;
        margin-bottom: 4px;
      }

      .quest-progress-bar-fill {
        height: 100%;
        background: var(--accent, #c9a84c);
        border-radius: 4px;
        transition: width 0.3s;
      }

      .quest-progress-label {
        font-size: 0.65rem;
        color: var(--comment);
      }
```

- [ ] **Step 2: Build to verify clean**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun run build 2>&1 | tail -20
```

Expected: clean build

- [ ] **Step 3: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/src/pages/game.astro && git commit -m "feat(web): add kids-mode CSS scoped to [data-ui=kids]"
```

---

### Task 7: Update `theme.ts` and `mud.ts` interfaces; add `_kidsMode`, `applyKidsMode()`

**Files:**
- Modify: `web/src/lib/theme.ts`
- Modify: `web/src/lib/mud.ts`

- [ ] **Step 1: Add `ui_profile` to `WorldMeta` in `theme.ts`**

Change:

```typescript
export interface WorldMeta {
  name: string;
  tagline: string;
  theme: WorldTheme;
}
```

to:

```typescript
export interface WorldMeta {
  name: string;
  tagline: string;
  theme: WorldTheme;
  ui_profile?: string;
}
```

- [ ] **Step 2: Add new interfaces to `mud.ts`**

After the existing `interface Recipe` block, add:

```typescript
interface RoomNPCInfo {
  id: string;
  name: string;
  can_talk: boolean;
  can_trade: boolean;
  attackable: boolean;
}

interface RoomItemInfo {
  id: string;
  name: string;
  takeable: boolean;
}

interface RoomResourceInfo {
  id: string;
  name: string;
  action: string;
}

interface QuestInfo {
  id: string;
  title: string;
  obj_count: number;
  obj_progress: number;
}
```

- [ ] **Step 3: Expand `StateUpdate` interface**

Replace:

```typescript
interface StateUpdate {
  hp: number;
  maxHp: number;
  roomName: string;
  exits: string[];
  inventory: InvItem[];
  credits: number;
  recipes?: Recipe[];
}
```

with:

```typescript
interface StateUpdate {
  hp: number;
  maxHp: number;
  roomName: string;
  exits: string[];
  inventory: InvItem[];
  credits: number;
  recipes?: Recipe[];
  room_npcs?: RoomNPCInfo[];
  room_items?: RoomItemInfo[];
  room_resources?: RoomResourceInfo[];
  quests?: QuestInfo[];
}
```

- [ ] **Step 4: Add module-level kids-mode state**

After `let _worldName = 'cyberspace';`, add:

```typescript
let _kidsMode = false;
let _lastState: StateUpdate | null = null;
```

- [ ] **Step 5: Add `applyKidsMode()` and `formatResourceName()` helper**

After the `updateCompass` function, add:

```typescript
// ── Kids mode ────────────────────────────────────────────────────────────────

function applyKidsMode(): void {
  _kidsMode = true;
  document.body.dataset.ui = 'kids';
}

function formatResourceName(id: string): string {
  return id.replace(/[-_]/g, ' ').replace(/\b\w/g, c => c.toUpperCase());
}
```

- [ ] **Step 6: Call `applyKidsMode()` in the `world_meta` handler**

In `handleServerMsg`, `world_meta` case, add the kids check:

```typescript
      case 'world_meta': {
        const meta = msg.payload as import('./theme').WorldMeta;
        import('./theme').then(({ applyTheme }) => {
          applyTheme(meta);
        });
        if (meta.ui_profile === 'kids') {
          applyKidsMode();
        }
        rebuildActionButtons(meta.name);
        break;
      }
```

- [ ] **Step 7: Build to verify no TypeScript errors**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun run build 2>&1 | tail -20
```

Expected: clean build

- [ ] **Step 8: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/src/lib/theme.ts web/src/lib/mud.ts && git commit -m "feat(web): add kids mode TypeScript interfaces and applyKidsMode() activation"
```

---

### Task 8: Room context panel + kids inventory chips

**Files:**
- Modify: `web/src/lib/mud.ts`

- [ ] **Step 1: Add `rebuildRoomContext()` after `applyKidsMode()`**

```typescript
function rebuildRoomContext(state: StateUpdate): void {
  const npcList = document.getElementById('room-npcs-list');
  if (npcList) {
    while (npcList.firstChild) npcList.removeChild(npcList.firstChild);
    const npcs = state.room_npcs ?? [];
    if (npcs.length === 0) {
      const empty = document.createElement('div');
      empty.style.cssText = 'font-size:0.7rem;color:var(--comment)';
      empty.textContent = 'Nobody here.';
      npcList.appendChild(empty);
    } else {
      for (const npc of npcs) {
        const row = document.createElement('div');
        row.className = 'room-npc-row';

        const nameEl = document.createElement('span');
        nameEl.textContent = npc.name;

        const badges = document.createElement('span');
        badges.className = 'room-npc-badges';
        if (npc.can_talk) {
          const b = document.createElement('span');
          b.className = 'room-npc-badge';
          b.title = 'Can talk';
          b.textContent = '💬';
          badges.appendChild(b);
        }
        if (npc.can_trade) {
          const b = document.createElement('span');
          b.className = 'room-npc-badge';
          b.title = 'Can trade';
          b.textContent = '🛒';
          badges.appendChild(b);
        }
        if (npc.attackable) {
          const b = document.createElement('span');
          b.className = 'room-npc-badge';
          b.title = 'Hostile';
          b.textContent = '⚔️';
          badges.appendChild(b);
        }
        row.appendChild(nameEl);
        row.appendChild(badges);
        npcList.appendChild(row);
      }
    }
  }

  const exitsEl = document.getElementById('kids-exits');
  if (exitsEl) {
    while (exitsEl.firstChild) exitsEl.removeChild(exitsEl.firstChild);
    const DIR_ARROW: Record<string, string> = {
      north: '↑ North', south: '↓ South', east: '→ East', west: '← West',
      up: '▲ Up', down: '▼ Down',
    };
    for (const exit of (state.exits ?? [])) {
      const btn = document.createElement('button');
      btn.className = 'kids-exit-btn';
      btn.textContent = DIR_ARROW[exit.toLowerCase()] ?? exit;
      const captured = exit;
      btn.addEventListener('click', () => {
        if (inputEnabled) sendCommand(captured);
      });
      exitsEl.appendChild(btn);
    }
  }
}
```

- [ ] **Step 2: Add a temporary stub for `rebuildKidsActionButtons`**

After `rebuildRoomContext`, add:

```typescript
// stub — replaced in Task 9
function rebuildKidsActionButtons(_state: StateUpdate): void {}
```

- [ ] **Step 3: Update `applyStateUpdate()` to store state and call kids functions**

Replace the existing `applyStateUpdate` function:

```typescript
  function applyStateUpdate(state: StateUpdate) {
    _lastState = state;
    roomEl.textContent    = state.roomName || '—';
    hpHearts.innerHTML    = renderHearts(state.hp, state.maxHp);
    hpText.textContent    = `${state.hp}/${state.maxHp}`;
    creditsEl.textContent = `¢ ${state.credits}`;
    if (state.recipes) _recipes = state.recipes;

    if (_kidsMode) {
      rebuildRoomContext(state);
      rebuildKidsActionButtons(state);
    } else {
      updateCompass(state.exits ?? []);
    }
    renderInventory(state.inventory ?? [], (item) => openItemModal(item, sendCommand));
  }
```

- [ ] **Step 4: Build to verify clean**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun run build 2>&1 | tail -20
```

Expected: clean build

- [ ] **Step 5: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/src/lib/mud.ts && git commit -m "feat(web): add kids room context panel rendering and state caching"
```

---

### Task 9: Kids action buttons + target picker

**Files:**
- Modify: `web/src/lib/mud.ts`

- [ ] **Step 1: Replace the `rebuildKidsActionButtons` stub with the real implementation**

Remove `function rebuildKidsActionButtons(_state: StateUpdate): void {}` and add:

```typescript
interface KidsActionDef {
  kidsAction: string;
  icon: string;
  label: string;
  cmd?: string;
  special?: string;
}

function rebuildKidsActionButtons(state: StateUpdate): void {
  const grid = document.getElementById('action-grid');
  if (!grid) return;
  while (grid.firstChild) grid.removeChild(grid.firstChild);

  const npcs      = state.room_npcs      ?? [];
  const resources = state.room_resources ?? [];

  const defs: KidsActionDef[] = [
    { kidsAction: 'look',   icon: '👁',  label: 'Look',   cmd: 'look' },
    ...(npcs.some(n => n.can_talk)   ? [{ kidsAction: 'talk',   icon: '💬', label: 'Talk' }]   : []),
    ...(npcs.some(n => n.attackable) ? [{ kidsAction: 'attack', icon: '⚔️', label: 'Attack' }] : []),
    ...(npcs.some(n => n.can_trade)  ? [{ kidsAction: 'trade',  icon: '🛒', label: 'Trade' }]  : []),
    ...(resources.length > 0         ? [{ kidsAction: 'forage', icon: '🌿', label: 'Forage' }] : []),
    { kidsAction: 'search', icon: '🔍', label: 'Search',  cmd: 'search' },
    { kidsAction: 'skills', icon: '⚡',  label: 'Skills',  cmd: 'skills' },
    { kidsAction: 'quests', icon: '📋', label: 'Quests',  special: 'quests-modal' },
    { kidsAction: 'craft',  icon: '🔧', label: 'Craft',   special: 'craft' },
  ];

  for (const a of defs) {
    const btn = document.createElement('button');
    btn.className = 'action-btn' + (a.special === 'craft' ? ' craft-btn' : '');
    btn.dataset.kidsAction = a.kidsAction;
    if (a.cmd)     btn.dataset.cmd     = a.cmd;
    if (a.special) btn.dataset.special = a.special;
    const icon = document.createElement('span');
    icon.className = 'btn-icon';
    icon.textContent = a.icon;
    btn.appendChild(icon);
    btn.appendChild(document.createTextNode(' ' + a.label));
    grid.appendChild(btn);
  }
}
```

- [ ] **Step 2: Add `showTargetPicker()` and `hideTargetPicker()`**

After `rebuildKidsActionButtons`, add:

```typescript
function showTargetPicker(
  label: string,
  targets: Array<{ id: string; name: string }>,
  onPick: (id: string) => void,
): void {
  const picker      = document.getElementById('target-picker');
  const pickerLabel = document.getElementById('target-picker-label');
  const pickerBtns  = document.getElementById('target-picker-btns');
  if (!picker || !pickerLabel || !pickerBtns) return;

  pickerLabel.textContent = label;
  while (pickerBtns.firstChild) pickerBtns.removeChild(pickerBtns.firstChild);

  for (const t of targets) {
    const btn = document.createElement('button');
    btn.className = 'target-btn';
    btn.textContent = t.name;
    const capturedId = t.id;
    btn.addEventListener('click', () => {
      hideTargetPicker();
      onPick(capturedId);
    });
    pickerBtns.appendChild(btn);
  }
  picker.classList.add('open');
}

function hideTargetPicker(): void {
  document.getElementById('target-picker')?.classList.remove('open');
}
```

- [ ] **Step 3: Add `openKidsQuestModal` stub (real implementation in Task 11)**

After `hideTargetPicker`, add:

```typescript
// stub — replaced in Task 11
function openKidsQuestModal(): void {}
```

- [ ] **Step 4: Add `handleKidsAction()`**

After the stub, add:

```typescript
function handleKidsAction(action: string): void {
  if (!inputEnabled) return;
  const state = _lastState;

  if (action === 'quests') {
    openKidsQuestModal();
    return;
  }
  if (action === 'look' || action === 'search' || action === 'skills') {
    hideTargetPicker();
    sendCommand(action);
    return;
  }
  if (action === 'craft') {
    return; // handled by the existing craft special in the click listener
  }

  if (action === 'talk') {
    const talkers = (state?.room_npcs ?? []).filter(n => n.can_talk);
    if (talkers.length === 0) return;
    if (talkers.length === 1) { sendCommand(`talk ${talkers[0].id}`); return; }
    showTargetPicker('Who do you want to talk to?', talkers, id => sendCommand(`talk ${id}`));
    return;
  }

  if (action === 'attack') {
    const hostiles = (state?.room_npcs ?? []).filter(n => n.attackable);
    if (hostiles.length === 0) return;
    if (hostiles.length === 1) { sendCommand(`attack ${hostiles[0].id}`); return; }
    showTargetPicker('Who do you want to attack?', hostiles, id => sendCommand(`attack ${id}`));
    return;
  }

  if (action === 'trade') {
    const traders = (state?.room_npcs ?? []).filter(n => n.can_trade);
    if (traders.length === 0) return;
    if (traders.length === 1) { sendCommand(`trade ${traders[0].id}`); return; }
    showTargetPicker('Who do you want to trade with?', traders, id => sendCommand(`trade ${id}`));
    return;
  }

  if (action === 'forage') {
    const resources = state?.room_resources ?? [];
    if (resources.length === 0) return;
    if (resources.length === 1) {
      sendCommand(`${resources[0].action} ${resources[0].id}`);
      return;
    }
    const namedResources = resources.map(r => ({ id: r.id, name: formatResourceName(r.id) }));
    showTargetPicker('What do you want to gather?', namedResources, id => {
      const res = resources.find(r => r.id === id);
      if (res) sendCommand(`${res.action} ${res.id}`);
    });
  }
}
```

- [ ] **Step 5: Update the action-grid click handler to use kids mode**

Replace the existing action-grid event listener:

```typescript
  document.getElementById('action-grid')?.addEventListener('click', (e) => {
    const btn = (e.target as HTMLElement).closest<HTMLButtonElement>('.action-btn');
    if (!btn) return;
    if (btn.dataset.special === 'craft') { openCraftModal(); return; }
    if (btn.dataset.cmd && inputEnabled) sendCommand(btn.dataset.cmd);
  });
```

with:

```typescript
  document.getElementById('action-grid')?.addEventListener('click', (e) => {
    const btn = (e.target as HTMLElement).closest<HTMLButtonElement>('.action-btn');
    if (!btn) return;
    if (btn.dataset.special === 'craft') { openCraftModal(); return; }

    if (_kidsMode && btn.dataset.kidsAction) {
      handleKidsAction(btn.dataset.kidsAction);
      return;
    }

    if (btn.dataset.cmd && inputEnabled) sendCommand(btn.dataset.cmd);
  });
```

- [ ] **Step 6: Add picker dismiss on outside click**

After the action-grid event listener, add:

```typescript
  document.addEventListener('click', (e) => {
    const picker = document.getElementById('target-picker');
    const grid   = document.getElementById('action-grid');
    if (
      picker &&
      grid &&
      !picker.contains(e.target as Node) &&
      !grid.contains(e.target as Node)
    ) {
      hideTargetPicker();
    }
  });
```

- [ ] **Step 7: Build and verify**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun run build 2>&1 | tail -20
```

Expected: clean build

- [ ] **Step 8: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/src/lib/mud.ts && git commit -m "feat(web): add kids context-sensitive action buttons and inline target picker"
```

---

### Task 10: Collapsed input toggle + onboarding hints

**Files:**
- Modify: `web/src/lib/mud.ts`

- [ ] **Step 1: Add input toggle logic inside `initMUD`**

After the existing craft modal overlay close handler, add:

```typescript
  // ── Kids input toggle ──────────────────────────────────────────────────────
  const kidsToggle    = document.getElementById('kids-input-toggle');
  const INPUT_OPEN_KEY = 'bh-kids-input-open';

  function applyInputVisibility(open: boolean): void {
    const cmdEl    = document.getElementById('cmd-input');
    const sendEl   = document.getElementById('send-btn');
    const promptEl = document.querySelector<HTMLElement>('.prompt');
    if (open) {
      cmdEl?.classList.add('kids-visible');
      sendEl?.classList.add('kids-visible');
      promptEl?.classList.add('kids-visible');
      if (kidsToggle) kidsToggle.style.opacity = '1';
      (document.getElementById('cmd-input') as HTMLInputElement | null)?.focus();
    } else {
      cmdEl?.classList.remove('kids-visible');
      sendEl?.classList.remove('kids-visible');
      promptEl?.classList.remove('kids-visible');
      if (kidsToggle) kidsToggle.style.opacity = '0.45';
    }
  }

  if (kidsToggle) {
    const savedOpen = localStorage.getItem(INPUT_OPEN_KEY) === 'true';
    if (_kidsMode) applyInputVisibility(savedOpen);

    kidsToggle.addEventListener('click', () => {
      if (!_kidsMode) return;
      const isOpen = document.getElementById('cmd-input')?.classList.contains('kids-visible') ?? false;
      const next = !isOpen;
      applyInputVisibility(next);
      localStorage.setItem(INPUT_OPEN_KEY, String(next));
      if (next) showHint('first_type', 'You can also type commands if you want.');
    });
  }
```

- [ ] **Step 2: Add hint system**

After the toggle block, add:

```typescript
  // ── Kids onboarding hints ──────────────────────────────────────────────────
  const HINTS_KEY = 'bh_hints_seen';

  function getSeenHints(): Set<string> {
    try {
      const raw = localStorage.getItem(HINTS_KEY);
      return new Set(raw ? JSON.parse(raw) as string[] : []);
    } catch { return new Set(); }
  }

  function markHintSeen(key: string): void {
    const seen = getSeenHints();
    seen.add(key);
    localStorage.setItem(HINTS_KEY, JSON.stringify([...seen]));
  }

  function showHint(key: string, message: string): void {
    if (!_kidsMode) return;
    if (getSeenHints().has(key)) return;
    markHintSeen(key);

    const banner = document.getElementById('hint-banner');
    if (!banner) return;

    while (banner.firstChild) banner.removeChild(banner.firstChild);

    const text = document.createElement('span');
    text.textContent = message;

    const close = document.createElement('button');
    close.className = 'hint-close';
    close.textContent = '✕';
    close.addEventListener('click', () => banner.classList.remove('visible'));

    banner.appendChild(text);
    banner.appendChild(close);
    banner.classList.add('visible');

    setTimeout(() => banner.classList.remove('visible'), 5000);
  }
```

- [ ] **Step 3: Fire hints at the right moments**

In `showHUD()`, after `setInputEnabled(true);`, add:

```typescript
    if (_kidsMode) {
      setTimeout(() => showHint('first_login', "Click the buttons below to do things. They change based on what's around you!"), 1500);
    }
```

In `applyStateUpdate()`, after `_lastState = state;`, add:

```typescript
    if (_kidsMode) {
      if ((state.room_npcs ?? []).length > 0) {
        showHint('first_npc', "Someone's here! Click Talk to chat with them.");
      }
      if ((state.inventory ?? []).length > 0) {
        showHint('first_item', "Check Your Stuff to see what you're carrying.");
      }
      if ((state.quests ?? []).length > 0) {
        showHint('first_quest', 'A new quest! Tap Quests to track your progress.');
      }
    }
```

- [ ] **Step 4: Build and verify**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun run build 2>&1 | tail -20
```

Expected: clean build (note: `showHint` is referenced in `applyStateUpdate` before it's declared in the closure — move the `showHint` function definition above `applyStateUpdate` if TS complains about use-before-assign)

- [ ] **Step 5: Commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/src/lib/mud.ts && git commit -m "feat(web): add kids input collapse toggle and onboarding hint system"
```

---

### Task 11: Quest progress modal

**Files:**
- Modify: `web/src/lib/mud.ts`

- [ ] **Step 1: Replace `openKidsQuestModal` stub with the real implementation**

Remove `function openKidsQuestModal(): void {}` and replace with:

```typescript
function openKidsQuestModal(): void {
  const modal = document.getElementById('quest-kids-modal');
  const list  = document.getElementById('quest-kids-list');
  if (!modal || !list) return;

  while (list.firstChild) list.removeChild(list.firstChild);

  const questData = _lastState?.quests ?? [];

  if (questData.length === 0) {
    const empty = document.createElement('div');
    empty.style.cssText = 'font-size:0.78rem;color:var(--comment);text-align:center;padding:1rem 0';
    empty.textContent = 'No active quests yet.';
    list.appendChild(empty);
  } else {
    for (const q of questData) {
      const card = document.createElement('div');
      card.className = 'quest-kids-card';

      const title = document.createElement('div');
      title.className = 'quest-kids-title';
      title.textContent = q.title;

      const barWrap = document.createElement('div');
      barWrap.className = 'quest-progress-bar-wrap';

      const barFill = document.createElement('div');
      barFill.className = 'quest-progress-bar-fill';
      const pct = q.obj_count > 0
        ? Math.min(100, Math.round((q.obj_progress / q.obj_count) * 100))
        : 0;
      barFill.style.width = `${pct}%`;
      barWrap.appendChild(barFill);

      const progressLabel = document.createElement('div');
      progressLabel.className = 'quest-progress-label';
      if (q.obj_count > 1) {
        progressLabel.textContent = `${q.obj_progress} of ${q.obj_count}`;
      } else if (q.obj_progress > 0) {
        progressLabel.textContent = 'Complete!';
      } else {
        progressLabel.textContent = 'In progress';
      }

      card.appendChild(title);
      card.appendChild(barWrap);
      card.appendChild(progressLabel);
      list.appendChild(card);
    }
  }

  modal.classList.add('open');
}
```

- [ ] **Step 2: Wire up the quest modal close button inside `initMUD`**

After the item modal overlay-click handler, add:

```typescript
  document.getElementById('quest-kids-modal-close')?.addEventListener('click', () => {
    document.getElementById('quest-kids-modal')?.classList.remove('open');
  });

  document.getElementById('quest-kids-modal')?.addEventListener('click', (e) => {
    if (e.target === e.currentTarget) {
      (e.currentTarget as HTMLElement).classList.remove('open');
    }
  });
```

- [ ] **Step 3: Build to verify clean**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun run build 2>&1 | tail -20
```

Expected: clean build

- [ ] **Step 4: Run frontend tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud/web && bun test
```

Expected: PASS (existing `buildWsUrl` tests pass)

- [ ] **Step 5: Run all Go tests**

```bash
cd /Users/stokes/Projects/gl1tch-mud && go test ./...
```

Expected: all PASS

- [ ] **Step 6: Final commit**

```bash
cd /Users/stokes/Projects/gl1tch-mud && git add web/src/lib/mud.ts && git commit -m "feat(web): add kids quest progress modal with visual progress bars"
```

---

## End-to-End Verification

After all tasks complete:

1. Start Block Haven — `data-ui="kids"` is on `<body>`
2. Compass and network panels are hidden; room context shows WHO'S HERE
3. Action buttons show only relevant ones (no Attack if no enemies)
4. Clicking Talk with 1 NPC fires the command; 2 NPCs shows the picker
5. Clicking an exit button navigates without typing
6. Text input is collapsed by default; ⌨️ expands it
7. Onboarding hints appear once (clear `bh_hints_seen` localStorage to reset)
8. Quests button opens visual modal with progress bars
9. Open `?world=cyberspace` — none of the kids changes are visible
