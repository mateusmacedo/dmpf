package grpc

import (
	"context"
	"log/slog"
	"sync/atomic"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"google.golang.org/grpc/stats"
	"google.golang.org/grpc/status"
)

// WHY: grpc-go decodes the request before the unary chain and sends the reply
// after it (grpc@v1.85.0-dev.0.20260825072537-93e31b48545e/server.go:1283-1288), so only stats.End
// (server.go:1342-1350) holds the status the call closed with (RF-A5).
type accessLog struct {
	logger *slog.Logger
	filter otelgrpc.Filter
}

type accessMark struct {
	fullMethod string
	note       atomic.Pointer[callNote]
}

type callNote struct {
	logger *slog.Logger
	slot   *assembled
	err    error
}

type accessMarkKey struct{}

func (h accessLog) TagRPC(ctx context.Context, info *stats.RPCTagInfo) context.Context {
	if !h.filter(info) {
		return ctx
	}
	return context.WithValue(ctx, accessMarkKey{}, &accessMark{fullMethod: info.FullMethodName})
}

func (h accessLog) HandleRPC(ctx context.Context, rs stats.RPCStats) {
	end, ended := rs.(*stats.End)
	if !ended {
		return
	}
	mark, tagged := ctx.Value(accessMarkKey{}).(*accessMark)
	if !tagged {
		return
	}
	logger, slot, err := h.logger, &assembled{}, end.Error
	if note := mark.note.Load(); note != nil {
		logger, slot = note.logger, note.slot
		if status.Code(publicStatus(note.err)) == status.Code(end.Error) {
			err = note.err
		}
	}
	logCall(ctx, logger, mark.fullMethod, slot, err)
}

func (accessLog) TagConn(ctx context.Context, _ *stats.ConnTagInfo) context.Context { return ctx }

func (accessLog) HandleConn(context.Context, stats.ConnStats) {}

func noteCall(ctx context.Context, logger *slog.Logger, fullMethod string, slot *assembled, err error) {
	if mark, tagged := ctx.Value(accessMarkKey{}).(*accessMark); tagged {
		mark.note.Store(&callNote{logger: logger, slot: slot, err: err})
		return
	}
	logCall(ctx, logger, fullMethod, slot, err)
}
