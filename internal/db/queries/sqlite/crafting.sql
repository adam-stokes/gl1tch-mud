-- name: CountUnlockedRecipe :one
SELECT COUNT(*) FROM unlocked_recipes WHERE recipe_id = ?;

-- name: DeleteOneInventoryItem :exec
DELETE FROM inventory WHERE rowid IN (SELECT inventory.rowid FROM inventory WHERE inventory.item_id = ? LIMIT 1);

-- name: InsertInventoryItemCraft :exec
INSERT OR IGNORE INTO inventory (item_id, item_name, item_desc) VALUES (?, ?, ?);

-- name: UnlockRecipe :exec
INSERT OR IGNORE INTO unlocked_recipes (recipe_id, unlocked_at) VALUES (?, ?);

-- name: IsRecipeUnlocked :one
SELECT COUNT(*) FROM unlocked_recipes WHERE recipe_id = ?;

-- name: SetPlayerFlag :exec
INSERT OR IGNORE INTO player_flags (flag) VALUES (?);

-- name: CountPlayerFlag :one
SELECT COUNT(*) FROM player_flags WHERE flag = ?;
