-- ============================================================
-- Shared World Queries (Postgres)
-- All queries prefixed with "Shared" to avoid SQLite conflicts.
-- ============================================================

-- ===================== Player State =====================

-- name: GetSharedPlayerState :one
SELECT room_id, hp, max_hp, credits
FROM shared_player_state
WHERE account_id = $1 AND world_id = $2;

-- name: UpsertSharedPlayerState :exec
INSERT INTO shared_player_state (account_id, world_id, room_id, hp, max_hp, credits)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (account_id, world_id)
DO UPDATE SET room_id = EXCLUDED.room_id, hp = EXCLUDED.hp, max_hp = EXCLUDED.max_hp, credits = EXCLUDED.credits;

-- ===================== Inventory =====================

-- name: ListSharedInventory :many
SELECT item_id, item_name, item_desc
FROM shared_inventory
WHERE account_id = $1 AND world_id = $2;

-- name: AddSharedItem :exec
INSERT INTO shared_inventory (account_id, world_id, item_id, item_name, item_desc)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (account_id, world_id, item_id) DO NOTHING;

-- name: RemoveSharedItem :execresult
DELETE FROM shared_inventory
WHERE account_id = $1 AND world_id = $2 AND item_id = $3;

-- name: RemoveOneSharedItem :exec
DELETE FROM shared_inventory
WHERE id = (
  SELECT si.id FROM shared_inventory si
  WHERE si.account_id = $1 AND si.world_id = $2 AND si.item_id = $3
  LIMIT 1
);

-- name: ClearSharedInventory :exec
DELETE FROM shared_inventory
WHERE account_id = $1 AND world_id = $2;

-- ===================== NPC State =====================

-- name: GetSharedNPCState :one
SELECT hp, alive, respawn_at
FROM shared_npc_state
WHERE world_id = $1 AND npc_id = $2;

-- name: UpsertSharedNPCDead :exec
INSERT INTO shared_npc_state (world_id, npc_id, room_id, hp, alive, respawn_at)
VALUES ($1, $2, $3, $4, FALSE, $5)
ON CONFLICT (world_id, npc_id)
DO UPDATE SET hp = EXCLUDED.hp, alive = FALSE, respawn_at = EXCLUDED.respawn_at;

-- name: UpsertSharedNPCAlive :exec
INSERT INTO shared_npc_state (world_id, npc_id, room_id, hp, alive, respawn_at)
VALUES ($1, $2, $3, $4, TRUE, NULL)
ON CONFLICT (world_id, npc_id)
DO UPDATE SET hp = EXCLUDED.hp, alive = TRUE, respawn_at = NULL;

-- name: RespawnExpiredNPCs :exec
UPDATE shared_npc_state
SET alive = TRUE, respawn_at = NULL
WHERE world_id = $1 AND alive = FALSE AND respawn_at IS NOT NULL AND respawn_at <= now();

-- ===================== Lock State =====================

-- name: GetSharedLockState :one
SELECT unlocked
FROM shared_lock_state
WHERE world_id = $1 AND lock_id = $2;

-- name: SetSharedLockUnlocked :exec
INSERT INTO shared_lock_state (world_id, lock_id, unlocked)
VALUES ($1, $2, TRUE)
ON CONFLICT (world_id, lock_id)
DO UPDATE SET unlocked = TRUE;

-- ===================== Resource State =====================

-- name: GetSharedResourceState :one
SELECT depleted, respawn_at
FROM shared_resources
WHERE world_id = $1 AND resource_id = $2;

-- name: DepleteSharedResource :exec
INSERT INTO shared_resources (world_id, resource_id, room_id, depleted, respawn_at)
VALUES ($1, $2, $3, TRUE, $4)
ON CONFLICT (world_id, resource_id)
DO UPDATE SET depleted = TRUE, respawn_at = EXCLUDED.respawn_at;

