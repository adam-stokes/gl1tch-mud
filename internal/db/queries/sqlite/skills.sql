-- name: LoadSkill :one
SELECT level, xp FROM player_skills WHERE skill = ?;

-- name: UpsertSkill :exec
INSERT INTO player_skills (skill, level, xp) VALUES (?, ?, ?)
ON CONFLICT(skill) DO UPDATE SET level = excluded.level, xp = excluded.xp;

-- name: ListAllSkills :many
SELECT skill, level, xp FROM player_skills;
