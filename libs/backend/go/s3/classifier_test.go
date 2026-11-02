package s3

import (
	"context"
	"errors"
	"fmt"
	"net"
	"testing"

	"github.com/aws/smithy-go"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
)

func TestEveryFailureIsRecordedUnderAnFND07CategoryOrOther(t *testing.T) {
	other := semconv.ErrorTypeOther.Value.AsString()
	generic := func(code string, fault smithy.ErrorFault) error {
		return &smithy.OperationError{ServiceID: "S3", OperationName: "PutObject",
			Err: &smithy.GenericAPIError{Code: code, Message: "tenant/acme/doc: secret", Fault: fault}}
	}
	cases := map[string]struct {
		err  error
		want string
	}{
		"success":            {nil, "ok"},
		"deadline":           {fmt.Errorf("x: %w", context.DeadlineExceeded), "DeadlineExceeded"},
		"cancelled":          {fmt.Errorf("x: %w", context.Canceled), "Cancelled"},
		"network":            {&net.OpError{Op: "dial", Err: errors.New("refused")}, "TransientDependency"},
		"plain":              {errors.New("tenant/acme/doc"), other},
		"unmapped server":    {generic("InternalFailure", smithy.FaultServer), "TransientDependency"},
		"unmapped client":    {generic("SomethingNew", smithy.FaultClient), other},
		"SlowDown":           {generic("SlowDown", smithy.FaultServer), "RateLimited"},
		"ServiceUnavailable": {generic("ServiceUnavailable", smithy.FaultServer), "TransientDependency"},
		"NoSuchKey":          {generic("NoSuchKey", smithy.FaultClient), "NotFound"},
		"NoSuchVersion":      {generic("NoSuchVersion", smithy.FaultClient), "NotFound"},
		"NoSuchBucket":       {generic("NoSuchBucket", smithy.FaultClient), "NotFound"},
		"AccessDenied":       {generic("AccessDenied", smithy.FaultClient), "Forbidden"},
		"ExpiredToken":       {generic("ExpiredToken", smithy.FaultClient), "Unauthenticated"},
		"BadDigest":          {generic("BadDigest", smithy.FaultClient), "Validation"},
		"InvalidRequest":     {generic("InvalidRequest", smithy.FaultClient), "Validation"},
		"lock not found":     {generic("ObjectLockConfigurationNotFoundError", smithy.FaultClient), "Validation"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := categoryOf(tc.err); got != tc.want {
				t.Fatalf("categoryOf() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestClassifier(t *testing.T) {
	cases := map[string]struct {
		err  error
		want retry.Retryability
	}{
		"cancelled":     {context.Canceled, retry.NotRetryable},
		"deadline":      {context.DeadlineExceeded, retry.NotRetryable},
		"server fault":  {&smithy.GenericAPIError{Code: "InternalError", Fault: smithy.FaultServer}, retry.Retryable},
		"slow down":     {&smithy.GenericAPIError{Code: "SlowDown", Fault: smithy.FaultClient}, retry.Retryable},
		"access denied": {&smithy.GenericAPIError{Code: "AccessDenied", Fault: smithy.FaultClient}, retry.NotRetryable},
		"not found":     {&smithy.GenericAPIError{Code: "NoSuchVersion", Fault: smithy.FaultClient}, retry.NotRetryable},
		"network":       {&net.OpError{Op: "dial", Err: errors.New("refused")}, retry.Retryable},
		"plain":         {errors.New("x"), retry.NotRetryable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := Classifier(tc.err); got != tc.want {
				t.Fatalf("Classifier() = %v, want %v", got, tc.want)
			}
		})
	}
}