-- name: UndepleteSharedResource :exec
UPDATE shared_resources
SET depleted = FALSE, respawn_at = NULL
WHERE world_id = $1 AND resource_id = $2;

-- name: RespawnExpiredResources :exec
UPDATE shared_resources
SET depleted = FALSE, respawn_at = NULL
WHERE world_id = $1 AND depleted = TRUE AND respawn_at IS NOT NULL AND respawn_at <= now();

-- ===================== System State =====================

-- name: GetSharedSystemState :one
SELECT hacked, intrusion_level, alert_level
FROM shared_system_state
WHERE world_id = $1 AND system_id = $2;

-- name: MarkSharedSystemHacked :exec
INSERT INTO shared_system_state (world_id, system_id, hacked, intrusion_level, alert_level)
VALUES ($1, $2, TRUE, 0, 0)
ON CONFLICT (world_id, system_id)
DO UPDATE SET hacked = TRUE;

-- name: IncrementSharedAlert :exec
INSERT INTO shared_system_state (world_id, system_id, hacked, intrusion_level, alert_level)
VALUES ($1, $2, FALSE, 0, 1)
ON CONFLICT (world_id, system_id)
DO UPDATE SET alert_level = shared_system_state.alert_level + 1;

-- name: UpsertSharedSystemAlert :exec
INSERT INTO shared_system_state (world_id, system_id, hacked, intrusion_level, alert_level)
VALUES ($1, $2, FALSE, 0, $3)
ON CONFLICT (world_id, system_id)
DO UPDATE SET alert_level = EXCLUDED.alert_level;

-- ===================== Skills =====================

-- name: GetSharedSkill :one
SELECT level, xp
FROM shared_player_skills
WHERE account_id = $1 AND world_id = $2 AND skill = $3;

-- name: UpsertSharedSkill :exec
INSERT INTO shared_player_skills (account_id, world_id, skill, level, xp)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (account_id, world_id, skill)
DO UPDATE SET level = EXCLUDED.level, xp = EXCLUDED.xp;

-- name: ListSharedSkills :many
SELECT skill, level, xp
FROM shared_player_skills
WHERE account_id = $1 AND world_id = $2;

-- ===================== Credits =====================

-- name: GetSharedCredits :one
SELECT credits
FROM shared_player_credits
WHERE account_id = $1 AND world_id = $2;

-- name: AddSharedCredits :exec
INSERT INTO shared_player_credits (account_id, world_id, credits)
VALUES ($1, $2, $3)
ON CONFLICT (account_id, world_id)
DO UPDATE SET credits = shared_player_credits.credits + EXCLUDED.credits;

-- ===================== Equipped Armor =====================

-- name: GetSharedEquippedArmor :one
SELECT item_id, item_name, defense
FROM shared_equipped_armor
WHERE account_id = $1 AND world_id = $2;

-- name: UpsertSharedEquippedArmor :exec
INSERT INTO shared_equipped_armor (account_id, world_id, item_id, item_name, defense)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (account_id, world_id)
DO UPDATE SET item_id = EXCLUDED.item_id, item_name = EXCLUDED.item_name, defense = EXCLUDED.defense;

-- name: DeleteSharedEquippedArmor :exec
DELETE FROM shared_equipped_armor
WHERE account_id = $1 AND world_id = $2;

-- ===================== Death Pile =====================

-- name: InsertSharedDeathPile :exec
INSERT INTO shared_death_pile (world_id, room_id, item_id, item_name, item_desc)
VALUES ($1, $2, $3, $4, $5);

-- name: GetSharedDeathPile :many
SELECT item_id, item_name, item_desc
FROM shared_death_pile
WHERE world_id = $1 AND room_id = $2;

-- name: DeleteSharedDeathPile :exec
DELETE FROM shared_death_pile
WHERE world_id = $1 AND room_id = $2;

