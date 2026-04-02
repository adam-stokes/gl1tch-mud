package db

const schema = `
CREATE TABLE IF NOT EXISTS player (
    id        INTEGER PRIMARY KEY,
    name      TEXT    NOT NULL DEFAULT 'hacker',
    room_id   TEXT    NOT NULL DEFAULT 'net-0',
    hp        INTEGER NOT NULL DEFAULT 100,
    max_hp    INTEGER NOT NULL DEFAULT 100,
    world     TEXT    NOT NULL DEFAULT 'cyberspace'
);

CREATE TABLE IF NOT EXISTS inventory (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    item_id   TEXT    NOT NULL UNIQUE,
    item_name TEXT    NOT NULL,
    item_desc TEXT    NOT NULL DEFAULT ''
);

CREATE TABLE IF NOT EXISTS npc_state (
    room_id TEXT    NOT NULL,
    npc_id  TEXT    NOT NULL,
    hp      INTEGER NOT NULL,
    alive   INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (room_id, npc_id)
);

CREATE TABLE IF NOT EXISTS visited (
    room_id TEXT NOT NULL PRIMARY KEY
);
`
