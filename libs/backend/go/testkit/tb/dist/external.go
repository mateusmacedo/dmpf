package dist

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/execctx"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb/pg"
)

const RoleExternalCrash Role = "external-crash"

const (
	CodeOrphanNotClosed = "DMPF-A001"
	CodeResent          = "DMPF-A002"
	CodeStepNotOnce     = "DMPF-A003"
)

const (
	externalConsumer   = "dist.external"
	externalKey        = ports.OutboundKey("k-dist-crash")
	externalMessage    = ports.MessageID("m-dist-crash")
	externalStartedAt  = ports.Instant(1_000)
	externalStaleAfter = int64(time.Minute)
	externalSettle     = 30 * time.Second
	externalInboxWait  = 5 * time.Second
	externalSweepBatch = 10

	submitInFlight = "dist: external submit in flight"
)

var ExternalAttemptPool = pg.Options{
	Project:      "distexternal",
	Capabilities: []postgres.Capability{postgres.Inbox, postgres.ExternalAttempt},
}

func NewPostgres(t testing.TB, s Spec) *Harness {
	t.Helper()
	h := &Harness{Spec: s, Pool: pg.OpenPool(t, s.Pool)}
	pg.GrantRuntime(t, h.Pool, s.Pool)
	return h
}

type externalResources struct {
	ledger ports.AttemptLedger
	inbox  ports.Inbox
}

func externalDispatch(pool *pgxpool.Pool, clk ports.Clock, command ports.ExternalCommand, apply func(context.Context, externalResources, application.ExternalStep) error) application.ExternalDispatch[externalResources] {
	return application.ExternalDispatch[externalResources]{
		UoW:        externalUoW(pool),
		Store:      postgres.NewAttemptStore(pool),
		Ledger:     func(res externalResources) ports.AttemptLedger { return res.ledger },
		Inbox:      func(res externalResources) ports.Inbox { return res.inbox },
		Receipt:    ports.Receipt{Consumer: externalConsumer, MessageID: externalMessage, MessageType: "partner.Submit", PayloadHash: "h", ReceivedAt: externalStartedAt},
		Command:    command,
		Key:        externalKey,
		Profile:    externalProfile(),
		StaleAfter: externalStaleAfter,
		Clock:      clk,
		SettleContext: func(ctx context.Context) (context.Context, context.CancelFunc) {
			return context.WithTimeout(ctx, externalSettle)
		},
		Instrumentation: ports.NoInstrumentation(),
		Apply:           apply,
	}
}

func externalUoW(pool *pgxpool.Pool) ports.UnitOfWork[externalResources] {
	return postgres.NewUnitOfWork(pool, func(tx *postgres.Tx) externalResources {
		return externalResources{ledger: tx.AttemptLedger(), inbox: tx.Inbox(externalConsumer, externalInboxWait)}
	})
}

func externalProfile() ports.ExternalProfile {
	return ports.ExternalProfile{
		KeySemantics: ports.KeyNative, Efficacy: ports.EfficacyOnSubmit, MaxQueries: 3,
		KeyAlphabet: "0123456789abcdef", KeyMaxLength: 32,
	}
}

type hangingCommand struct{}

func (hangingCommand) Submit(context.Context, ports.OutboundKey, []byte) (ports.ExternalClass, ports.Observation, error) {
	fmt.Println(submitInFlight)
	select {}
}

// SubmitThatNeverReturns is the child of CrashBetweenBeginAndFinishVector: its
// partner call blocks for good once Begin committed, so only the kill ends it.
func SubmitThatNeverReturns(t *testing.T, ctx context.Context) {
	t.Helper()
	pool, err := pgxpool.New(ctx, os.Getenv(pg.PostgresDSN))
	if err != nil {
		t.Fatalf("dist: pgxpool.New: %v", err)
	}
	defer pool.Close()
	noStep := func(context.Context, externalResources, application.ExternalStep) error { return nil }
	_, err = application.Dispatch(execctx.WithExecution(t, ctx), externalDispatch(pool, clock.New(externalStartedAt), hangingCommand{}, noStep))
	t.Fatalf("dist: Dispatch returned %v from a submit that never returns", err)
}

type AttemptRow struct {
	N     int
	State string
	Class ports.ExternalClass
}

type CrashOutcome struct {
	Attempts []AttemptRow
	Steps    []application.ExternalStep
	// Submits counts the partner calls of both processes: the child's before the
	// kill and the redelivery's after the sweep.
	Submits int
}

