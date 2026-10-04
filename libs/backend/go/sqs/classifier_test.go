package sqs_test

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	snstypes "github.com/aws/aws-sdk-go-v2/service/sns/types"
	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"github.com/aws/smithy-go"
	semconv "go.opentelemetry.io/otel/semconv/v1.43.0"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/sqs"
)

func TestEveryFailureIsRecordedUnderAnFND07CategoryOrOther(t *testing.T) {
	other := semconv.ErrorTypeOther.Value.AsString()
	generic := func(code string, fault smithy.ErrorFault) error {
		return &smithy.GenericAPIError{Code: code, Message: "queue 10.0.0.7: secret", Fault: fault}
	}
	operation := func(err error) error {
		return &smithy.OperationError{ServiceID: "SQS", OperationName: "SendMessage", Err: err}
	}
	cases := map[string]struct {
		err  error
		want string
	}{
		"success":                   {nil, "ok"},
		"categorized":               {fmt.Errorf("wrapped: %w", categorized{category: "Unexpected"}), "Unexpected"},
		"context deadline":          {fmt.Errorf("queue 10.0.0.7: %w", context.DeadlineExceeded), "DeadlineExceeded"},
		"context cancelled":         {fmt.Errorf("queue 10.0.0.7: %w", context.Canceled), "Cancelled"},
		"sink panicked":             {fmt.Errorf("%w: secret", provider.ErrSinkPanicked), "Unexpected"},
		"network":                   {&net.OpError{Op: "dial", Net: "tcp", Err: errors.New("connection refused")}, "TransientDependency"},
		"plain error":               {errors.New("queue 10.0.0.7: secret"), other},
		"server fault":              {operation(generic("InternalFailure", smithy.FaultServer)), "TransientDependency"},
		"ServiceUnavailable":        {generic("ServiceUnavailable", smithy.FaultUnknown), "TransientDependency"},
		"InternalError":             {&snstypes.InternalErrorException{}, "TransientDependency"},
		"Throttling":                {generic("Throttling", smithy.FaultClient), "RateLimited"},
		"ThrottlingException":       {generic("ThrottlingException", smithy.FaultClient), "RateLimited"},
		"RequestThrottled":          {operation(&sqstypes.RequestThrottled{}), "RateLimited"},
		"RequestThrottledException": {generic("RequestThrottledException", smithy.FaultClient), "RateLimited"},
		"TooManyRequestsException":  {generic("TooManyRequestsException", smithy.FaultClient), "RateLimited"},
		"RequestLimitExceeded":      {generic("RequestLimitExceeded", smithy.FaultClient), "RateLimited"},
		"Throttled":                 {&snstypes.ThrottledException{}, "RateLimited"},
		"KmsThrottled":              {&sqstypes.KmsThrottled{}, "RateLimited"},
		"KMSThrottling":             {&snstypes.KMSThrottlingException{}, "RateLimited"},
		"QueueDoesNotExist":         {operation(&sqstypes.QueueDoesNotExist{}), "NotFound"},
		"NotFound":                  {&snstypes.NotFoundException{}, "NotFound"},
		"AccessDeniedException":     {generic("AccessDeniedException", smithy.FaultClient), "Forbidden"},
		"AuthorizationError":        {&snstypes.AuthorizationErrorException{}, "Forbidden"},
		"KmsAccessDenied":           {&sqstypes.KmsAccessDenied{}, "Forbidden"},
		"KMSAccessDenied":           {&snstypes.KMSAccessDeniedException{}, "Forbidden"},
		"InvalidSecurity (SQS)":     {&sqstypes.InvalidSecurity{}, "Unauthenticated"},
		"InvalidSecurity (SNS)":     {&snstypes.InvalidSecurityException{}, "Unauthenticated"},
		"InvalidMessageContents":    {&sqstypes.InvalidMessageContents{}, "Validation"},
		"InvalidAttributeName":      {&sqstypes.InvalidAttributeName{}, "Validation"},
		"InvalidAttributeValue":     {&sqstypes.InvalidAttributeValue{}, "Validation"},
		"InvalidParameter":          {&snstypes.InvalidParameterException{}, "Validation"},
		"ParameterValueInvalid":     {&snstypes.InvalidParameterValueException{}, "Validation"},
		"ValidationException":       {&snstypes.ValidationException{}, "Validation"},
		"UnsupportedOperation":      {&sqstypes.UnsupportedOperation{}, other},
		"KmsDisabled":               {&sqstypes.KmsDisabled{}, other},
		"EndpointDisabled":          {&snstypes.EndpointDisabledException{}, other},
		"undeclared client code":    {generic("SomethingElse", smithy.FaultClient), other},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			if got := provider.CategoryOf(c.err); got != c.want {
				t.Fatalf("CategoryOf(%v) = %q, want %q: FND-07 read back from the context, the network or the SDK code, or %s (RF-B1)", c.err, got, c.want, other)
			}
		})
	}
}

