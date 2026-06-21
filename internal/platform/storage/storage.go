// Package storage manages the Postgres connection pool and exposes the
// sqlc-generated query layer (subpackage dbgen).
package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"waymeet-backend/internal/platform/storage/dbgen"
)

// DB bundles the pgx pool with the generated Queries. Pass *DB into module
// repositories; use Pool directly for transactions (pool.Begin â†’ q.WithTx(tx)).
type DB struct {
	Pool    *pgxpool.Pool
	Queries *dbgen.Queries
}

// Connect opens a pgx pool against databaseURL and verifies connectivity with a
// ping. The caller must Close the returned *DB on shutdown.
func Connect(ctx context.Context, databaseURL string) (*DB, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 10
	cfg.MaxConnIdleTime = 5 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return &DB{Pool: pool, Queries: dbgen.New(pool)}, nil
}

// Close releases all pooled connections.
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}
