-- name: GetUserByID :one
SELECT id, clerk_id, email, name, created_at, updated_at
FROM users
WHERE id = $1;

-- name: GetUserByClerkID :one
SELECT id, clerk_id, email, name, created_at, updated_at
FROM users
WHERE clerk_id = $1;

-- name: UpsertUserByClerkID :one
INSERT INTO users (clerk_id, email, name)
VALUES ($1, $2, $3)
ON CONFLICT (clerk_id) DO UPDATE SET
    email = EXCLUDED.email,
    name = EXCLUDED.name,
    updated_at = now()
RETURNING id, clerk_id, email, name, created_at, updated_at;

-- name: DeleteUserByClerkID :execrows
DELETE FROM users
WHERE clerk_id = $1;

-- name: ListUsers :many
SELECT id, clerk_id, email, name, created_at, updated_at
FROM users
ORDER BY created_at DESC
LIMIT $1;
