-- name: GetCredits :one
SELECT credits FROM player_credits WHERE id = 1;

-- name: UpsertCredits :exec
INSERT INTO player_credits (id, credits) VALUES (1, ?)
ON CONFLICT(id) DO UPDATE SET credits = credits + excluded.credits;