-- name: AnySharedDeathPile :one
SELECT room_id, COUNT(*) as count FROM shared_death_pile
WHERE world_id = $1
GROUP BY room_id ORDER BY MAX(dropped_at) DESC LIMIT 1;

-- ===================== Quests =====================

-- name: AcceptSharedQuest :exec
INSERT INTO shared_quests
 (id, account_id, world_id, title, description, status, obj_type, obj_target, obj_room,
  obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
  reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at,
  next_quest_id)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
ON CONFLICT (id, account_id, world_id) DO NOTHING;

-- name: GetSharedActiveQuests :many
SELECT id, title, description, status, obj_type, obj_target, obj_room,
       obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
       reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at,
       next_quest_id
FROM shared_quests
WHERE account_id = $1 AND world_id = $2 AND status = 'active';

-- name: ProgressSharedQuest :exec
UPDATE shared_quests
SET obj_progress = obj_progress + $1
WHERE id = $2 AND account_id = $3 AND world_id = $4 AND status = 'active';

-- name: CompleteSharedQuest :exec
UPDATE shared_quests SET status = 'completed'
WHERE id = $1 AND account_id = $2 AND world_id = $3;

-- name: GetSharedQuest :one
SELECT id, title, description, status, obj_type, obj_target, obj_room,
       obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
       reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at,
       next_quest_id
FROM shared_quests
WHERE id = $1 AND account_id = $2 AND world_id = $3;

-- name: FailSharedQuest :exec
UPDATE shared_quests SET status = 'failed'
WHERE id = $1 AND account_id = $2 AND world_id = $3;

-- name: ListSharedActiveQuestsByTypeTarget :many
SELECT id, title, description, status, obj_type, obj_target, obj_room,
       obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
       reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at,
       next_quest_id
FROM shared_quests
WHERE account_id = $1 AND world_id = $2 AND status = 'active' AND obj_type = $3 AND obj_target = $4;

-- name: ListSharedActiveQuestIDs :many
SELECT id FROM shared_quests
WHERE account_id = $1 AND world_id = $2 AND status = 'active';

-- ===================== Factions =====================

-- name: CreateSharedFaction :exec
INSERT INTO shared_player_faction (account_id, world_id, faction_id, faction_name, agenda, hideout_room_id, credits, created_at)
VALUES ($1, $2, $3, $4, $5, '', 0, $6);

-- name: GetSharedFaction :one
SELECT faction_id, faction_name, agenda, hideout_room_id, credits, created_at
FROM shared_player_faction
WHERE account_id = $1 AND world_id = $2;

-- name: SharedFactionExists :one
SELECT account_id FROM shared_player_faction
WHERE account_id = $1 AND world_id = $2;

-- name: SetSharedHideout :exec
UPDATE shared_player_faction SET hideout_room_id = $1
WHERE account_id = $2 AND world_id = $3;

-- name: RecruitSharedMember :exec
INSERT INTO shared_faction_members (account_id, world_id, npc_id, npc_name, npc_desc, role, stationed_room, loyalty, recruited_at)
VALUES ($1, $2, $3, $4, $5, $6, '', 50, $7);

-- name: GetSharedMembers :many
SELECT npc_id, npc_name, npc_desc, role, stationed_room, loyalty, recruited_at
FROM shared_faction_members
WHERE account_id = $1 AND world_id = $2;

-- name: IsSharedRecruited :one
SELECT npc_id FROM shared_faction_members
WHERE npc_id = $1;

-- name: SharedMemberCount :one
SELECT COUNT(*) FROM shared_faction_members
WHERE account_id = $1 AND world_id = $2;

-- name: UpdateSharedFactionMemberStation :exec
UPDATE shared_faction_members SET stationed_room = $1
WHERE npc_id = $2;

-- ===================== Building / Base =====================

-- name: InsertSharedBuild :exec
INSERT INTO shared_builds (world_id, room_id, build_id, placed_by)
VALUES ($1, $2, $3, $4);

