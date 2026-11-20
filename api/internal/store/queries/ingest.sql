-- name: OperatorByKeyHash :one
SELECT code FROM operators WHERE api_key_hash = @hash;

-- name: SetOperatorKeyHash :execrows
UPDATE operators SET api_key_hash = @hash WHERE code = @code;

-- name: LockPlayer :exec
SELECT pg_advisory_xact_lock(hashtext(@player_id));

-- name: PlayerOwner :one
SELECT o.operator_code::text FROM (
    SELECT p.operator_code, 0 AS src FROM players p WHERE p.player_id = @player_id
    UNION ALL
    (SELECT e.operator_code, 1 AS src FROM events e WHERE e.player_id = @player_id LIMIT 1)
) o ORDER BY o.src, o.operator_code LIMIT 1;

-- name: InsertEvents :execrows
INSERT INTO events (operator_code, player_id, event_order, payload)
SELECT @operator_code, @player_id, (@orders::bigint[])[i], (@payloads::jsonb[])[i]
FROM generate_series(1, cardinality(@orders::bigint[])) AS i
ON CONFLICT DO NOTHING;

-- name: LastEventOrder :one
SELECT max(event_order)::bigint FROM events WHERE operator_code = @operator_code AND player_id = @player_id;
