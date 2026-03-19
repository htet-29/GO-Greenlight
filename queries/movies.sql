-- name: CreateMovie :one
INSERT INTO movies (title, year, runtime, genres)
VALUES ($1, $2, $3, $4)
RETURNING id, created_at, version;

-- name: GetMovie :one
SELECT  * FROM movies
WHERE id = $1;

-- name: ListMovies :many
SELECT * FROM movies
WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', @filter_title) OR @filter_title = '')
AND (genres @> @filter_genres OR @filter_genres = '{}'::text[])
ORDER BY id;

-- name: UpdateMovie :one
UPDATE movies
SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
WHERE id = $5 AND version = $6
RETURNING *;

-- name: DeleteMovie :exec
DELETE FROM movies
WHERE id = $1;
