//go:build integration

package postgres_test

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

var benchmarkedCreates atomic.Int64

func benchmarkCreates(b *testing.B, pool *pgxpool.Pool, save func(ctx context.Context, tx *postgres.Tx, n int64) error) {
	ctx := scopedTo(b, "acme")
	for b.Loop() {
		b.StopTimer()
		conn, err := pool.Begin(ctx)
		if err != nil {
			b.Fatalf("Begin() = %v", err)
		}
		b.StartTimer()
		if err := save(ctx, postgres.NewTx(conn), benchmarkedCreates.Add(1)); err != nil {
			_ = conn.Rollback(ctx)
			b.Fatalf("Save() = %v", err)
		}
		b.StopTimer()
		if err := conn.Commit(ctx); err != nil {
			b.Fatalf("Commit() = %v", err)
		}
		b.StartTimer()
	}
}

func BenchmarkSave(b *testing.B) {
	pool := openPool(b)
	plain := uniqueTable
	plain.Unique = nil
	for _, tt := range []struct {
		name  string
		table postgres.Table[string, uniqueProbe]
	}{{"without Unique", plain}, {"with Unique", uniqueTable}} {
		b.Run(tt.name, func(b *testing.B) {
			benchmarkCreates(b, pool, func(ctx context.Context, tx *postgres.Tx, n int64) error {
				v := fmt.Sprint(n)
				return tt.table.Repository(tx).Save(ctx, "B-"+v, uniqueProbe{Name: v, Code: v, Label: v, RefA: v, RefB: v}, 0)
			})
		})
	}
}
