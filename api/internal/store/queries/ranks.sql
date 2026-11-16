-- name: FillDailyRanks :execrows
WITH pl AS (
    SELECT p.player_id, p.last_day FROM players p
    WHERE EXISTS (SELECT 1 FROM daily_scores s
                  WHERE s.model_version = sqlc.arg(model_version)::text AND s.player_id = p.player_id)
),
days AS (
    SELECT generate_series(0, max(last_day)) AS day FROM pl
),
at AS (
    SELECT d.day, pl.player_id, s.score
    FROM days d
    CROSS JOIN pl
    JOIN daily_scores s ON s.model_version = sqlc.arg(model_version)::text
        AND s.player_id = pl.player_id AND s.day = least(d.day, pl.last_day)
),
ranked AS (
    SELECT day, player_id, score,
           CASE WHEN score IS NULL THEN NULL
                ELSE row_number() OVER (PARTITION BY day, score IS NULL ORDER BY score DESC, player_id)
           END AS rank,
           count(score) OVER (PARTITION BY day) AS k
    FROM at
)
INSERT INTO daily_ranks (model_version, day, player_id, score, rank, band)
SELECT sqlc.arg(model_version)::text, day, player_id, score, rank::integer,
       CASE WHEN rank IS NULL THEN 'insufficient_data'
            WHEN rank <= ceil(k * 0.1) THEN 'high'
            WHEN rank <= ceil(k * 0.2) THEN 'elevated'
            ELSE 'low' END
FROM ranked;
