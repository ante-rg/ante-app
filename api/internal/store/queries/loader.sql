-- name: UpsertOperator :exec
INSERT INTO operators (code, name) VALUES (@code, @code) ON CONFLICT DO NOTHING;

-- name: UpsertPlayer :exec
INSERT INTO players (player_id, operator_code, last_day, session_count, deposit_count)
VALUES (@player_id, @operator_code, @last_day, @session_count, @deposit_count)
ON CONFLICT (player_id) DO UPDATE SET
    operator_code = EXCLUDED.operator_code,
    last_day = EXCLUDED.last_day,
    session_count = EXCLUDED.session_count,
    deposit_count = EXCLUDED.deposit_count;

-- name: DeleteActivityForPlayers :exec
DELETE FROM daily_activity WHERE player_id = ANY(@ids::text[]);

-- name: DeleteSessionsForPlayers :exec
DELETE FROM sessions WHERE player_id = ANY(@ids::text[]);

-- name: DeleteRuleFlagsForPlayers :exec
DELETE FROM rule_flags WHERE player_id = ANY(@ids::text[]);

-- name: DeleteModel :exec
DELETE FROM models WHERE model_version = @model_version;

-- name: DeactivateModels :exec
UPDATE models SET is_active = false WHERE is_active;

-- name: InsertActiveModel :exec
INSERT INTO models (model_version, card, is_active) VALUES (@model_version, @card, true);
