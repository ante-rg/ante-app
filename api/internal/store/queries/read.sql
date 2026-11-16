-- name: GetMaxRankDay :one
SELECT count(*) AS ranks, coalesce(max(day), 0)::int AS max_day FROM daily_ranks WHERE model_version = @model_version;

-- name: ListQueue :many
SELECT r.player_id, p.operator_code, p.last_day, r.rank, r.score, r.band,
       (SELECT s.reasons FROM daily_scores s
         WHERE s.model_version = r.model_version AND s.player_id = r.player_id
           AND s.day = least(r.day, p.last_day))::jsonb AS reasons,
       EXISTS (SELECT 1 FROM rule_flags f
                WHERE f.player_id = r.player_id AND f.day <= least(r.day, p.last_day)) AS flagged,
       to_jsonb(ARRAY(SELECT s.score FROM generate_series(least(r.day, p.last_day) - 29, least(r.day, p.last_day)) AS d
                      LEFT JOIN daily_scores s ON s.model_version = r.model_version
                                              AND s.player_id = r.player_id AND s.day = d
                      ORDER BY d))::jsonb AS trend
FROM daily_ranks r
JOIN players p ON p.player_id = r.player_id
WHERE r.model_version = @model_version AND r.day = @day
  AND r.band = ANY(@bands::text[])
  AND (sqlc.narg(operator)::text IS NULL OR p.operator_code = sqlc.narg(operator)::text)
ORDER BY r.rank NULLS LAST, r.player_id
LIMIT @row_limit;

-- name: QueueCounts :one
SELECT (count(*) FILTER (WHERE r.band = 'high'))::int AS high,
       (count(*) FILTER (WHERE r.band = 'elevated'))::int AS elevated,
       (count(*) FILTER (WHERE r.band = 'low'))::int AS low,
       (count(*) FILTER (WHERE r.band = 'insufficient_data'))::int AS insufficient_data,
       (count(*) FILTER (WHERE r.band = 'high' AND r.day > 0 AND NOT EXISTS (
            SELECT 1 FROM daily_ranks q
             WHERE q.model_version = r.model_version AND q.day = r.day - 1
               AND q.player_id = r.player_id AND q.band = 'high')))::int AS new_high
FROM daily_ranks r
WHERE r.model_version = @model_version AND r.day = @day;

-- name: GetPlayerCase :one
SELECT p.player_id, p.operator_code, p.last_day, r.score, r.rank, r.band,
       (SELECT count(x.rank) FROM daily_ranks x WHERE x.model_version = r.model_version AND x.day = r.day)::int AS rank_of,
       (SELECT s.reasons FROM daily_scores s
         WHERE s.model_version = r.model_version AND s.player_id = p.player_id
           AND s.day = least(r.day, p.last_day))::jsonb AS reasons,
       coalesce((SELECT min(h.day) FROM daily_ranks h
         WHERE h.model_version = r.model_version AND h.player_id = p.player_id AND h.band = 'high'
           AND h.day <= least(r.day, p.last_day)), -1)::int AS first_high_day,
       coalesce((SELECT min(f.day) FROM rule_flags f
         WHERE f.player_id = p.player_id AND f.day <= least(r.day, p.last_day)), -1)::int AS first_rule_flag_day,
       (SELECT count(*) FROM sessions s
         WHERE s.player_id = p.player_id AND s.start_day <= least(r.day, p.last_day))::int AS sessions_to_date,
       (SELECT coalesce(sum(s.deposits_n), 0) FROM sessions s
         WHERE s.player_id = p.player_id AND s.start_day <= least(r.day, p.last_day))::int AS deposits_to_date,
       (SELECT coalesce(sum(s.deposits_eur), 0) FROM sessions s
         WHERE s.player_id = p.player_id AND s.start_day <= least(r.day, p.last_day))::float8 AS deposits_eur_to_date
FROM daily_ranks r
JOIN players p ON p.player_id = r.player_id
WHERE r.model_version = @model_version AND r.day = @day AND r.player_id = @player_id;

-- name: ListPlayerRuleFlags :many
SELECT f.rule_id, min(f.rule_text)::text AS rule_text, min(f.day)::int AS first_day
FROM rule_flags f
WHERE f.player_id = @player_id AND f.day <= @effective_day
GROUP BY f.rule_id
ORDER BY min(f.day), f.rule_id;

-- name: ListTimelineDays :many
SELECT a.day, a.deposits_n, a.deposits_eur, a.sessions_n, a.play_minutes, a.net_result_eur,
       a.chase_sessions_n, a.night_sessions_n, a.end_balance_eur, s.score, r.band AS band
FROM daily_activity a
LEFT JOIN daily_scores s ON s.model_version = @model_version AND s.player_id = a.player_id AND s.day = a.day
LEFT JOIN daily_ranks r ON r.model_version = @model_version AND r.player_id = a.player_id AND r.day = a.day
WHERE a.player_id = @player_id AND a.day <= @effective_day
ORDER BY a.day;