-- name: CountSharedBuilds :one
SELECT COUNT(*) FROM shared_builds
WHERE world_id = $1 AND room_id = $2;

-- name: CountSharedBuildsByType :one
SELECT COUNT(*) FROM shared_builds
WHERE world_id = $1 AND room_id = $2 AND build_id = $3;

-- name: ListSharedBuilds :many
SELECT build_id FROM shared_builds
WHERE world_id = $1 AND room_id = $2
ORDER BY placed_at;

-- name: CountSharedChestInRoom :one
SELECT COUNT(*) FROM shared_builds
WHERE world_id = $1 AND room_id = $2 AND build_id = 'chest';

-- name: CountSharedEnchantingTable :one
SELECT COUNT(*) FROM shared_builds
WHERE world_id = $1 AND room_id = $2 AND build_id = 'enchanting-table';

-- ===================== Chests =====================

-- name: InsertSharedChest :exec
INSERT INTO shared_chests (world_id, room_id, item_id, item_name, stored_by)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT DO NOTHING;

-- name: GetSharedChestItems :many
SELECT item_id, item_name
FROM shared_chests
WHERE world_id = $1 AND room_id = $2;

-- name: DeleteSharedChestItem :exec
DELETE FROM shared_chests
WHERE world_id = $1 AND room_id = $2 AND item_id = $3;

-- name: CountSharedChestItems :one
SELECT COUNT(*) FROM shared_chests
WHERE world_id = $1 AND room_id = $2;

-- name: ListSharedRandomChestItems :many
SELECT item_id, item_name FROM shared_chests
WHERE world_id = $1 AND room_id = $2
ORDER BY RANDOM() LIMIT $3;

-- ===================== Arena =====================

-- name: StartSharedArena :exec
INSERT INTO shared_arena_sessions (account_id, world_id, game_type, wave, enemies)
VALUES ($1, $2, $3, $4, $5);

-- name: GetSharedActiveArena :one
SELECT id, game_type, wave, enemies, started_at
FROM shared_arena_sessions
WHERE account_id = $1 AND world_id = $2
ORDER BY started_at DESC
LIMIT 1;

-- name: QuitSharedArena :exec
DELETE FROM shared_arena_sessions
WHERE account_id = $1 AND world_id = $2;

-- name: SaveSharedArena :exec
UPDATE shared_arena_sessions
SET wave = $1, enemies = $2
WHERE id = $3;

-- ===================== Enchanting =====================

-- name: ApplySharedEnchant :exec
INSERT INTO shared_enchants (account_id, world_id, item_id, enchant_id, level, applied_at)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (account_id, world_id, item_id, enchant_id)
DO UPDATE SET level = EXCLUDED.level, applied_at = EXCLUDED.applied_at;

-- name: ListSharedEnchants :many
SELECT item_id, enchant_id, level
FROM shared_enchants
WHERE account_id = $1 AND world_id = $2 AND item_id = $3;

-- name: AddSharedEnchantingXP :exec
INSERT INTO shared_enchanting_xp (account_id, world_id, xp, level)
VALUES ($1, $2, $3, 1)
ON CONFLICT (account_id, world_id)
DO UPDATE SET xp = shared_enchanting_xp.xp + EXCLUDED.xp,
             level = LEAST(GREATEST(1, (shared_enchanting_xp.xp + EXCLUDED.xp) / 100), 30);

-- name: GetSharedEnchantingXP :one
SELECT xp, level
FROM shared_enchanting_xp
WHERE account_id = $1 AND world_id = $2;

-- name: DeductSharedEnchantingXP :exec
UPDATE shared_enchanting_xp SET xp = xp - $1
WHERE account_id = $2 AND world_id = $3;

-- ===================== Weather =====================

-- name: GetSharedWeather :one
SELECT condition, expires_action
FROM shared_weather_state
WHERE world_id = $1 AND biome = $2;

