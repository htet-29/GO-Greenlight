-- name: CreateUser :one
INSERT INTO users (name, email, password_hash, activated)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetUserByEmail :one
SELECT * FROM users
WHERE email = $1;

-- name: UpdateUser :one
UPDATE users
SET name = $1, email = $2, password_hash = $3, activated = $4, version = version + 1
WHERE id = $5 AND version = $6
RETURNING version;

-- name: GetUserByToken :one
SELECT sqlc.embed(users) FROM users
INNER JOIN tokens
ON users.id = tokens.user_id
WHERE tokens.hash = $1
AND tokens.scope = $2
AND tokens.expiry > $3;
