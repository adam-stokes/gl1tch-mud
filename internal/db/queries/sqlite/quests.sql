-- name: AcceptQuest :exec
INSERT OR IGNORE INTO quests
 (id, title, description, status, obj_type, obj_target, obj_room,
  obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
  reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at,
  next_quest_id)
 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?);

-- name: ListActiveQuests :many
SELECT id, title, description, status, obj_type, obj_target, obj_room,
       obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
       reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at,
       next_quest_id
FROM quests WHERE status = 'active';

-- name: ProgressQuest :exec
UPDATE quests SET obj_progress = obj_progress + ? WHERE id = ? AND status = 'active';

-- name: CompleteQuest :exec
UPDATE quests SET status = 'completed' WHERE id = ?;

-- name: FailQuest :exec
UPDATE quests SET status = 'failed' WHERE id = ?;

-- name: GetQuest :one
SELECT id, title, description, status, obj_type, obj_target, obj_room,
       obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
       reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at,
       next_quest_id
FROM quests WHERE id = ?;

-- name: ListActiveQuestsByTypeTarget :many
SELECT id, title, description, status, obj_type, obj_target, obj_room,
       obj_count, obj_progress, reward_credits, reward_xp_skill, reward_xp_amount,
       reward_item_id, reward_item_name, reward_item_desc, giver_npc_id, accepted_at,
       next_quest_id
FROM quests WHERE status = 'active' AND obj_type = ? AND obj_target = ?;

-- name: ListActiveQuestIDs :many
SELECT id FROM quests WHERE status = 'active';
