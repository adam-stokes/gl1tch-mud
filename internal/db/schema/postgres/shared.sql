CREATE TABLE shared_player_state (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL DEFAULT 'net-0',
    hp INT NOT NULL DEFAULT 100,
    max_hp INT NOT NULL DEFAULT 100,
    credits INT NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared_inventory (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    item_desc TEXT NOT NULL DEFAULT ''
);

CREATE TABLE shared_player_skills (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    skill TEXT NOT NULL,
    level INT NOT NULL DEFAULT 0,
    xp INT NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id, skill)
);

CREATE TABLE shared_equipped_armor (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    defense INT NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared_npc_state (
    world_id TEXT NOT NULL,
    npc_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    hp INT NOT NULL,
    alive BOOLEAN NOT NULL DEFAULT TRUE,
    respawn_at TIMESTAMPTZ,
    PRIMARY KEY (world_id, npc_id)
);

CREATE TABLE shared_lock_state (
    world_id TEXT NOT NULL,
    lock_id TEXT NOT NULL,
    unlocked BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (world_id, lock_id)
);

CREATE TABLE shared_resources (
    world_id TEXT NOT NULL,
    resource_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    depleted BOOLEAN NOT NULL DEFAULT FALSE,
    respawn_at TIMESTAMPTZ,
    PRIMARY KEY (world_id, resource_id)
);

CREATE TABLE shared_system_state (
    world_id TEXT NOT NULL,
    system_id TEXT NOT NULL,
    hacked BOOLEAN NOT NULL DEFAULT FALSE,
    intrusion_level INT NOT NULL DEFAULT 0,
    alert_level INT NOT NULL DEFAULT 0,
    PRIMARY KEY (world_id, system_id)
);

CREATE TABLE shared_death_pile (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    item_desc TEXT NOT NULL DEFAULT '',
    dropped_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE shared_arena_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    game_type TEXT NOT NULL,
    wave INT NOT NULL DEFAULT 0,
    enemies TEXT NOT NULL DEFAULT '[]',
    started_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE shared_builds (
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    build_id TEXT NOT NULL,
    placed_by UUID,
    placed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (world_id, room_id, build_id)
);

CREATE TABLE shared_chests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    item_name TEXT NOT NULL,
    stored_by UUID
);

CREATE TABLE shared_player_reputation (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    faction TEXT NOT NULL,
    value INT NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id, faction)
);

