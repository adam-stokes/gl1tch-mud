-- name: SeedCrystalShard :exec
INSERT OR IGNORE INTO crystal_shards (shard_id, biome, collected, collected_at) VALUES (?, ?, 0, 0);

-- name: CountStartingItem :one
SELECT COUNT(*) FROM inventory WHERE item_id = 'wooden-pickaxe';

-- name: InsertStartingItem :exec
INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?, ?, ?);