-- name: GetSharedWeatherCondition :one
SELECT condition
FROM shared_weather_state
WHERE world_id = $1 AND biome = $2;

-- name: UpsertSharedWeather :exec
INSERT INTO shared_weather_state (world_id, biome, condition, expires_action)
VALUES ($1, $2, $3, $4)
ON CONFLICT (world_id, biome)
DO UPDATE SET condition = EXCLUDED.condition, expires_action = EXCLUDED.expires_action;

-- ===================== Mining / Actions =====================

-- name: BumpSharedActions :exec
INSERT INTO shared_player_actions (account_id, world_id, count)
VALUES ($1, $2, 1)
ON CONFLICT (account_id, world_id)
DO UPDATE SET count = shared_player_actions.count + 1;

-- name: GetSharedActionCount :one
SELECT count
FROM shared_player_actions
WHERE account_id = $1 AND world_id = $2;

-- ===================== Farming / Crops =====================

-- name: GetSharedReadyCrops :many
SELECT seed_id
FROM shared_crops
WHERE world_id = $1 AND room_id = $2 AND ready_at_action <= $3 AND harvested = FALSE;

-- name: CountSharedReadyCrops :one
SELECT COUNT(*) FROM shared_crops
WHERE world_id = $1 AND room_id = $2 AND seed_id = $3 AND ready_at_action <= $4 AND harvested = FALSE;

-- name: HarvestSharedCrops :exec
UPDATE shared_crops SET harvested = TRUE
WHERE world_id = $1 AND room_id = $2 AND seed_id = $3 AND ready_at_action <= $4 AND harvested = FALSE;

-- name: PlantSharedCrop :exec
INSERT INTO shared_crops (world_id, room_id, slot, seed_id, planted_at_action, ready_at_action)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: GetSharedUsedSlots :many
SELECT slot FROM shared_crops
WHERE world_id = $1 AND room_id = $2 AND harvested = FALSE;

-- ===================== Visited =====================

-- name: MarkSharedVisited :exec
INSERT INTO shared_visited (account_id, world_id, room_id)
VALUES ($1, $2, $3)
ON CONFLICT (account_id, world_id, room_id) DO NOTHING;

-- name: HasSharedVisited :one
SELECT room_id FROM shared_visited
WHERE account_id = $1 AND world_id = $2 AND room_id = $3;

-- ===================== Stealth =====================

-- name: UpsertSharedStealth :exec
INSERT INTO shared_player_stealth (account_id, world_id, level, disguise)
VALUES ($1, $2, $3, $4)
ON CONFLICT (account_id, world_id)
DO UPDATE SET level = EXCLUDED.level, disguise = EXCLUDED.disguise;

-- name: GetSharedStealth :one
SELECT level, disguise
FROM shared_player_stealth
WHERE account_id = $1 AND world_id = $2;

-- ===================== Player Flags =====================

-- name: SetSharedPlayerFlag :exec
INSERT INTO shared_player_flags (account_id, world_id, flag)
VALUES ($1, $2, $3)
ON CONFLICT (account_id, world_id, flag) DO NOTHING;

-- name: HasSharedPlayerFlag :one
SELECT COUNT(*) FROM shared_player_flags
WHERE account_id = $1 AND world_id = $2 AND flag = $3;

-- ===================== Crystal Shards =====================

-- name: SeedSharedCrystalShard :exec
INSERT INTO shared_crystal_shards (world_id, shard_id, biome, collected, collected_at)
VALUES ($1, $2, $3, FALSE, 0)
ON CONFLICT (world_id, shard_id) DO NOTHING;

-- name: MarkSharedShardCollected :exec
UPDATE shared_crystal_shards SET collected = TRUE, collected_at = $1
WHERE world_id = $2 AND shard_id = $3;

-- name: CountSharedCollectedShards :one
SELECT COUNT(*) FROM shared_crystal_shards
WHERE world_id = $1 AND collected = TRUE;

