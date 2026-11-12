// Package store holds every SQL query. Handlers never write SQL themselves.
package store

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

//go:embed schema.sql
var schemaSQL string

// Errors the API layer turns into HTTP status codes.
var (
	ErrNotFound       = errors.New("not found")
	ErrDuplicate      = errors.New("already exists")
	ErrInvalidPolygon = errors.New("polygon is not valid")
)

// Store wraps a pool of database connections. A pool is safe to share
// between the goroutines that serve requests.
type Store struct {
	pool *pgxpool.Pool
}

// New connects to PostgreSQL, waits until it answers, and creates the tables.
func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("read database url: %w", err)
	}

	// With Docker Compose the database can start a little after the API,
	// so retry for up to 30 seconds before giving up.
	var pingErr error
	for attempt := 0; attempt < 30; attempt++ {
		if pingErr = pool.Ping(ctx); pingErr == nil {
			break
		}
		time.Sleep(time.Second)
	}
	if pingErr != nil {
		pool.Close()
		return nil, fmt.Errorf("connect to database: %w", pingErr)
	}

	if _, err := pool.Exec(ctx, schemaSQL); err != nil {
		pool.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}
	return &Store{pool: pool}, nil
}

// Close releases all database connections.
func (s *Store) Close() {
	s.pool.Close()
}

// isUniqueViolation reports whether err is PostgreSQL's "duplicate key" error.
func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
