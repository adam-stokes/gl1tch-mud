-- 005_inventory_stack.sql
-- Drops the unique constraint on shared_inventory so multiple rows of the
-- same item_id can stack. The 003 migration's uniqueness was incompatible
-- with the existing trade/loot code which counts duplicate rows to know how
-- many of an item the player has.

ALTER TABLE shared_inventory DROP CONSTRAINT IF EXISTS shared_inventory_unique;
