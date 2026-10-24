package app

import (
	"context"
	"errors"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository helpers centralize pgx row decoding. All SQL stays parameterized.
func rows[T any](ctx context.Context, db *pgxpool.Pool, q string, args ...any) ([]T, error) {
	rs, e := db.Query(ctx, q, args...)
	if e != nil {
		return nil, e
	}
	defer rs.Close()
	return pgx.CollectRows(rs, pgx.RowToStructByName[T])
}
func one[T any](ctx context.Context, db *pgxpool.Pool, q string, args ...any) (T, error) {
	var z T
	rs, e := rows[T](ctx, db, q, args...)
	if e != nil || len(rs) == 0 {
		return z, errors.New("not found")
	}
	return rs[0], nil
}
