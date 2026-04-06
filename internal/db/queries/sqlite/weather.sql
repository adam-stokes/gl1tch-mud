-- name: GetWeatherCondition :one
SELECT condition FROM weather_state WHERE biome = ?;

-- name: GetWeatherState :one
SELECT condition, expires_action FROM weather_state WHERE biome = ?;

-- name: UpsertWeatherState :exec
INSERT INTO weather_state (biome, condition, expires_action) VALUES (?, ?, ?)
ON CONFLICT(biome) DO UPDATE SET condition = excluded.condition, expires_action = excluded.expires_action;
