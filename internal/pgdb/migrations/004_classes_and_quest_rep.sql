-- 004_classes_and_quest_rep.sql
-- Adds character class field to players and rep-gating fields to quests.

ALTER TABLE shared_player_state
    ADD COLUMN IF NOT EXISTS class TEXT NOT NULL DEFAULT '';

ALTER TABLE shared_quests
    ADD COLUMN IF NOT EXISTS giver_faction      TEXT NOT NULL DEFAULT '';
ALTER TABLE shared_quests
    ADD COLUMN IF NOT EXISTS min_rep            INT  NOT NULL DEFAULT 0;
ALTER TABLE shared_quests
    ADD COLUMN IF NOT EXISTS reward_rep_faction TEXT NOT NULL DEFAULT '';
ALTER TABLE shared_quests
    ADD COLUMN IF NOT EXISTS reward_rep_delta   INT  NOT NULL DEFAULT 0;
