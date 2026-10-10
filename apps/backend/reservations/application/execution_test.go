package application_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

func withExecution(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	return execctx.WithExecution(t, ctx, execctx.WithSlot(), execctx.AutoKey())
}

func withKey(t *testing.T, ctx context.Context, key string) context.Context {
	t.Helper()
	return execctx.WithKey(t, ctx, key, execctx.WithSlot())
}
