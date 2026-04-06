-- name: BumpActions :exec
INSERT INTO player_actions (id, count) VALUES (1, 1)
ON CONFLICT(id) DO UPDATE SET count = count + 1;

-- name: GetResourceState :one
SELECT depleted, depleted_at_action FROM room_resources WHERE room_id = ? AND resource_id = ?;

-- name: ClearResourceDepletion :exec
UPDATE room_resources SET depleted = 0 WHERE room_id = ? AND resource_id = ?;

-- name: DepleteResource :exec
INSERT INTO room_resources (room_id, resource_id, depleted, depleted_at_action) VALUES (?, ?, 1, ?)
ON CONFLICT(room_id, resource_id) DO UPDATE SET depleted = 1, depleted_at_action = excluded.depleted_at_action;

-- name: ListReadyCrops :many
SELECT seed_id FROM crops WHERE room_id = ? AND ready_at_action <= ? AND harvested = 0;

-- name: CountReadyCrops :one
SELECT COUNT(*) FROM crops WHERE room_id = ? AND seed_id = ? AND ready_at_action <= ? AND harvested = 0;

-- name: HarvestCrops :exec
UPDATE crops SET harvested = 1 WHERE room_id = ? AND seed_id = ? AND ready_at_action <= ? AND harvested = 0;

-- name: ListActiveCropSlots :many
SELECT slot FROM crops WHERE room_id = ? AND harvested = 0;

-- name: InsertCrop :exec
INSERT INTO crops (room_id, slot, seed_id, planted_at_action, ready_at_action) VALUES (?, ?, ?, ?, ?);
