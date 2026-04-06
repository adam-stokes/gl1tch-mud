-- 002_taken_room_items.sql: track taken YAML-defined room items in shared worlds

CREATE TABLE IF NOT EXISTS shared_taken_room_items (
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    taken_by UUID,
    taken_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (world_id, room_id, item_id)
);
