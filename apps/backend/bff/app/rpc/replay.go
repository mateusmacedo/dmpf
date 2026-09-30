package rpc

import (
	"context"
	"slices"
	"sync/atomic"

	"google.golang.org/grpc/metadata"

	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
)

type replayKey struct{}

// WithReplaySlot gives one request a place to learn that a context answered a
// command from its record (IDM-08), so the edge can say so to the client.
func WithReplaySlot(ctx context.Context) context.Context {
	return context.WithValue(ctx, replayKey{}, new(atomic.Bool))
}

func Replayed(ctx context.Context) bool {
	slot, ok := ctx.Value(replayKey{}).(*atomic.Bool)
	return ok && slot.Load()
}

func markReplayed(ctx context.Context, header metadata.MD) {
	if slot, ok := ctx.Value(replayKey{}).(*atomic.Bool); ok && slices.Equal(header.Get(kernelgrpc.ReplayedHeader), []string{"true"}) {
		slot.Store(true)
	}
}
