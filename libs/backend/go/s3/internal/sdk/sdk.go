// comment-discipline-ok-file: peças do cliente S3 que a loja e o caminho administrativo compartilham; cada godoc cita a regra (DAT-22, ERR-11, RES-27, TRC-12, MET-07) que o símbolo realiza, dentro do limite de 3 linhas.

// Package sdk builds the S3 client the store and the administrative path share:
// the SDK never repeats a call, its log leaves only a classification, and every
// failure is classified for retry and for telemetry by one closed table.
package sdk

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"strconv"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
	awslog "github.com/aws/smithy-go/logging"
	"go.opentelemetry.io/otel/log"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/logging"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/observe"
)

// Client is the S3 client over the resolved AWS configuration, pointed at the
// endpoint when one is given. The SDK never repeats a call: only the sheet
// authorizes another attempt (RES-27).
func Client(cfg aws.Config, endpoint string, pathStyle bool, provider log.LoggerProvider) *awss3.Client {
	return awss3.NewFromConfig(cfg, func(o *awss3.Options) {
		o.Logger = Logger(provider)
		o.Retryer = aws.NopRetryer{}
		o.UsePathStyle = pathStyle
		if endpoint != "" {
			o.BaseEndpoint = &endpoint
		}
	})
}

// WHY: a library's log carries the library's module as scope, as boot does for
// gRPC and the OpenTelemetry SDK (observability/boot/libraries.go:25-26).
const awsScope = "github.com/aws/aws-sdk-go-v2"

const awsDiagnostic = "aws sdk: diagnostic withheld, its text may carry a payload or a credential"

// WHY: the SDK formats the request and response dumps and the canonical string, session
// token included, into the text (smithy-go transport/http/middleware_http_logging.go:47,71;
// aws/signer/v4/v4.go:545), so only the classification leaves (DAT-22).
type awsLogger struct {
	logger *slog.Logger
	ctx    context.Context
}

// Logger is the SDK logger that emits the level and a fixed message only.
func Logger(provider log.LoggerProvider) awslog.Logger {
	return awsLogger{logger: logging.NewLogger(provider, awsScope), ctx: context.Background()}
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

// Classifier is the retry taxonomy of the S3 calls: a throttle, a server fault
// and a network failure are worth another attempt; a cancellation, a deadline
// and any other API error are not. Idempotence is the operation's to declare.
func Classifier(err error) retry.Retryability {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return retry.NotRetryable
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		if api.ErrorFault() == smithy.FaultServer || Status(err) >= 500 || isThrottle(api.ErrorCode()) {
			return retry.Retryable
		}
		return retry.NotRetryable
	}
	if Status(err) >= 500 {
		return retry.Retryable
	}
	var network net.Error
	if errors.As(err, &network) {
		return retry.Retryable
	}
	return retry.NotRetryable
}

func isThrottle(code string) bool {
	switch code {
	case "SlowDown", "Throttling", "ThrottlingException", "RequestThrottled", "TooManyRequestsException",
		"RequestLimitExceeded", "KMS.ThrottlingException", "ServiceUnavailable", "InternalError":
		return true
	}
	return false
}

// Category is the closed category of FND-07 a failure is reported under (ERR-11);
// a code outside the table is _OTHER, or TransientDependency on a server fault.
func Category(err error) string {
	if err == nil {
		return observe.CategoryOK
	}
	var categorized interface{ ErrorCategory() string }
	if errors.As(err, &categorized) {
		return categorized.ErrorCategory()
	}
	switch {
	case errors.Is(err, context.DeadlineExceeded):
		return "DeadlineExceeded"
	case errors.Is(err, context.Canceled):
		return "Cancelled"
	}
	var api smithy.APIError
	if errors.As(err, &api) {
		return apiCategory(api, Status(err))
	}
	var network net.Error
	if errors.As(err, &network) || Status(err) >= 500 {
		return categoryTransientDependency
	}
	return semconv.ErrorTypeOther.Value.AsString()
}

const categoryTransientDependency = "TransientDependency"

var fnd07Codes = map[string]string{
	"ServiceUnavailable":                   categoryTransientDependency,
	"InternalError":                        categoryTransientDependency,
	"SlowDown":                             "RateLimited",
	"Throttling":                           "RateLimited",
	"ThrottlingException":                  "RateLimited",
	"RequestThrottled":                     "RateLimited",
	"TooManyRequestsException":             "RateLimited",
	"RequestLimitExceeded":                 "RateLimited",
	"KMS.ThrottlingException":              "RateLimited",
	"NoSuchKey":                            "NotFound",
	"NoSuchVersion":                        "NotFound",
	"NoSuchBucket":                         "NotFound",
	"NotFound":                             "NotFound",
	"AccessDenied":                         "Forbidden",
	"KMS.AccessDeniedException":            "Forbidden",
	"KMS.DisabledException":                "Forbidden",
	"InvalidAccessKeyId":                   "Unauthenticated",
	"SignatureDoesNotMatch":                "Unauthenticated",
	"ExpiredToken":                         "Unauthenticated",
	"InvalidToken":                         "Unauthenticated",
	"BadDigest":                            "Validation",
	"InvalidDigest":                        "Validation",
	"InvalidRequest":                       "Validation",
	"InvalidArgument":                      "Validation",
	"InvalidBucketState":                   "Validation",
	"ObjectLockConfigurationNotFoundError": "Validation",
}

// WHY: the S3 deserializer builds its API errors without a fault, so a 5xx is
// recognized by the status of the response, not by smithy.FaultServer.
func apiCategory(api smithy.APIError, status int) string {
	if category, mapped := fnd07Codes[api.ErrorCode()]; mapped {
		return category
	}
	if api.ErrorFault() == smithy.FaultServer || status >= 500 {
		return categoryTransientDependency
	}
	return semconv.ErrorTypeOther.Value.AsString()
}

// Code is the error code of an S3 answer, empty when the failure is not one.
func Code(err error) string {
	var api smithy.APIError
	if errors.As(err, &api) {
		return api.ErrorCode()
	}
	return ""
}

// Status is the HTTP status of the response a failure came with, zero when
// none came back (the request did not complete, or the failure is not one).
func Status(err error) int {
	var response interface{ HTTPStatusCode() int }
	if errors.As(err, &response) {
		return response.HTTPStatusCode()
	}
	return 0
}

// Fail is the failure a caller receives: it unwraps to the SDK's error for
// errors.Is and errors.As, but its text carries only operation, category and
// code, never the URL with the physical key a transport error prints (DAT-22).
func Fail(operation string, err error) error {
	if err == nil {
		return nil
	}
	return &failure{operation: operation, err: err}
}

type failure struct {
	operation string
	err       error
}

func (f *failure) Error() string {
	text := "s3: " + f.operation + " failed: " + Category(f.err)
	if code := Code(f.err); code != "" {
		text += " (" + code + ")"
	}
	if status := Status(f.err); status != 0 {
		text += " status " + strconv.Itoa(status)
	}
	return text
}

func (f *failure) Unwrap() error { return f.err }