// CrashBetweenBeginAndFinishVector is the process that dies with its partner
// call in flight (RES-43): the sweep closes the attempt as UnknownSent and
// applies its step once, and neither the sweep nor a redelivery resends.
func CrashBetweenBeginAndFinishVector(t testing.TB, h *Harness) Verdict {
	t.Helper()
	child := h.Start(t, RoleExternalCrash)
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("%s wrote: %s", child.Name(), child.Output())
		}
	})
	WaitFor(t, "the child to call the partner after Begin", 60*time.Second, func() bool {
		select {
		case <-child.Done():
			t.Fatalf("dist: %s exited before the kill: %v\n%s", child.Name(), child.Err(), child.Output())
		default:
		}
		return strings.Contains(child.Output(), submitInFlight)
	})
	child.Kill()
	if rows := h.attempts(t); len(rows) != 1 || rows[0].State != "begun" {
		t.Fatalf("dist: the kill left attempts %+v, want one begun: the vector needs an orphan to sweep", rows)
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), pg.RuntimeConfig(t, h.Spec.Pool.Project))
	if err != nil {
		t.Fatalf("dist: pgxpool.NewWithConfig: %v", err)
	}
	defer pool.Close()
	ctx := execctx.WithExecution(t, context.Background())
	clk := clock.New(externalStartedAt + ports.Instant(externalStaleAfter) + 1)
	var steps []application.ExternalStep
	apply := func(_ context.Context, _ externalResources, step application.ExternalStep) error {
		steps = append(steps, step)
		return nil
	}

	for range 2 {
		sweepExternal(t, ctx, pool, clk, apply)
	}
	partner := serviceskit.NewPartner(clk, &ids.Sequence{Prefix: "obs-"})
	if _, err := application.Dispatch(ctx, externalDispatch(pool, clk, partner, apply)); err != nil {
		t.Fatalf("dist: the redelivery after the sweep = %v", err)
	}

	return DecideCrash(CrashOutcome{
		Attempts: h.attempts(t),
		Steps:    steps,
		Submits:  strings.Count(child.Output(), submitInFlight) + len(partner.Submitted()),
	})
}

func sweepExternal(t testing.TB, ctx context.Context, pool *pgxpool.Pool, clk ports.Clock, apply func(context.Context, externalResources, application.ExternalStep) error) {
	t.Helper()
	abandon := func(ctx context.Context, attempt ports.Attempt) error {
		_, err := application.Abandon(ctx, application.ExternalAbandon[externalResources]{
			UoW:             externalUoW(pool),
			Ledger:          func(res externalResources) ports.AttemptLedger { return res.ledger },
			Attempt:         attempt,
			Profile:         externalProfile(),
			Clock:           clk,
			Instrumentation: ports.NoInstrumentation(),
			Apply:           apply,
		})
		return err
	}
	cutoff := clk.Now() - ports.Instant(externalStaleAfter)
	if _, err := application.SweepStale(ctx, postgres.NewAttemptStore(pool), cutoff, ports.AttemptCursor{}, externalSweepBatch, abandon); err != nil {
		t.Fatalf("dist: SweepStale = %v", err)
	}
}

func (h *Harness) attempts(t testing.TB) []AttemptRow {
	t.Helper()
	rows, err := h.Pool.Query(context.Background(),
		"SELECT attempt, state, coalesce(class, 0) FROM external_attempt WHERE outbound_key = $1 ORDER BY attempt", string(externalKey))
	if err != nil {
		t.Fatalf("dist: reading the attempts: %v", err)
	}
	attempts, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (AttemptRow, error) {
		var a AttemptRow
		var class int16
		err := row.Scan(&a.N, &a.State, &class)
		a.Class = ports.ExternalClass(class)
		return a, err
	})
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("dist: reading the attempts: %v", err)
	}
	return attempts
}

func DecideCrash(o CrashOutcome) Verdict {
	var v Verdict
	if len(o.Attempts) != 1 || o.Attempts[0].State != "finished" || o.Attempts[0].Class != ports.ClassUnknownSent {
		v.Diagnostics = append(v.Diagnostics, Diagnostic{
			Code:   CodeOrphanNotClosed,
			Detail: fmt.Sprintf("attempts %+v, want the one the kill left finished as UnknownSent", o.Attempts),
		})
	}
	if o.Submits != 1 {
		v.Diagnostics = append(v.Diagnostics, Diagnostic{
			Code:   CodeResent,
			Detail: fmt.Sprintf("the partner was called %d times, want only the call the kill interrupted", o.Submits),
		})
	}
	if len(o.Steps) != 1 || o.Steps[0].Class != ports.ClassUnknownSent {
		v.Diagnostics = append(v.Diagnostics, Diagnostic{
			Code:   CodeStepNotOnce,
			Detail: fmt.Sprintf("steps %+v, want the step of UnknownSent applied once", o.Steps),
		})
	}
	return v
}
