package grpc_test

import (
	"context"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	"github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type outcome struct {
	response  string
	rejection *domain.Rejection
}

func (o outcome) Response() string { return o.response }

func (o outcome) Rejection() (*domain.Rejection, bool) { return o.rejection, o.rejection != nil }

func assembled(t *testing.T) context.Context {
	t.Helper()
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-command", CorrelationID: "c-command", TraceContext: "t-command",
		Deadline: ports.Instant(1_755_432_000_000_000_000), Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	return ports.WithExecutionContext(context.Background(), execution)
}

func refusedWith(r *domain.Rejection) string { return "refused:" + string(r.Code()) }

func acceptedWith(response string) string { return "accepted:" + response }

func TestCommandAnswersInternalWithoutRunningWhenTheExecutionContextIsAbsent(t *testing.T) {
	ran := false
	_, err := grpc.Command(context.Background(),
		func(context.Context) (outcome, error) { ran = true; return outcome{}, nil },
		refusedWith, acceptedWith)

	if status.Code(err) != codes.Internal {
		t.Fatalf("code = %v, want Internal", status.Code(err))
	}
	if ran {
		t.Fatal("the use case ran without an execution context")
	}
}

func TestCommandTranslatesTheTechnicalChannelByStatusOf(t *testing.T) {
	_, err := grpc.Command(assembled(t),
		func(context.Context) (outcome, error) { return outcome{}, ports.ErrNotFound },
		refusedWith, acceptedWith)

	if status.Code(err) != codes.NotFound {
		t.Fatalf("code = %v, want NotFound", status.Code(err))
	}
}

func TestCommandHandsTheRejectionToRefused(t *testing.T) {
	got, err := grpc.Command(assembled(t),
		func(context.Context) (outcome, error) {
			return outcome{rejection: domain.Reject("orders/order/closed", "the order is closed")}, nil
		},
		refusedWith, acceptedWith)

	if err != nil || got != "refused:orders/order/closed" {
		t.Fatalf("Command() = %q, %v; want the rejection mapped by refused", got, err)
	}
}

func TestCommandHandsTheResponseToAccepted(t *testing.T) {
	got, err := grpc.Command(assembled(t),
		func(context.Context) (outcome, error) { return outcome{response: "o-1"}, nil },
		refusedWith, acceptedWith)

	if err != nil || got != "accepted:o-1" {
		t.Fatalf("Command() = %q, %v; want the response mapped by accepted", got, err)
	}
}

func TestExecutionOfReturnsTheAssembledContext(t *testing.T) {
	execution, err := grpc.ExecutionOf(assembled(t))

	if err != nil || execution.RequestID() != "r-command" {
		t.Fatalf("ExecutionOf() = %v, %v; want the assembled context", execution.RequestID(), err)
	}
}