CREATE TABLE shared_player_stealth (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    level INT NOT NULL DEFAULT 50,
    disguise TEXT NOT NULL DEFAULT 'none',
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared_player_flags (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    flag TEXT NOT NULL,
    PRIMARY KEY (account_id, world_id, flag)
);

CREATE TABLE shared_player_actions (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    count INT NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared_quests (
    id TEXT NOT NULL,
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    obj_type TEXT NOT NULL,
    obj_target TEXT NOT NULL,
    obj_room TEXT,
    obj_count INT NOT NULL DEFAULT 1,
    obj_progress INT NOT NULL DEFAULT 0,
    reward_credits INT NOT NULL DEFAULT 0,
    reward_xp_skill TEXT,
    reward_xp_amount INT NOT NULL DEFAULT 0,
    reward_item_id TEXT,
    reward_item_name TEXT,
    reward_item_desc TEXT,
    giver_npc_id TEXT,
    accepted_at INT NOT NULL,
    next_quest_id TEXT NOT NULL DEFAULT '',
    PRIMARY KEY (id, account_id, world_id)
);

CREATE TABLE shared_world_events (
    id TEXT PRIMARY KEY,
    world_id TEXT NOT NULL,
    type TEXT NOT NULL,
    title TEXT NOT NULL,
    description TEXT,
    target_room TEXT NOT NULL,
    faction TEXT,
    payout_credits INT NOT NULL DEFAULT 0,
    payout_item_id TEXT,
    payout_item_name TEXT,
    payout_item_desc TEXT,
    status TEXT NOT NULL DEFAULT 'active',
    expires_actions INT NOT NULL DEFAULT 20,
    created_actions INT NOT NULL DEFAULT 0,
    created_at INT NOT NULL
);

CREATE TABLE shared_player_faction (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    faction_id TEXT NOT NULL,
    faction_name TEXT NOT NULL,
    agenda TEXT,
    hideout_room_id TEXT,
    credits INT NOT NULL DEFAULT 0,
    created_at INT NOT NULL,
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared_faction_members (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    npc_id TEXT PRIMARY KEY,
    npc_name TEXT NOT NULL,
    npc_desc TEXT,
    role TEXT NOT NULL DEFAULT 'associate',
    stationed_room TEXT,
    loyalty INT NOT NULL DEFAULT 50,
    recruited_at INT NOT NULL
);

CREATE TABLE shared_hideout_upgrades (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    upgrade_id TEXT NOT NULL,
    installed_at INT NOT NULL,
    PRIMARY KEY (account_id, world_id, upgrade_id)
);

CREATE TABLE shared_unlocked_recipes (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    recipe_id TEXT NOT NULL,
    unlocked_at INT NOT NULL,
    PRIMARY KEY (account_id, world_id, recipe_id)
);

CREATE TABLE shared_npc_memory (
    world_id TEXT NOT NULL,
    npc_id TEXT NOT NULL,
    action TEXT NOT NULL,
    ts INT NOT NULL,
    PRIMARY KEY (world_id, npc_id, action)
);

CREATE TABLE shared_generated_content (
    world_id TEXT NOT NULL,
    prompt_hash TEXT NOT NULL,
    type TEXT NOT NULL,
    yaml_blob TEXT NOT NULL,
    created_at INT NOT NULL,
    PRIMARY KEY (world_id, prompt_hash)
);

CREATE TABLE shared_weather_state (
    world_id TEXT NOT NULL,
    biome TEXT NOT NULL,
    condition TEXT NOT NULL DEFAULT 'clear',
    expires_action INT NOT NULL DEFAULT 0,
    PRIMARY KEY (world_id, biome)
);

CREATE TABLE shared_enchants (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    enchant_id TEXT NOT NULL,
    level INT NOT NULL DEFAULT 1,
    applied_at INT NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id, item_id, enchant_id)
);

CREATE TABLE shared_enchanting_xp (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    xp INT NOT NULL DEFAULT 0,
    level INT NOT NULL DEFAULT 1,
    PRIMARY KEY (account_id, world_id)
);

CREATE TABLE shared_crystal_shards (
    world_id TEXT NOT NULL,
    shard_id TEXT NOT NULL,
    biome TEXT NOT NULL,
    collected BOOLEAN NOT NULL DEFAULT FALSE,
    collected_at INT NOT NULL DEFAULT 0,
    PRIMARY KEY (world_id, shard_id)
);

CREATE TABLE shared_crops (
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    slot INT NOT NULL,
    seed_id TEXT NOT NULL,
    planted_at_action INT NOT NULL,
    ready_at_action INT NOT NULL,
    harvested BOOLEAN NOT NULL DEFAULT FALSE,
    PRIMARY KEY (world_id, room_id, slot)
);

CREATE TABLE shared_visited (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    PRIMARY KEY (account_id, world_id, room_id)
);

CREATE TABLE shared_player_augments (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    skill TEXT NOT NULL,
    bonus INT NOT NULL DEFAULT 0,
    installed_at INT NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id, skill)
);

CREATE TABLE shared_taken_room_items (
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    item_id TEXT NOT NULL,
    taken_by UUID,
    taken_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (world_id, room_id, item_id)
);

CREATE TABLE shared_item_mods (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    item_instance TEXT NOT NULL,
    mod_id TEXT NOT NULL,
    applied_at INT NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id, item_instance)
);

CREATE TABLE shared_bounties (
    world_id TEXT NOT NULL,
    room_id TEXT NOT NULL,
    npc_id TEXT NOT NULL,
    created_at INT NOT NULL,
    PRIMARY KEY (world_id, room_id)
);

CREATE TABLE shared_vuln_windows (
    world_id TEXT NOT NULL,
    system_id TEXT NOT NULL,
    bonus INT NOT NULL DEFAULT 0,
    expires_action INT NOT NULL DEFAULT 0,
    PRIMARY KEY (world_id, system_id)
);

CREATE TABLE shared_player_credits (
    account_id UUID NOT NULL,
    world_id TEXT NOT NULL,
    credits INT NOT NULL DEFAULT 0,
    PRIMARY KEY (account_id, world_id)
);