func TestEveryThrottleRecordedAsRateLimitedIsRetried(t *testing.T) {
	generic := func(code string) error {
		return &smithy.GenericAPIError{Code: code, Message: "queue 10.0.0.7: secret", Fault: smithy.FaultClient}
	}
	cases := map[string]error{
		"Throttling":                generic("Throttling"),
		"ThrottlingException":       generic("ThrottlingException"),
		"RequestThrottled":          &sqstypes.RequestThrottled{},
		"RequestThrottledException": generic("RequestThrottledException"),
		"TooManyRequestsException":  generic("TooManyRequestsException"),
		"RequestLimitExceeded":      generic("RequestLimitExceeded"),
		"Throttled":                 &snstypes.ThrottledException{},
		"KmsThrottled":              &sqstypes.KmsThrottled{},
		"KMSThrottling":             &snstypes.KMSThrottlingException{},
		"KMS.ThrottlingException":   generic("KMS.ThrottlingException"),
	}
	for name, err := range cases {
		t.Run(name, func(t *testing.T) {
			if got := provider.CategoryOf(err); got != "RateLimited" {
				t.Fatalf("CategoryOf(%v) = %q, want RateLimited", err, got)
			}
			if got := provider.Classifier(err); got != retry.Retryable {
				t.Fatalf("Classifier(%v) = %v, want Retryable: the mapping keeps the retryability of RateLimited (MAP-03)", err, got)
			}
		})
	}
}

func TestTheQueryCompatibleCodeOfARealSQSRefusalKeepsItsFND07Category(t *testing.T) {
	cases := map[string]struct {
		shape, queryCode, want string
	}{
		"QueueDoesNotExist": {"QueueDoesNotExist", "AWS.SimpleQueueService.NonExistentQueue", "NotFound"},
		"KmsAccessDenied":   {"KmsAccessDenied", "KMS.AccessDeniedException", "Forbidden"},
		"KmsThrottled":      {"KmsThrottled", "KMS.ThrottlingException", "RateLimited"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Type", "application/x-amz-json-1.0")
				w.Header().Set("X-Amzn-Query-Error", c.queryCode+";Sender")
				w.WriteHeader(http.StatusBadRequest)
				_, _ = fmt.Fprintf(w, `{"__type":"com.amazonaws.sqs#%s","message":"queue 10.0.0.7: secret"}`, c.shape)
			}))
			defer srv.Close()
			client := sqs.New(sqs.Options{
				BaseEndpoint: aws.String(srv.URL),
				Region:       "us-east-1",
				Credentials:  aws.AnonymousCredentials{},
				Retryer:      aws.NopRetryer{},
			})

			_, err := client.SendMessage(context.Background(), &sqs.SendMessageInput{QueueUrl: aws.String(srv.URL), MessageBody: aws.String("{}")})

			if got := provider.CategoryOf(err); got != c.want {
				t.Fatalf("CategoryOf(%v) = %q, want %q: the SDK reports the query code, not the shape name (RF-B1)", err, got, c.want)
			}
		})
	}
}

func TestARefusedSendIsRecordedUnderItsFND07CategoryOnTheResilienceSpan(t *testing.T) {
	cfg, recorder, _ := traced(validConfig())
	api := provider.NewFakeSQS()
	api.SendErr = &sqstypes.QueueDoesNotExist{}
	pub, err := provider.NewPublisher(cfg, api)
	if err != nil {
		t.Fatalf("NewPublisher() = %v", err)
	}
	raw, _ := validRaw(t, "k1")

	if err := pub.Publish(context.Background(), "reservations", raw); err == nil {
		t.Fatal("Publish() = nil, want the SDK's refusal")
	}

	spans := recorder.Ended()
	if len(spans) != 1 || spans[0].Name() != "dmpf.resilience sqs" {
		t.Fatalf("ended %d spans, want only the resilience span over the attempts", len(spans))
	}
	if got, _ := attributeOf(spans[0], semconv.ErrorTypeKey); got.AsString() != "NotFound" {
		t.Errorf("error.type = %q, want NotFound, never the SDK code (RF-B1)", got.AsString())
	}
}
