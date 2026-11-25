-- name: InsertReview :one
INSERT INTO reviews (player_id, as_of_day, outcome, note, reviewer, blind, model_version)
VALUES (@player_id, @as_of_day, @outcome, sqlc.narg(note), @reviewer, @blind, @model_version)
RETURNING *;

-- name: ListPlayerReviews :many
SELECT * FROM reviews WHERE player_id = @player_id AND as_of_day <= @day ORDER BY id DESC;

-- name: PlayerLastDay :one
SELECT last_day FROM players WHERE player_id = @player_id;
