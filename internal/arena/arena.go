// Package arena manages single-player arena mini-game sessions.
package arena

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/adam-stokes/gl1tch-mud/internal/base"
	"github.com/adam-stokes/gl1tch-mud/internal/credits"
	"github.com/adam-stokes/gl1tch-mud/internal/player"
	"github.com/adam-stokes/gl1tch-mud/internal/world"
)

const (
	tdmRaiderCount   = 5
	tdWaveCount      = 3
	tdRaidersPerWave = 3
	playerDamage     = 15
)

// Enemy represents one opponent in an arena match.
type Enemy struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	HP     int    `json:"hp"`
	Attack int    `json:"attack"`
	Alive  bool   `json:"alive"`
}

// Match represents a loaded arena session row.
type Match struct {
	ID             string
	GameType       string
	Phase          string
	Wave           int
	Enemies        []Enemy
	RewardCredits  int
	RewardItemID   string
	RewardItemName string
	RewardItemDesc string
	Status         string
	StartedAt      int64
}

// AttackResult is returned by ProcessAttack.
type AttackResult struct {
	Output string
	Won    bool
	Lost   bool
}

// StartTDM creates a new active TDM match with 5 raiders.
func StartTDM(db *sql.DB) error {
	enemies := makeTDMEnemies()
	enemyJSON, _ := json.Marshal(enemies)
	id := fmt.Sprintf("arena-%d", time.Now().UnixNano())
	_, err := db.Exec(
		`INSERT OR REPLACE INTO arena_sessions
		 (id, game_type, phase, wave, enemies_json, reward_credits,
		  reward_item_id, reward_item_name, reward_item_desc, status, started_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, "tdm", "fight", 0, string(enemyJSON), 200,
		"", "", "", "active", time.Now().Unix(),
	)
	return err
}

// StartTowerDefense creates a new active tower-defense match with wave 0.
// Applies turret auto-damage (base.DefenseScore) to the first wave's enemies immediately.
func StartTowerDefense(db *sql.DB, w *world.World) error {
	enemies := makeTDEnemies()
	defScore := base.DefenseScore(db, w)
	enemies = applyTurretDamage(enemies, defScore)
	enemyJSON, _ := json.Marshal(enemies)
	id := fmt.Sprintf("arena-%d", time.Now().UnixNano())
	_, err := db.Exec(
		`INSERT OR REPLACE INTO arena_sessions
		 (id, game_type, phase, wave, enemies_json, reward_credits,
		  reward_item_id, reward_item_name, reward_item_desc, status, started_at)
		 VALUES (?,?,?,?,?,?,?,?,?,?,?)`,
		id, "tower-defense", "wave", 0, string(enemyJSON), 300,
		"pre-war-circuitry", "Pre-War Circuitry", "High-density pre-war circuit board.",
		"active", time.Now().Unix(),
	)
	return err
}

// GetActive returns the current active match, or nil if none exists.
func GetActive(db *sql.DB) *Match {
	var m Match
	var enemyJSON string
	err := db.QueryRow(
		`SELECT id, game_type, phase, wave, enemies_json, reward_credits,
		        reward_item_id, reward_item_name, reward_item_desc, status, started_at
		 FROM arena_sessions WHERE status='active' LIMIT 1`,
	).Scan(&m.ID, &m.GameType, &m.Phase, &m.Wave, &enemyJSON,
		&m.RewardCredits, &m.RewardItemID, &m.RewardItemName, &m.RewardItemDesc,
		&m.Status, &m.StartedAt)
	if err != nil {
		return nil
	}
	json.Unmarshal([]byte(enemyJSON), &m.Enemies) //nolint:errcheck
	return &m
}

// ProcessAttack executes one combat tick in the active arena match.
// Mutates s.HP in place. Returns output and won/lost flags.
func ProcessAttack(db *sql.DB, w *world.World, s *player.State) AttackResult {
	m := GetActive(db)
	if m == nil {
		return AttackResult{Output: "no active arena match."}
	}
	var out strings.Builder
	if m.GameType == "tdm" {
		return processTDMAttack(db, m, s, &out)
	}
	return processTDAttack(db, w, m, s, &out)
}

// Quit forfeits the active match and marks it lost.
func Quit(db *sql.DB) string {
	db.Exec(`UPDATE arena_sessions SET status='lost' WHERE status='active'`) //nolint:errcheck
	return "you forfeit the match."
}

// ── internal helpers ──────────────────────────────────────────────────────────

func makeTDMEnemies() []Enemy {
	enemies := make([]Enemy, tdmRaiderCount)
	for i := range enemies {
		enemies[i] = Enemy{
			ID:     fmt.Sprintf("raider-%d", i+1),
			Name:   "Ash Raider",
			HP:     30,
			Attack: 8,
			Alive:  true,
		}
	}
	return enemies
}

func makeTDEnemies() []Enemy {
	enemies := make([]Enemy, tdRaidersPerWave)
	for i := range enemies {
		enemies[i] = Enemy{
			ID:     fmt.Sprintf("wave-raider-%d", i+1),
			Name:   "Ash Raider",
			HP:     25,
			Attack: 6,
			Alive:  true,
		}
	}
	return enemies
}

// applyTurretDamage distributes defScore damage across enemies evenly.
// Remainder is applied to enemy index 0.
func applyTurretDamage(enemies []Enemy, defScore int) []Enemy {
	if defScore <= 0 || len(enemies) == 0 {
		return enemies
	}
	perEnemy := defScore / len(enemies)
	remainder := defScore % len(enemies)
	for i := range enemies {
		dmg := perEnemy
		if i == 0 {
			dmg += remainder
		}
		enemies[i].HP -= dmg
		if enemies[i].HP <= 0 {
			enemies[i].HP = 0
			enemies[i].Alive = false
		}
	}
	return enemies
}

func aliveCount(enemies []Enemy) int {
	n := 0
	for _, e := range enemies {
		if e.Alive {
			n++
		}
	}
	return n
}

func firstAliveIdx(enemies []Enemy) int {
	for i, e := range enemies {
		if e.Alive {
			return i
		}
	}
	return -1
}

func saveMatch(db *sql.DB, m *Match) {
	enemyJSON, _ := json.Marshal(m.Enemies)
	db.Exec( //nolint:errcheck
		`UPDATE arena_sessions SET phase=?, wave=?, enemies_json=?, status=? WHERE id=?`,
		m.Phase, m.Wave, string(enemyJSON), m.Status, m.ID,
	)
}

func processTDMAttack(db *sql.DB, m *Match, s *player.State, out *strings.Builder) AttackResult {
	idx := firstAliveIdx(m.Enemies)
	if idx == -1 {
		return AttackResult{Output: "no enemies left."}
	}

	m.Enemies[idx].HP -= playerDamage
	if m.Enemies[idx].HP <= 0 {
		m.Enemies[idx].HP = 0
		m.Enemies[idx].Alive = false
		fmt.Fprintf(out, "you fire at %s. [%d dmg → dead]\n", m.Enemies[idx].Name, playerDamage)
	} else {
		fmt.Fprintf(out, "you fire at %s. [%d dmg → %d HP]\n", m.Enemies[idx].Name, playerDamage, m.Enemies[idx].HP)
	}

	alive := aliveCount(m.Enemies)
	if alive == 0 {
		m.Status = "won"
		saveMatch(db, m)
		credits.Add(db, m.RewardCredits) //nolint:errcheck
		fmt.Fprintf(out, "--- all enemies down. match won. ---\n+%d caps deposited.", m.RewardCredits)
		return AttackResult{Output: strings.TrimRight(out.String(), "\n"), Won: true}
	}

	for _, e := range m.Enemies {
		if !e.Alive {
			continue
		}
		dmg := e.Attack - s.Defense
		if dmg < 1 {
			dmg = 1
		}
		s.HP -= dmg
		fmt.Fprintf(out, "%s retaliates for %d. your HP: %d/%d.\n", e.Name, dmg, s.HP, s.MaxHP)
	}
	fmt.Fprintf(out, "--- %d enemies remaining ---", alive)

	if s.HP <= 0 {
		m.Status = "lost"
		saveMatch(db, m)
		return AttackResult{Output: strings.TrimRight(out.String(), "\n"), Lost: true}
	}

	saveMatch(db, m)
	return AttackResult{Output: strings.TrimRight(out.String(), "\n")}
}

func processTDAttack(db *sql.DB, w *world.World, m *Match, s *player.State, out *strings.Builder) AttackResult {
	// All current wave enemies dead — advance wave or win
	if aliveCount(m.Enemies) == 0 {
		m.Wave++
		if m.Wave >= tdWaveCount {
			m.Status = "won"
			saveMatch(db, m)
			credits.Add(db, m.RewardCredits) //nolint:errcheck
			if m.RewardItemID != "" {
				player.AddItem(db, m.RewardItemID, m.RewardItemName, m.RewardItemDesc) //nolint:errcheck
			}
			fmt.Fprintf(out, "--- all waves survived. match won. ---\n+%d caps deposited.", m.RewardCredits)
			if m.RewardItemID != "" {
				fmt.Fprintf(out, "\n%s added to inventory.", m.RewardItemName)
			}
			return AttackResult{Output: strings.TrimRight(out.String(), "\n"), Won: true}
		}

		s.HP += 15
		if s.HP > s.MaxHP {
			s.HP = s.MaxHP
		}
		enemies := makeTDEnemies()
		defScore := base.DefenseScore(db, w)
		enemies = applyTurretDamage(enemies, defScore)
		m.Enemies = enemies
		fmt.Fprintf(out, "Wave %d cleared. +15 HP. [HP: %d/%d]\n--- Wave %d incoming ---\n", m.Wave, s.HP, s.MaxHP, m.Wave+1)
		if defScore > 0 {
			perEnemy := defScore / tdRaidersPerWave
			remainder := defScore % tdRaidersPerWave
			for i, e := range m.Enemies {
				applied := perEnemy
				if i == 0 {
					applied += remainder
				}
				if applied > 0 {
					fmt.Fprintf(out, "  %s takes %d turret damage. [%d HP]\n", e.Name, applied, e.HP)
				}
			}
		}
		saveMatch(db, m)
		return AttackResult{Output: strings.TrimRight(out.String(), "\n")}
	}

	// Attack first alive enemy
	idx := firstAliveIdx(m.Enemies)
	m.Enemies[idx].HP -= playerDamage
	if m.Enemies[idx].HP <= 0 {
		m.Enemies[idx].HP = 0
		m.Enemies[idx].Alive = false
		fmt.Fprintf(out, "you fire at %s. [%d dmg → dead]\n", m.Enemies[idx].Name, playerDamage)
	} else {
		fmt.Fprintf(out, "you fire at %s. [%d dmg → %d HP]\n", m.Enemies[idx].Name, playerDamage, m.Enemies[idx].HP)
	}

	alive := aliveCount(m.Enemies)
	if alive == 0 {
		fmt.Fprintf(out, "--- wave cleared. type 'attack' to continue. ---")
		saveMatch(db, m)
		return AttackResult{Output: strings.TrimRight(out.String(), "\n")}
	}

	for _, e := range m.Enemies {
		if !e.Alive {
			continue
		}
		dmg := e.Attack - s.Defense
		if dmg < 1 {
			dmg = 1
		}
		s.HP -= dmg
		fmt.Fprintf(out, "%s retaliates for %d. your HP: %d/%d.\n", e.Name, dmg, s.HP, s.MaxHP)
	}
	fmt.Fprintf(out, "--- %d enemies remaining ---", alive)

	if s.HP <= 0 {
		m.Status = "lost"
		saveMatch(db, m)
		return AttackResult{Output: strings.TrimRight(out.String(), "\n"), Lost: true}
	}

	saveMatch(db, m)
	return AttackResult{Output: strings.TrimRight(out.String(), "\n")}
}
