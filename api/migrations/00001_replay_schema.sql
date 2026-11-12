-- +goose Up
CREATE TABLE operators (
    code         text PRIMARY KEY,
    name         text NOT NULL,
    api_key_hash text
);

CREATE TABLE players (
    player_id     text PRIMARY KEY,
    operator_code text    NOT NULL REFERENCES operators (code),
    last_day      integer NOT NULL CHECK (last_day >= 0),
    session_count integer NOT NULL,
    deposit_count integer NOT NULL
);

CREATE TABLE models (
    model_version text PRIMARY KEY,
    card          jsonb       NOT NULL,
    loaded_at     timestamptz NOT NULL DEFAULT now(),
    is_active     boolean     NOT NULL DEFAULT false
);
CREATE UNIQUE INDEX models_one_active ON models (is_active) WHERE is_active;

CREATE TABLE daily_scores (
    model_version text    NOT NULL REFERENCES models (model_version) ON DELETE CASCADE,
    player_id     text    NOT NULL REFERENCES players (player_id) ON DELETE CASCADE,
    day           integer NOT NULL CHECK (day >= 0),
    score         real CHECK (score IS NULL OR score BETWEEN 0 AND 1),
    reasons       jsonb   NOT NULL DEFAULT '[]',
    PRIMARY KEY (player_id, model_version, day)
);

CREATE TABLE daily_activity (
    player_id        text             NOT NULL REFERENCES players (player_id) ON DELETE CASCADE,
    day              integer          NOT NULL CHECK (day >= 0),
    deposits_n       integer          NOT NULL,
    deposits_eur     double precision NOT NULL,
    sessions_n       integer          NOT NULL,
    play_minutes     double precision NOT NULL,
    net_result_eur   double precision NOT NULL,
    chase_sessions_n integer          NOT NULL,
    night_sessions_n integer          NOT NULL,
    end_balance_eur  double precision,
    PRIMARY KEY (player_id, day)
);

CREATE TABLE sessions (
    player_id      text             NOT NULL REFERENCES players (player_id) ON DELETE CASCADE,
    session_no     integer          NOT NULL,
    start_day      integer          NOT NULL CHECK (start_day >= 0),
    duration_s     integer          NOT NULL,
    deposits_n     integer          NOT NULL,
    deposits_eur   double precision NOT NULL,
    net_result_eur double precision NOT NULL,
    is_night       boolean          NOT NULL,
    is_chase       boolean          NOT NULL,
    PRIMARY KEY (player_id, session_no)
);

CREATE TABLE rule_flags (
    player_id text    NOT NULL REFERENCES players (player_id) ON DELETE CASCADE,
    day       integer NOT NULL CHECK (day >= 0),
    rule_id   text    NOT NULL,
    rule_text text    NOT NULL,
    PRIMARY KEY (player_id, day, rule_id)
);

CREATE TABLE review_sample (
    model_version text    NOT NULL REFERENCES models (model_version) ON DELETE CASCADE,
    player_id     text    NOT NULL REFERENCES players (player_id) ON DELETE CASCADE,
    source        text    NOT NULL CHECK (source IN ('model', 'rules', 'both')),
    position      integer NOT NULL,
    PRIMARY KEY (model_version, player_id),
    UNIQUE (model_version, position)
);

CREATE TABLE audit_log (
    id     bigserial PRIMARY KEY,
    at     timestamptz NOT NULL DEFAULT now(),
    actor  text    NOT NULL,
    method text    NOT NULL,
    path   text    NOT NULL,
    status integer NOT NULL
);

-- +goose Down
DROP TABLE audit_log;
DROP TABLE review_sample;
DROP TABLE rule_flags;
DROP TABLE sessions;
DROP TABLE daily_activity;
DROP TABLE daily_scores;
DROP TABLE models;
DROP TABLE players;
DROP TABLE operators;
