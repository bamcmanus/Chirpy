-- name: CreateChirp :one
INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1,
    $2
)
RETURNING *;

-- name: ListChirps :many
SELECT *
FROM chirps
ORDER BY created_at;

-- name: GetChirp :one
SELECT *
FROM Chirps
WHERE Id = $1;

-- name: DeleteChirp :exec
DELETE
FROM chirps
WHERE id = $1;

-- name: UpgradeUser :one
UPDATE users
SET is_chirpy_red = true
WHERE id = $1
RETURNING *;