-- name: CountSharedTotalShards :one
SELECT COUNT(*) FROM shared_crystal_shards
WHERE world_id = $1;

-- ===================== Recipes =====================

-- name: UnlockSharedRecipe :exec
INSERT INTO shared_unlocked_recipes (account_id, world_id, recipe_id, unlocked_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (account_id, world_id, recipe_id) DO NOTHING;

-- name: IsSharedRecipeUnlocked :one
SELECT COUNT(*) FROM shared_unlocked_recipes
WHERE account_id = $1 AND world_id = $2 AND recipe_id = $3;

-- ===================== Reputation =====================

-- name: GetSharedReputation :one
SELECT value FROM shared_player_reputation
WHERE account_id = $1 AND world_id = $2 AND faction = $3;

-- name: UpsertSharedReputation :exec
INSERT INTO shared_player_reputation (account_id, world_id, faction, value)
VALUES ($1, $2, $3, $4)
ON CONFLICT (account_id, world_id, faction)
DO UPDATE SET value = EXCLUDED.value;

-- name: ListSharedReputations :many
SELECT faction, value FROM shared_player_reputation
WHERE account_id = $1 AND world_id = $2;

-- name: ListSharedHighRepFactions :many
SELECT faction, value FROM shared_player_reputation
WHERE account_id = $1 AND world_id = $2 AND value >= 3;

-- ===================== NPC Memory =====================

-- name: UpsertSharedNPCMemory :exec
INSERT INTO shared_npc_memory (world_id, npc_id, action, ts)
VALUES ($1, $2, $3, $4)
ON CONFLICT (world_id, npc_id, action)
DO UPDATE SET ts = EXCLUDED.ts;

-- name: GetSharedNPCMemory :one
SELECT ts FROM shared_npc_memory
WHERE world_id = $1 AND npc_id = $2 AND action = $3;

-- ===================== World Events =====================

-- name: InsertSharedWorldEvent :exec
INSERT INTO shared_world_events
 (id, world_id, type, title, description, target_room, faction,
  payout_credits, payout_item_id, payout_item_name, payout_item_desc,
  status, expires_actions, created_actions, created_at)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15);

-- name: CountSharedActiveBaseRaids :one
SELECT COUNT(*) FROM shared_world_events
WHERE world_id = $1 AND type = 'base-raid' AND target_room = $2 AND status = 'active';

-- name: ListSharedExpiredBaseRaids :many
SELECT id FROM shared_world_events
WHERE world_id = $1 AND type = 'base-raid' AND target_room = $2 AND status = 'active'
AND (created_actions + expires_actions) <= $3;

-- name: ListSharedActiveEvents :many
SELECT id, type, title, description, target_room, faction,
       payout_credits, payout_item_id, payout_item_name, payout_item_desc,
       status, expires_actions, created_actions, created_at
FROM shared_world_events WHERE world_id = $1 AND status = 'active';

-- name: GetSharedEvent :one
SELECT id, type, title, description, target_room, faction,
       payout_credits, payout_item_id, payout_item_name, payout_item_desc,
       status, expires_actions, created_actions, created_at
FROM shared_world_events WHERE id = $1 AND world_id = $2;

-- name: CompleteSharedEvent :exec
UPDATE shared_world_events SET status = 'completed' WHERE id = $1 AND world_id = $2;

-- name: ExpireSharedOldEvents :execresult
UPDATE shared_world_events SET status = 'expired'
WHERE world_id = $1 AND status = 'active' AND (created_actions + expires_actions) <= $2;

-- name: ResolveSharedWorldEvent :exec
UPDATE shared_world_events SET status = 'resolved'
WHERE id = $1;

-- ===================== Bounties =====================

