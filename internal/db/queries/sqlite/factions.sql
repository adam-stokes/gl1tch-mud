-- name: FactionExists :one
SELECT id FROM player_faction WHERE id = 1;

-- name: CreateFaction :exec
INSERT INTO player_faction (id, faction_id, faction_name, agenda, hideout_room_id, credits, created_at)
VALUES (1, ?, ?, ?, '', 0, ?);

-- name: GetFaction :one
SELECT faction_id, faction_name, agenda, hideout_room_id, credits, created_at
FROM player_faction WHERE id = 1;

-- name: SetFactionHideout :exec
UPDATE player_faction SET hideout_room_id = ? WHERE id = 1;

-- name: ListFactionMembers :many
SELECT npc_id, npc_name, npc_desc, role, stationed_room, loyalty, recruited_at
FROM faction_members;

-- name: InsertFactionMember :exec
INSERT INTO faction_members (npc_id, npc_name, npc_desc, role, stationed_room, loyalty, recruited_at)
VALUES (?, ?, ?, ?, '', 50, ?);

-- name: GetFactionMember :one
SELECT npc_id FROM faction_members WHERE npc_id = ?;

-- name: CountFactionMembers :one
SELECT COUNT(*) FROM faction_members;
