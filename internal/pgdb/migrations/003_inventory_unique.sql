-- 003_inventory_unique.sql: deduplicate shared_inventory and enforce uniqueness
-- Postgres allowed two rows of the same item per (account, world) because the
-- table only had a UUID primary key. SQLite uses INSERT OR IGNORE on a unique
-- key, so behaviour was inconsistent across backends.

-- Remove duplicate inventory rows, keeping the first row inserted.
DELETE FROM shared_inventory a USING shared_inventory b
WHERE a.id > b.id
  AND a.account_id = b.account_id
  AND a.world_id = b.world_id
  AND a.item_id = b.item_id;

-- Add unique constraint matching SQLite's behaviour.
ALTER TABLE shared_inventory
ADD CONSTRAINT shared_inventory_unique
UNIQUE (account_id, world_id, item_id);
