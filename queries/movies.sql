-- name: CreateMovie :one
INSERT INTO movies (title, year, runtime, genres)
VALUES ($1, $2, $3, $4)
RETURNING id, created_at, version;

-- name: GetMovie :one
SELECT  * FROM movies
WHERE id = $1;

-- name: ListMovies :many
SELECT * FROM movies
WHERE (to_tsvector('simple', title) @@ plainto_tsquery('simple', @filter_title) 
OR @filter_title = '')
AND (genres @> @filter_genres OR @filter_genres = '{}'::text[])
ORDER BY
  CASE WHEN @sort_col = 'title' AND @sort_dir = 'ASC' THEN title END ASC,
  CASE WHEN @sort_col = 'title' AND @sort_dir = 'DESC' THEN title END DESC,

  CASE WHEN @sort_col = 'year' AND @sort_dir = 'ASC' THEN year END ASC,
  CASE WHEN @sort_col = 'year' AND @sort_dir = 'DESC' THEN year END DESC,

  CASE WHEN @sort_col = 'runtime' AND @sort_dir = 'ASC' THEN runtime END ASC,
  CASE WHEN @sort_col = 'runtime' AND @sort_dir = 'DESC' THEN runtime END DESC,
  
  id ASC;

-- name: UpdateMovie :one
UPDATE movies
SET title = $1, year = $2, runtime = $3, genres = $4, version = version + 1
WHERE id = $5 AND version = $6
RETURNING *;

-- name: DeleteMovie :exec
DELETE FROM movies
WHERE id = $1;
