-- +goose Up
CREATE TABLE reviews (
    id            bigserial   PRIMARY KEY,
    player_id     text        NOT NULL,
    as_of_day     integer     NOT NULL CHECK (as_of_day >= 0),
    outcome       text        NOT NULL CHECK (outcome IN ('intervention_needed', 'no_concern', 'not_enough_data')),
    note          text,
    reviewer      text        NOT NULL,
    blind         boolean     NOT NULL,
    model_version text        NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX reviews_player_day ON reviews (player_id, as_of_day, id);

-- +goose Down
DROP TABLE reviews;
