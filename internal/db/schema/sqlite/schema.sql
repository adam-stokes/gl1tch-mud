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
    accepted_at      INTEGER NOT NULL,
    next_quest_id    TEXT    NOT NULL DEFAULT ''
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

CREATE TABLE IF NOT EXISTS room_resources (
    room_id             TEXT    NOT NULL,
    resource_id         TEXT    NOT NULL,
    depleted            INTEGER NOT NULL DEFAULT 0,
    depleted_at_action  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (room_id, resource_id)
);

CREATE TABLE IF NOT EXISTS weather_state (
    biome       TEXT    PRIMARY KEY,
    condition   TEXT    NOT NULL DEFAULT 'clear',
    expires_action INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS enchants (
    item_id     TEXT    NOT NULL,
    enchant_id  TEXT    NOT NULL,
    level       INTEGER NOT NULL DEFAULT 1,
    applied_at  INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (item_id, enchant_id)
);

CREATE TABLE IF NOT EXISTS builds (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id     TEXT    NOT NULL,
    build_id    TEXT    NOT NULL,
    name        TEXT    NOT NULL,
    desc        TEXT    NOT NULL DEFAULT '',
    placed_at   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS crystal_shards (
    shard_id        TEXT    PRIMARY KEY,
    biome           TEXT    NOT NULL,
    collected       INTEGER NOT NULL DEFAULT 0,
    collected_at    INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS death_pile (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    room_id     TEXT    NOT NULL,
    item_id     TEXT    NOT NULL,
    item_name   TEXT    NOT NULL,
    item_desc   TEXT    NOT NULL DEFAULT '',
    died_at     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_death_pile_room ON death_pile(room_id);

CREATE TABLE IF NOT EXISTS enchanting_xp (
    id      INTEGER PRIMARY KEY CHECK (id = 1),
    xp      INTEGER NOT NULL DEFAULT 0,
    level   INTEGER NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS crops (
    room_id         TEXT    NOT NULL,
    slot            INTEGER NOT NULL,
    seed_id         TEXT    NOT NULL,
    planted_at_action   INTEGER NOT NULL,
    ready_at_action     INTEGER NOT NULL,
    harvested       INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (room_id, slot)
);

CREATE TABLE IF NOT EXISTS chests (
    room_id     TEXT    NOT NULL,
    item_id     TEXT    NOT NULL,
    item_name   TEXT    NOT NULL,
    item_desc   TEXT    NOT NULL DEFAULT '',
    PRIMARY KEY (room_id, item_id)
);

CREATE TABLE IF NOT EXISTS player_flags (
    flag TEXT PRIMARY KEY
);

CREATE TABLE IF NOT EXISTS equipped_armor (
    id        INTEGER PRIMARY KEY CHECK(id = 1),
    item_id   TEXT    NOT NULL,
    item_name TEXT    NOT NULL,
    defense   INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS arena_sessions (
    id               TEXT    PRIMARY KEY,
    game_type        TEXT    NOT NULL,
    phase            TEXT    NOT NULL DEFAULT 'fight',
    wave             INTEGER NOT NULL DEFAULT 0,
    enemies_json     TEXT    NOT NULL DEFAULT '[]',
    reward_credits   INTEGER NOT NULL DEFAULT 0,
    reward_item_id   TEXT    NOT NULL DEFAULT '',
    reward_item_name TEXT    NOT NULL DEFAULT '',
    reward_item_desc TEXT    NOT NULL DEFAULT '',
    status           TEXT    NOT NULL DEFAULT 'active',
    started_at       INTEGER NOT NULL DEFAULT 0
);
