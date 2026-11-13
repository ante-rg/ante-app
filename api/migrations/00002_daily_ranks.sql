-- +goose Up
CREATE TABLE daily_ranks (
    model_version text    NOT NULL REFERENCES models (model_version) ON DELETE CASCADE,
    day           integer NOT NULL CHECK (day >= 0),
    player_id     text    NOT NULL REFERENCES players (player_id) ON DELETE CASCADE,
    score         real,
    rank          integer,
    band          text    NOT NULL CHECK (band IN ('high', 'elevated', 'low', 'insufficient_data')),
    PRIMARY KEY (model_version, day, player_id)
);
CREATE INDEX daily_ranks_by_rank ON daily_ranks (model_version, day, rank);
CREATE INDEX daily_ranks_high ON daily_ranks (model_version, player_id, day) WHERE band = 'high';

-- +goose Down
DROP TABLE daily_ranks;
