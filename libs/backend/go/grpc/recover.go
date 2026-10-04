package grpc

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"
)

// WHY: grpc-go runs every call on a goroutine without recover
// (grpc@v1.85.0-dev.0.20260825072537-93e31b48545e/server.go:1107), so a panic ends the process with the runtime's
// stack on stderr, outside cmd/main.go and outside OTLP (ERR-23, RF-A1).
func recoverUnary(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		slot := &assembled{}
		defer func() {
			if recover() != nil {
				resp, err = nil, panicked(ctx, logger, info.FullMethod, slot)
			}
		}()
		return handler(context.WithValue(ctx, assembledKey{}, slot), req)
	}
}

func recoverStream(logger *slog.Logger) grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if recover() != nil {
				err = panicked(stream.Context(), logger, info.FullMethod, &assembled{})
			}
		}()
		return handler(srv, stream)
	}
}

func panicked(ctx context.Context, logger *slog.Logger, fullMethod string, slot *assembled) error {
	err := status.Error(codes.Internal, "internal failure")
	span := trace.SpanFromContext(ctx)
	span.SetAttributes(tracing.Attributes{}.OutcomeCategory(categoryOf(err)).KeyValues()...)
	tracing.RecordError(span, categoryOf(err))
	noteCall(ctx, logger, fullMethod, slot, err)
	return err
}
