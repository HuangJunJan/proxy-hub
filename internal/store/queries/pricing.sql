-- model_pricing queries. ASCII-only comments.

-- name: ListModelPricing :many
SELECT * FROM model_pricing ORDER BY model_id;

-- name: GetModelPricing :one
SELECT * FROM model_pricing WHERE model_id = ?;

-- name: UpsertModelPricing :exec
-- Admin override: replaces values and marks source.
INSERT INTO model_pricing (
    model_id, input_per_million, output_per_million,
    cache_read_per_million, cache_creation_per_million, source, updated_at
) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(model_id) DO UPDATE SET
    input_per_million = excluded.input_per_million,
    output_per_million = excluded.output_per_million,
    cache_read_per_million = excluded.cache_read_per_million,
    cache_creation_per_million = excluded.cache_creation_per_million,
    source = excluded.source,
    updated_at = excluded.updated_at;

-- name: SeedModelPricing :exec
-- Seed insert: only when absent; never overwrites an admin-edited row.
INSERT INTO model_pricing (
    model_id, input_per_million, output_per_million,
    cache_read_per_million, cache_creation_per_million, source, updated_at
) VALUES (?, ?, ?, ?, ?, 'seed', ?)
ON CONFLICT(model_id) DO NOTHING;

-- name: DeleteModelPricing :exec
DELETE FROM model_pricing WHERE model_id = ?;
