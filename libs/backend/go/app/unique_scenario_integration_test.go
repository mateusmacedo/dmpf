//go:build integration

package app_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	eventv1 "github.com/mateusmacedo/dmpf/libs/backend/go/contracts/gen/go/dmpf/testing/v1"
	"github.com/mateusmacedo/dmpf/libs/backend/go/domain"
	kernelgrpc "github.com/mateusmacedo/dmpf/libs/backend/go/grpc"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

const namedSchema = `CREATE TABLE IF NOT EXISTS named (
  tenant_id text   NOT NULL,
  named_id  text   NOT NULL,
  version   bigint NOT NULL DEFAULT 0,
  name      text   NOT NULL,
  CONSTRAINT named_pkey PRIMARY KEY (tenant_id, named_id),
  CONSTRAINT named_tenant_id_name_key UNIQUE (tenant_id, name)
)`

type namedState struct{ Name string }

type named struct{ name string }

func (n *named) Snapshot() namedState { return namedState{Name: n.name} }

type namedRegistered struct{ id string }

func (namedRegistered) EventName() string { return "named.registered" }

type namedMapper struct{}

func (namedMapper) Map(event domain.DomainEvent) (postgres.Mapped, error) {
	registered, ok := event.(namedRegistered)
	if !ok {
		return postgres.Mapped{}, postgres.ErrUnmappedEvent
	}
	return postgres.Mapped{Message: &eventv1.OrderPlaced{OrderId: registered.id}, Type: "com.company.named.named-registered.v1"}, nil
}

var namedTable = postgres.Table[string, namedState]{
	Name:     "named",
	IDColumn: "named_id",
	Columns:  []string{"name"},
	Encode:   func(s namedState) ([]any, error) { return []any{s.Name}, nil },
	Decode: func(scan func(dest ...any) error) (namedState, error) {
		var s namedState
		err := scan(&s.Name)
		return s, err
	},
	Unique: []postgres.Unique{{
		Name:      "named_tenant_id_name_key",
		Columns:   []string{"tenant_id", "name"},
		Rejection: domain.Reject("app/named/duplicate-name", "the name is taken"),
	}},
}

func scopedToTenant(t *testing.T) context.Context {
	t.Helper()
	subject, tenant := ports.SubjectID("s-1"), ports.TenantID("acme")
	execution, err := ports.NewExecutionContext(ports.ExecutionContextSpec{
		RequestID: "r-1", CorrelationID: "c-1", TraceContext: "t-1",
		Subject: &subject, Tenant: &tenant, Permissions: []ports.Permission{},
		Deadline: ports.Instant(time.Now().Add(time.Minute).UnixNano()), Locale: "en",
	})
	if err != nil {
		t.Fatalf("NewExecutionContext() = %v", err)
	}
	return ports.WithExecutionContext(context.Background(), execution)
}

func register(ctx context.Context, tx *postgres.Tx, id, name string) (application.Outcome[string], error) {
	origin := application.Origin{Destination: "named.events", AggregateType: "named", AggregateID: id}
	identity := application.Identity{OccurredAt: 1, MessageIDs: []ports.MessageID{ports.MessageID("evt-" + id)}}
	return application.Decide(ctx, namedTable.Repository(tx), tx.Outbox(namedMapper{}), origin, id, identity,
		application.OrNew(func(string) *named { return &named{name: name} }, func(s namedState) *named { return &named{name: s.Name} }),
		func(*named) (domain.Accepted[string], *domain.Rejection) {
			return domain.Accept(id, namedRegistered{id: id}), nil
		})
}

func TestADeclaredViolationAfterAnotherWriteRollsTheWholeTransactionBack(t *testing.T) {
	pool := pg.OpenPool(t, pg.Options{Project: "app", Capabilities: []postgres.Capability{postgres.Outbox, postgres.Inbox}, Schemas: []string{namedSchema}})
	ctx := scopedToTenant(t)
	uow := postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) *postgres.Tx { return tx })
	if err := uow.Within(ctx, func(ctx context.Context, tx *postgres.Tx) error {
		_, err := register(ctx, tx, "x", "taken")
		return err
	}); err != nil {
		t.Fatalf("seed = %v", err)
	}
	seed(t, pool, "DELETE FROM outbox")

	err := uow.Within(ctx, func(ctx context.Context, tx *postgres.Tx) error {
		if _, err := tx.CommandInbox("named.commands", time.Second).Register(ctx, ports.Receipt{
			Consumer: "named.commands", MessageID: "cmd-1", MessageType: "named.Register", PayloadHash: "h", ReceivedAt: 1, ExpiresAt: 2,
		}); err != nil {
			return err
		}
		if outcome, err := register(ctx, tx, "a", "free"); err != nil {
			return err
		} else if _, rejected := outcome.Rejection(); rejected {
			t.Fatalf("register(a) = %+v, want accepted", outcome)
		}
		_, err := register(ctx, tx, "b", "taken")
		return err
	})

	if !errors.Is(err, ports.ErrUniqueViolation) {
		t.Fatalf("Within() = %v, want ErrUniqueViolation", err)
	}
	if _, ok := application.DeclaredRejection(err); ok {
		t.Fatalf("DeclaredRejection(%v) = true, want false: converting would keep the earlier write (DEC-10)", err)
	}
	if got := application.CategoryOf(err); got != string(application.Conflict) {
		t.Fatalf("CategoryOf() = %q, want Conflict", got)
	}
	if got := status.Code(kernelgrpc.StatusOf(err)); got != codes.AlreadyExists {
		t.Fatalf("StatusOf() = %v, want AlreadyExists", got)
	}
	if got := remaining(t, pool, "SELECT named_id FROM named"); len(got) != 1 || got[0] != "x" {
		t.Fatalf("named = %v, want only the seeded x: the save of a was rolled back", got)
	}
	if got := remaining(t, pool, "SELECT message_id FROM outbox"); len(got) != 0 {
		t.Fatalf("outbox = %v, want none", got)
	}
	if got := remaining(t, pool, "SELECT message_id FROM inbox"); len(got) != 0 {
		t.Fatalf("inbox = %v, want none", got)
	}
}
