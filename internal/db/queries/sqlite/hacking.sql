-- name: GetSystemHacked :one
SELECT hacked FROM system_state WHERE room_id = ? AND system_id = ?;

-- name: GetSystemAlert :one
SELECT alert FROM system_state WHERE room_id = ? AND system_id = ?;

-- name: MarkSystemHacked :exec
INSERT INTO system_state (room_id, system_id, hacked, alert) VALUES (?, ?, 1, 0)
ON CONFLICT(room_id, system_id) DO UPDATE SET hacked = 1;

-- name: IncrementSystemAlert :exec
INSERT INTO system_state (room_id, system_id, alert, hacked) VALUES (?, ?, 1, 0)
ON CONFLICT(room_id, system_id) DO UPDATE SET alert = alert + 1;

-- name: GetSystemAlertForHackMulti :one
SELECT alert FROM system_state WHERE room_id = ? AND system_id = ?;

-- name: UpsertSystemAlert :exec
INSERT INTO system_state (room_id, system_id, intrusion, alert) VALUES (?, ?, 0, ?)
ON CONFLICT(room_id, system_id) DO UPDATE SET alert = excluded.alert;

-- name: InsertBounty :exec
INSERT OR REPLACE INTO bounties (room_id, npc_id, created_at) VALUES (?, ?, ?);

-- name: SetVulnWindow :exec
INSERT OR REPLACE INTO vuln_windows (system_id, bonus, expires_action) VALUES (?, ?, ?);

-- name: GetVulnWindow :one
SELECT bonus, expires_action FROM vuln_windows WHERE system_id = ?;

-- name: DeleteVulnWindow :exec
DELETE FROM vuln_windows WHERE system_id = ?;