-- name: InsertSharedBounty :exec
INSERT INTO shared_bounties (world_id, room_id, npc_id, created_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (world_id, room_id)
DO UPDATE SET npc_id = EXCLUDED.npc_id, created_at = EXCLUDED.created_at;

-- name: GetSharedBounty :one
SELECT npc_id, created_at FROM shared_bounties
WHERE world_id = $1 AND room_id = $2;

-- ===================== Vuln Windows =====================

-- name: SetSharedVulnWindow :exec
INSERT INTO shared_vuln_windows (world_id, system_id, bonus, expires_action)
VALUES ($1, $2, $3, $4)
ON CONFLICT (world_id, system_id)
DO UPDATE SET bonus = EXCLUDED.bonus, expires_action = EXCLUDED.expires_action;

-- name: GetSharedVulnWindow :one
SELECT bonus, expires_action FROM shared_vuln_windows
WHERE world_id = $1 AND system_id = $2;

-- name: DeleteSharedVulnWindow :exec
DELETE FROM shared_vuln_windows
WHERE world_id = $1 AND system_id = $2;

-- ===================== Generated Content =====================

-- name: GetSharedGeneratedContent :one
SELECT type, yaml_blob, created_at
FROM shared_generated_content
WHERE world_id = $1 AND prompt_hash = $2;

-- name: UpsertSharedGeneratedContent :exec
INSERT INTO shared_generated_content (world_id, prompt_hash, type, yaml_blob, created_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (world_id, prompt_hash)
DO UPDATE SET type = EXCLUDED.type, yaml_blob = EXCLUDED.yaml_blob, created_at = EXCLUDED.created_at;

-- ===================== Hideout Upgrades =====================

-- name: InsertSharedHideoutUpgrade :exec
INSERT INTO shared_hideout_upgrades (account_id, world_id, upgrade_id, installed_at)
VALUES ($1, $2, $3, $4)
ON CONFLICT (account_id, world_id, upgrade_id) DO NOTHING;

-- name: ListSharedHideoutUpgrades :many
SELECT upgrade_id, installed_at
FROM shared_hideout_upgrades
WHERE account_id = $1 AND world_id = $2;

-- ===================== Augments =====================

-- name: UpsertSharedAugment :exec
INSERT INTO shared_player_augments (account_id, world_id, skill, bonus, installed_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (account_id, world_id, skill)
DO UPDATE SET bonus = EXCLUDED.bonus, installed_at = EXCLUDED.installed_at;

-- name: GetSharedAugment :one
SELECT bonus, installed_at
FROM shared_player_augments
WHERE account_id = $1 AND world_id = $2 AND skill = $3;

-- name: ListSharedAugments :many
SELECT skill, bonus, installed_at
FROM shared_player_augments
WHERE account_id = $1 AND world_id = $2;

-- ===================== Taken Room Items =====================

-- name: TakeSharedRoomItem :exec
INSERT INTO shared_taken_room_items (world_id, room_id, item_id, taken_by)
VALUES ($1, $2, $3, $4)
ON CONFLICT (world_id, room_id, item_id) DO NOTHING;

-- name: IsSharedRoomItemTaken :one
SELECT item_id FROM shared_taken_room_items
WHERE world_id = $1 AND room_id = $2 AND item_id = $3;

-- name: ListSharedTakenRoomItems :many
SELECT item_id FROM shared_taken_room_items
WHERE world_id = $1 AND room_id = $2;

-- ===================== Item Mods =====================

-- name: UpsertSharedItemMod :exec
INSERT INTO shared_item_mods (account_id, world_id, item_instance, mod_id, applied_at)
VALUES ($1, $2, $3, $4, $5)
ON CONFLICT (account_id, world_id, item_instance)
DO UPDATE SET mod_id = EXCLUDED.mod_id, applied_at = EXCLUDED.applied_at;

-- name: GetSharedItemMod :one
SELECT mod_id, applied_at
FROM shared_item_mods
WHERE account_id = $1 AND world_id = $2 AND item_instance = $3;
