package serviceskit_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
)

func withExecution(t *testing.T, ctx context.Context) context.Context {
	t.Helper()
	return execctx.WithExecution(t, ctx)
}
