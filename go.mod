module ruammit-backend

go 1.26

// The runnable scaffold (cmd/api, cmd/worker, internal/platform) is
// stdlib-only so `go build ./...` works immediately. Production
// dependencies are added per module as features land — see README.md
// "Roadmap" for the planned set (chi, pgx, sqlc, redis, asynq, ...).

require (
	github.com/golang-jwt/jwt/v5 v5.3.1
	github.com/jackc/pgx/v5 v5.10.0
	golang.org/x/crypto v0.53.0
)

require (
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	golang.org/x/sync v0.21.0 // indirect
	golang.org/x/text v0.38.0 // indirect
)
