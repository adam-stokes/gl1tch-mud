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

CREATE TABLE IF NOT EXISTS player_credits (
    id      INTEGER PRIMARY KEY CHECK(id = 1),
    credits INTEGER NOT NULL DEFAULT 0
);
INSERT OR IGNORE INTO player_credits (id, credits) VALUES (1, 0);

CREATE TABLE IF NOT EXISTS quests (
    id               TEXT    PRIMARY KEY,
    title            TEXT    NOT NULL,
    description      TEXT,
    status           TEXT    NOT NULL DEFAULT 'active',
    obj_type         TEXT    NOT NULL,
    obj_target       TEXT    NOT NULL,
    obj_room         TEXT,
    obj_count        INTEGER NOT NULL DEFAULT 1,
    obj_progress     INTEGER NOT NULL DEFAULT 0,
    reward_credits   INTEGER NOT NULL DEFAULT 0,
    reward_xp_skill  TEXT,
    reward_xp_amount INTEGER NOT NULL DEFAULT 0,
    reward_item_id   TEXT,
    reward_item_name TEXT,
    reward_item_desc TEXT,
    giver_npc_id     TEXT,
    accepted_at      INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS world_events (
    id               TEXT    PRIMARY KEY,
    type             TEXT    NOT NULL,
    title            TEXT    NOT NULL,
    description      TEXT,
    target_room      TEXT    NOT NULL,
    faction          TEXT,
    payout_credits   INTEGER NOT NULL DEFAULT 0,
    payout_item_id   TEXT,
    payout_item_name TEXT,
    payout_item_desc TEXT,
    status           TEXT    NOT NULL DEFAULT 'active',
    expires_actions  INTEGER NOT NULL DEFAULT 20,
    created_actions  INTEGER NOT NULL DEFAULT 0,
    created_at       INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS player_faction (
    id              INTEGER PRIMARY KEY CHECK(id = 1),
    faction_id      TEXT    NOT NULL,
    faction_name    TEXT    NOT NULL,
    agenda          TEXT,
    hideout_room_id TEXT,
    credits         INTEGER NOT NULL DEFAULT 0,
    created_at      INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS faction_members (
    npc_id         TEXT    PRIMARY KEY,
    npc_name       TEXT    NOT NULL,
    npc_desc       TEXT,
    role           TEXT    NOT NULL DEFAULT 'associate',
    stationed_room TEXT,
    loyalty        INTEGER NOT NULL DEFAULT 50,
    recruited_at   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS hideout_upgrades (
    upgrade_id   TEXT    PRIMARY KEY,
    installed_at INTEGER NOT NULL
);
`
