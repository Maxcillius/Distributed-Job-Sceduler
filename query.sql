-- name: GetJobStatus :one
SELECT * FROM jobstatus WHERE id = $1;

-- name: GetJobsByStatus :many
SELECT * FROM jobstatus WHERE status = $1;

-- name: InsertJob :one
INSERT INTO jobstatus (
    id, name, command, Args, WorkDir, TimeoutSeconds, status
) VALUES (
    $1, $2, $3, $4, $5, $6, $7
)
ON CONFLICT (id) DO UPDATE SET status = EXCLUDED.status
RETURNING *;

-- name: UpdateJob :one
UPDATE jobstatus SET status = $2 WHERE id = $1
RETURNING *;

-- name: DeleteJob :exec
DELETE FROM jobstatus WHERE id = $1;