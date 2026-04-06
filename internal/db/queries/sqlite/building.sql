-- name: CountBuildsByType :one
SELECT COUNT(*) FROM builds WHERE room_id = ? AND build_id = ?;

-- name: InsertBuild :exec
INSERT INTO builds (room_id, build_id, name, desc, placed_at) VALUES (?, ?, ?, ?, ?);

-- name: CountChestInRoom :one
SELECT COUNT(*) FROM builds WHERE room_id = ? AND build_id = 'chest';

-- name: ListChestItems :many
SELECT item_id, item_name FROM chests WHERE room_id = ?;

-- name: GetChestItem :one
SELECT item_name, item_desc FROM chests WHERE room_id = ? AND item_id = ?;

-- name: DeleteInventoryItem :exec
DELETE FROM inventory WHERE item_id = ?;

-- name: InsertChestItem :exec
INSERT OR IGNORE INTO chests (room_id, item_id, item_name, item_desc) VALUES (?, ?, ?, ?);

-- name: DeleteChestItem :exec
DELETE FROM chests WHERE room_id = ? AND item_id = ?;

-- name: InsertInventoryItem :exec
INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?, ?, ?);
