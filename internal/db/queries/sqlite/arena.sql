-- name: InsertArenaSession :exec
INSERT OR REPLACE INTO arena_sessions
 (id, game_type, phase, wave, enemies_json, reward_credits,
  reward_item_id, reward_item_name, reward_item_desc, status, started_at)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: GetActiveArenaSession :one
SELECT id, game_type, phase, wave, enemies_json, reward_credits,
       reward_item_id, reward_item_name, reward_item_desc, status, started_at
FROM arena_sessions WHERE status = 'active' LIMIT 1;

-- name: UpdateArenaSession :exec
UPDATE arena_sessions SET phase = ?, wave = ?, enemies_json = ?, status = ? WHERE id = ?;

-- name: QuitArenaSession :exec
UPDATE arena_sessions SET status = 'lost' WHERE status = 'active';
