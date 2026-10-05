package application_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

type spanKey struct{}

type stepRecorder struct {
	steps  []string
	result ports.Result
}

func (r *stepRecorder) BeginOperation(ctx context.Context, operation string) (context.Context, ports.EndOperation) {
	r.steps = append(r.steps, "begin "+operation)
	return context.WithValue(ctx, spanKey{}, operation), func(result ports.Result) {
		r.steps = append(r.steps, "end "+string(result.Outcome))
		r.result = result
	}
}

func (*stepRecorder) Audit(context.Context, ports.AuditEvent) {}

type lookup interface{ target() string }

type findThing struct{ id string }

func (f findThing) target() string { return f.id }

func (r *stepRecorder) authorize(err error) application.Authorize[lookup] {
	return func(ctx context.Context, op lookup) error {
		r.steps = append(r.steps, fmt.Sprintf("authorize %s in %v", op.target(), ctx.Value(spanKey{})))
		return err
	}
}

func (r *stepRecorder) load(value string, err error) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		r.steps = append(r.steps, fmt.Sprintf("load in %v", ctx.Value(spanKey{})))
		return value, err
	}
}

func TestQueryWalksBeginAuthorizeLoadEndAndReturnsTheSnapshot(t *testing.T) {
	r := &stepRecorder{}

	got, err := application.Query[lookup](context.Background(), r, r.authorize(nil), "things.Find", findThing{id: "t-1"}, r.load("snapshot", nil))

	if err != nil || got != "snapshot" {
		t.Fatalf("Query = (%q, %v), want (\"snapshot\", nil)", got, err)
	}
	want := []string{"begin things.Find", "authorize t-1 in things.Find", "load in things.Find", "end accepted"}
	if !slices.Equal(r.steps, want) {
		t.Fatalf("steps = %q, want %q", r.steps, want)
	}
}

func TestQueryEndsDeniedWithoutLoadingWhenTheAuthorizerRefuses(t *testing.T) {
	r := &stepRecorder{}
	denial := fmt.Errorf("%w: missing things:read", ports.ErrDenied)

	got, err := application.Query[lookup](context.Background(), r, r.authorize(denial), "things.Find", findThing{id: "t-1"}, r.load("snapshot", nil))

	if err != denial || got != "" {
		t.Fatalf("Query = (%q, %v), want (\"\", the denial)", got, err)
	}
	want := []string{"begin things.Find", "authorize t-1 in things.Find", "end denied"}
	if !slices.Equal(r.steps, want) {
		t.Fatalf("steps = %q, want %q", r.steps, want)
	}
	if r.result.Err != nil {
		t.Fatalf("Result.Err = %v on a denial, want nil", r.result.Err)
	}
}

func TestQueryEndsFailedWithoutLoadingWhenTheAuthorizerFails(t *testing.T) {
	r := &stepRecorder{}
	cause := errors.New("policy engine unreachable")

	_, err := application.Query[lookup](context.Background(), r, r.authorize(cause), "things.Find", findThing{id: "t-1"}, r.load("snapshot", nil))

	if err != cause {
		t.Fatalf("err = %v, want %v", err, cause)
	}
	if r.result != (ports.Result{Outcome: ports.OutcomeFailed, Err: cause}) {
		t.Fatalf("result = %+v, want failed with the authorizer error", r.result)
	}
	if slices.Contains(r.steps, "load in things.Find") {
		t.Fatal("Query loaded after the authorizer failed")
	}
}

func TestQueryReturnsTheLoadErrorUnchangedAndEndsFailedWithIt(t *testing.T) {
	r := &stepRecorder{}
	failed := fmt.Errorf("application: find thing t-1: %w", ports.ErrNotFound)

	got, err := application.Query[lookup](context.Background(), r, r.authorize(nil), "things.Find", findThing{id: "t-1"}, r.load("partial", failed))

	if err != failed {
		t.Fatalf("err = %v, want the load error unchanged", err)
	}
	if got != "" {
		t.Fatalf("snapshot = %q on failure, want the zero value", got)
	}
	if r.result != (ports.Result{Outcome: ports.OutcomeFailed, Err: failed}) {
		t.Fatalf("result = %+v, want failed with the load error", r.result)
	}
}

func TestQueryTreatsNilInstrumentationAsInert(t *testing.T) {
	got, err := application.Query[lookup](context.Background(), nil, application.AllowAll[lookup](), "things.Find", findThing{id: "t-1"},
		func(context.Context) (string, error) { return "snapshot", nil })

	if err != nil || got != "snapshot" {
		t.Fatalf("Query = (%q, %v), want (\"snapshot\", nil)", got, err)
	}
}
