-- name: GetActionCountGeneral :one
SELECT count FROM player_actions WHERE id = 1;

-- name: ListReputations :many
SELECT faction, value FROM player_reputation;

-- name: CountCollectedShards :one
SELECT COUNT(*) FROM crystal_shards WHERE collected = 1;

-- name: CountTotalShards :one
SELECT COUNT(*) FROM crystal_shards;

-- name: ListBuildsInRoom :many
SELECT build_id, name FROM builds WHERE room_id = ? ORDER BY placed_at;

-- name: CountChestItemsInRoom :one
SELECT COUNT(*) FROM chests WHERE room_id = ?;

-- name: ListHighRepFactions :many
SELECT faction, value FROM player_reputation WHERE value >= 3;

-- name: UpdateFactionMemberStation :exec
UPDATE faction_members SET stationed_room = ? WHERE npc_id = ?;
