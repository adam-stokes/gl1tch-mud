-- name: GetActionCountBase :one
SELECT count FROM player_actions WHERE id = 1;

-- name: ListBuildIDsInRoom :many
SELECT build_id FROM builds WHERE room_id = ?;

-- name: CountBuildsInRoom :one
SELECT COUNT(*) FROM builds WHERE room_id = ?;

-- name: CountActiveBaseRaids :one
SELECT COUNT(*) FROM world_events WHERE type = 'base-raid' AND target_room = ? AND status = 'active';

-- name: InsertWorldEvent :exec
INSERT INTO world_events
 (id, type, title, description, target_room, faction,
  payout_credits, payout_item_id, payout_item_name, payout_item_desc,
  status, expires_actions, created_actions, created_at)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListExpiredBaseRaids :many
SELECT id FROM world_events
WHERE type = 'base-raid' AND target_room = ? AND status = 'active'
AND (created_actions + expires_actions) <= ?;

-- name: ResolveWorldEvent :exec
UPDATE world_events SET status = 'resolved' WHERE id = ?;

-- name: ListRandomChestItems :many
SELECT item_id, item_name FROM chests WHERE room_id = ? ORDER BY RANDOM() LIMIT ?;

-- name: DeleteChestItemBase :exec
DELETE FROM chests WHERE room_id = ? AND item_id = ?;
