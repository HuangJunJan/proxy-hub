-- Abilities (derived routing index). On channel save, in one tx:
-- DeleteAbilitiesForChannel then CreateAbility per row (incremental, never TRUNCATE).
-- On startup, ListEnabledAbilities builds the in-memory RouteIndex.
-- ASCII comments on purpose (sqlc CJK parser limitation).

-- name: DeleteAbilitiesForChannel :exec
DELETE FROM abilities WHERE channel_id = ?;

-- name: CreateAbility :exec
INSERT INTO abilities (
    group_name, alias_model, channel_id, upstream_model, priority, weight, enabled
) VALUES (?, ?, ?, ?, ?, ?, ?);

-- name: ListEnabledAbilities :many
SELECT * FROM abilities WHERE enabled = 1;

-- name: ListAbilitiesForChannel :many
SELECT * FROM abilities WHERE channel_id = ?;
