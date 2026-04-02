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

CREATE TABLE IF NOT EXISTS player_skills (
    skill TEXT    PRIMARY KEY,
    level INTEGER NOT NULL DEFAULT 0,
    xp    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS player_reputation (
    faction TEXT    PRIMARY KEY,
    value   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS system_state (
    room_id   TEXT    NOT NULL,
    system_id TEXT    NOT NULL,
    intrusion REAL    NOT NULL DEFAULT 0,
    alert     INTEGER NOT NULL DEFAULT 0,
    hacked    INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (room_id, system_id)
);

CREATE TABLE IF NOT EXISTS lock_state (
    lock_id  TEXT    PRIMARY KEY,
    unlocked INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS npc_memory (
    npc_id TEXT NOT NULL,
    action TEXT NOT NULL,
    ts     INTEGER NOT NULL,
    PRIMARY KEY (npc_id, action)
);

CREATE TABLE IF NOT EXISTS player_stealth (
    id       INTEGER PRIMARY KEY CHECK (id = 1),
    level    INTEGER NOT NULL DEFAULT 50,
    disguise TEXT    NOT NULL DEFAULT 'none'
);

CREATE TABLE IF NOT EXISTS generated_content (
    prompt_hash TEXT    PRIMARY KEY,
    type        TEXT    NOT NULL,
    yaml_blob   TEXT    NOT NULL,
    created_at  INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS unlocked_recipes (recipe_id TEXT PRIMARY KEY, unlocked_at INT);
CREATE TABLE IF NOT EXISTS player_augments (skill TEXT, bonus INT, installed_at INT);
CREATE TABLE IF NOT EXISTS item_mods (item_instance TEXT PRIMARY KEY, mod_id TEXT, applied_at INT);
CREATE TABLE IF NOT EXISTS bounties (room_id TEXT PRIMARY KEY, npc_id TEXT, created_at INT);
CREATE TABLE IF NOT EXISTS vuln_windows (system_id TEXT PRIMARY KEY, bonus INT, expires_action INT);
CREATE TABLE IF NOT EXISTS player_actions (id INT PRIMARY KEY CHECK(id=1), count INT DEFAULT 0);
`
