-- name: GetPlayer :one
SELECT name, room_id, hp, max_hp, world FROM player WHERE id = 1;

-- name: SeedPlayer :exec
INSERT INTO player (id, name, room_id, hp, max_hp, world) VALUES (1, ?, ?, ?, ?, ?);

-- name: SavePlayer :exec
UPDATE player SET room_id = ?, hp = ?, max_hp = ?, world = ? WHERE id = 1;

-- name: ListInventory :many
SELECT item_id, item_name, item_desc FROM inventory;

-- name: AddItem :exec
INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?, ?, ?);

-- name: RemoveItem :execresult
DELETE FROM inventory WHERE item_id = ?;

-- name: GetNPCState :one
SELECT hp, alive FROM npc_state WHERE room_id = ? AND npc_id = ?;

-- name: UpsertNPCDead :exec
INSERT OR REPLACE INTO npc_state (room_id, npc_id, hp, alive) VALUES (?, ?, ?, 0);

-- name: UpsertNPCAlive :exec
INSERT OR REPLACE INTO npc_state (room_id, npc_id, hp, alive) VALUES (?, ?, ?, 1);

-- name: MarkVisited :exec
INSERT OR IGNORE INTO visited (room_id) VALUES (?);

-- name: HasVisited :one
SELECT room_id FROM visited WHERE room_id = ?;

-- name: ClearInventory :exec
DELETE FROM inventory;

-- name: InsertDeathPile :exec
INSERT INTO death_pile (room_id, item_id, item_name, item_desc, died_at) VALUES (?, ?, ?, ?, ?);

-- name: GetDeathPile :many
SELECT item_id, item_name, item_desc FROM death_pile WHERE room_id = ?;

-- name: DeleteDeathPile :exec
DELETE FROM death_pile WHERE room_id = ?;

-- name: AnyDeathPile :one
SELECT room_id, COUNT(*) as count FROM death_pile GROUP BY room_id ORDER BY MAX(died_at) DESC, MAX(id) DESC LIMIT 1;

-- name: MarkShardCollected :exec
UPDATE crystal_shards SET collected = 1, collected_at = ? WHERE shard_id = ?;

-- name: GetActionCount :one
SELECT count FROM player_actions WHERE id = 1;

-- name: EquipArmor :exec
INSERT OR REPLACE INTO equipped_armor (id, item_id, item_name, defense) VALUES (1, ?, ?, ?);

-- name: UnequipArmor :exec
DELETE FROM equipped_armor WHERE id = 1;

-- name: GetEquippedArmor :one
SELECT item_id, item_name, defense FROM equipped_armor WHERE id = 1;
