-- +goose Up
CREATE TABLE events (
    operator_code text        NOT NULL REFERENCES operators (code),
    player_id     text        NOT NULL,
    event_order   bigint      NOT NULL CHECK (event_order >= 1),
    payload       jsonb       NOT NULL,
    received_at   timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY (operator_code, player_id, event_order)
);
CREATE UNIQUE INDEX operators_api_key_hash ON operators (api_key_hash) WHERE api_key_hash IS NOT NULL;
CREATE INDEX events_player_id ON events (player_id);

-- +goose Down
DROP INDEX events_player_id;
DROP INDEX operators_api_key_hash;
DROP TABLE events;
