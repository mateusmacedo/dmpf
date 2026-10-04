package sqs

import (
	"context"
	"log/slog"

	awslog "github.com/aws/smithy-go/logging"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
)

// WHY: a library's log carries the library's module as scope, as boot does for
// gRPC and the OpenTelemetry SDK (observability/boot/libraries.go:25-26).
const awsScope = "github.com/aws/aws-sdk-go-v2"

const awsDiagnostic = "aws sdk: diagnostic withheld, its text may carry a payload or a credential"

// WHY: the SDK formats the request and response dumps and the canonical string, session
// token included, into the text (smithy-go transport/http/middleware_http_logging.go:47,71;
// aws/signer/v4/v4.go:545), so only the classification leaves.
type awsLogger struct {
	logger *slog.Logger
	ctx    context.Context
}

func (c Config) awsLogger() awsLogger {
	return awsLogger{logger: logging.NewLogger(c.LoggerProvider, awsScope), ctx: context.Background()}
}

func (l awsLogger) Logf(classification awslog.Classification, _ string, _ ...any) {
	level := slog.LevelDebug
	if classification == awslog.Warn {
		level = slog.LevelWarn
	}
	l.logger.LogAttrs(l.ctx, level, awsDiagnostic)
}

func (l awsLogger) WithContext(ctx context.Context) awslog.Logger {
	return awsLogger{logger: l.logger, ctx: ctx}
}
