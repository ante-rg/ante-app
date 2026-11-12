-- name: GetActiveModel :one
SELECT model_version, card, loaded_at FROM models WHERE is_active;
