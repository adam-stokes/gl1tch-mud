-- name: CountEnchantingTable :one
SELECT COUNT(*) FROM builds WHERE room_id = ? AND build_id = 'enchanting-table';

-- name: DeductEnchantingXP :exec
UPDATE enchanting_xp SET xp = xp - ? WHERE id = 1;
