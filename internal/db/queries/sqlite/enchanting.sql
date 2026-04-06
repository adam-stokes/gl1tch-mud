-- name: ApplyEnchant :exec
INSERT INTO enchants (item_id, enchant_id, level, applied_at) VALUES (?, ?, ?, ?)
ON CONFLICT(item_id, enchant_id) DO UPDATE SET level = excluded.level, applied_at = excluded.applied_at;

-- name: ListEnchants :many
SELECT item_id, enchant_id, level FROM enchants WHERE item_id = ?;

-- name: AddEnchantingXP :exec
UPDATE enchanting_xp
SET xp    = xp + ?,
    level = MIN(MAX(1, (xp + ?) / 100), 30)
WHERE id = 1;

-- name: GetEnchantingXPState :one
SELECT xp, level FROM enchanting_xp WHERE id = 1;
