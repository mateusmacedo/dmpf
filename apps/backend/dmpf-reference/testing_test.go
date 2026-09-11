//go:build integration

package dmpfreference_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"

	dmpfreference "github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference"
	"github.com/mateusmacedo/dmpf/apps/backend/dmpf-reference/api"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/tb"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-testkit/tb/pg"
)

const (
	e2eTimeout  = 90 * time.Second
	pollEvery   = 100 * time.Millisecond
	adminWindow = 30 * time.Second
)

// The runtime is booted once for the whole binary: otelboot starts once per
// process, and every role of the e2e shares it, which is how a process
// hosting the three roles would.
var (
	runtimeOnce sync.Once
	runtime     *otelboot.Runtime
	runtimeErr  error
)

func bootRuntime(t *testing.T) *otelboot.Runtime {
	t.Helper()
	runtimeOnce.Do(func() {
		cfg := dmpfreference.Defaults(dmpfreference.RoleAPI)
		cfg.Instance = "e2e"
		runtime, runtimeErr = dmpfreference.NewTelemetry(context.Background(), cfg, io.Discard)
	})
	if runtimeErr != nil {
		t.Fatalf("NewTelemetry() = %v", runtimeErr)
	}
	return runtime
}

// harness is the e2e over the Postgres and the Redpanda of the environment:
// tables reset by the test kit, topics unique to this run, one runtime for the
// three roles, and the logs of each role kept for the failure report.
type harness struct {
	pool    *pgxpool.Pool
	rt      *otelboot.Runtime
	admin   *kadm.Client
	brokers string
	topic   string
	group   string
	dlq     string

	mu   sync.Mutex
	logs bytes.Buffer
}

func newHarness(t *testing.T) *harness {
	t.Helper()

	brokers := tb.Env(t, "DMPF_KAFKA_BROKERS")
	pool := pg.OpenPool(t)
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	h := &harness{
		pool:    pool,
		rt:      bootRuntime(t),
		brokers: brokers,
		topic:   "dmpf-reference-" + suffix,
		group:   "dmpf-reference-group-" + suffix,
		dlq:     "dmpf-reference-" + suffix + "-dlq",
	}
	reservationsTopic, reservationsDLQ := h.topic+"-reservations", h.topic+"-reservations-dlq"

	cl, err := kgo.NewClient(kgo.SeedBrokers(strings.Split(brokers, ",")...))
	if err != nil {
		t.Fatalf("kgo.NewClient() = %v", err)
	}
	t.Cleanup(cl.Close)
	h.admin = kadm.NewClient(cl)

	ctx, cancel := context.WithTimeout(context.Background(), adminWindow)
	defer cancel()
	topics := []string{h.topic, h.dlq, reservationsTopic, reservationsDLQ}
	if _, err := h.admin.CreateTopics(ctx, 1, 1, nil, topics...); err != nil {
		t.Fatalf("CreateTopics() = %v", err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), adminWindow)
		defer cancel()
		_, _ = h.admin.DeleteTopics(ctx, topics...)
	})
	t.Cleanup(func() {
		if t.Failed() {
			t.Logf("role logs:\n%s", h.logs.String())
		}
	})
	return h
}

func (h *harness) Write(p []byte) (int, error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.logs.Write(p)
}

func (h *harness) config(t *testing.T, role dmpfreference.Role) dmpfreference.Config {
	t.Helper()
	cfg, err := dmpfreference.FromEnv(role, env(
		"DMPF_PG_DSN", tb.Env(t, "DMPF_PG_DSN"),
		"DMPF_KAFKA_BROKERS", h.brokers,
		"DMPF_KAFKA_INSECURE", "true",
		"DMPF_KAFKA_TOPIC", h.topic,
		"DMPF_KAFKA_GROUP", h.group,
		"DMPF_KAFKA_DLQ", h.dlq,
		"DMPF_KAFKA_RESERVATIONS_TOPIC", h.topic+"-reservations",
		"DMPF_KAFKA_RESERVATIONS_DLQ", h.topic+"-reservations-dlq",
		"DMPF_INSTANCE_ID", "e2e",
		"DMPF_ITEM_LIMIT", "3",
	))
	if err != nil {
		t.Fatalf("FromEnv(%s) = %v", role, err)
	}
	cfg.Relay.Interval = pollEvery
	return cfg
}

// startRole runs a role in a goroutine over the shared runtime until the test
// ends; a role that stops on its own, or with an error, fails the test.
func (h *harness) startRole(t *testing.T, role dmpfreference.Role) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- dmpfreference.RunWith(ctx, h.config(t, role), h.rt, h) }()
	t.Cleanup(func() {
		cancel()
		select {
		case err := <-done:
			if err != nil {
				t.Errorf("%s: RunWith() = %v, want nil after cancellation", role, err)
			}
		case <-time.After(adminWindow):
			t.Errorf("%s did not stop within %v of cancellation", role, adminWindow)
		}
	})
}

// apiServer is the api role behind an httptest listener: the same service,
// admission and handler serveAPI mounts, on a port the test can reach.
func (h *harness) apiServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := h.config(t, dmpfreference.RoleAPI)
	ctrl, err := dmpfreference.NewAdmission(cfg.Admission)
	if err != nil {
		t.Fatalf("NewAdmission() = %v", err)
	}
	handler, err := api.NewHandler(dmpfreference.NewOrdersService(h.pool, h.rt, cfg, h), ctrl, h.rt.Tracer(), h.rt.Instruments(), api.Options{Budget: cfg.Budget})
	if err != nil {
		t.Fatalf("NewHandler() = %v", err)
	}
	server := httptest.NewServer(handler)
	t.Cleanup(server.Close)
	return server
}

func (h *harness) call(t *testing.T, method, url, body string) (int, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest() = %v", err)
	}
	if method == http.MethodPost {
		req.Header.Set("Idempotency-Key", fmt.Sprintf("k-%d", time.Now().UnixNano()))
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer res.Body.Close()
	got, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatalf("ReadAll() = %v", err)
	}
	return res.StatusCode, got
}

func (h *harness) waitUntil(t *testing.T, what string, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(e2eTimeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(pollEvery)
	}
	t.Fatalf("timed out after %v waiting until %s", e2eTimeout, what)
}

func (h *harness) count(t *testing.T, table string) int {
	t.Helper()
	return h.countWhere(t, table, "true")
}

func (h *harness) countWhere(t *testing.T, table, where string) int {
	t.Helper()
	var n int
	if err := h.pool.QueryRow(context.Background(), "SELECT count(*) FROM "+table+" WHERE "+where).Scan(&n); err != nil {
		t.Fatalf("count(%s where %s) = %v", table, where, err)
	}
	return n
}

type outboxRow struct {
	MessageID   string
	Status      string
	PayloadHash string
	Metadata    map[string]string
}

func (h *harness) outboxRow(t *testing.T, messageType string) (outboxRow, bool) {
	t.Helper()
	var (
		row outboxRow
		raw []byte
	)
	err := h.pool.QueryRow(context.Background(),
		"SELECT message_id, status, payload_hash, metadata FROM dmpf_outbox WHERE message_type = $1", messageType,
	).Scan(&row.MessageID, &row.Status, &row.PayloadHash, &raw)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return outboxRow{}, false
		}
		t.Fatalf("outbox row of %s: %v", messageType, err)
	}
	if err := json.Unmarshal(raw, &row.Metadata); err != nil {
		t.Fatalf("metadata %s of %s: %v", raw, messageType, err)
	}
	return row, true
}

func (h *harness) inboxRow(t *testing.T, messageID string) (status, payloadHash string, ok bool) {
	t.Helper()
	err := h.pool.QueryRow(context.Background(),
		"SELECT status, payload_hash FROM dmpf_inbox WHERE message_id = $1", messageID,
	).Scan(&status, &payloadHash)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			return "", "", false
		}
		t.Fatalf("inbox row of %s: %v", messageID, err)
	}
	return status, payloadHash, true
}

// consumedEverything is true once the group's committed offset reached the end
// of the topic and the topic holds at least published records: the consumer
// took every delivery, the ones it subscribes to and the ones it only acks.
func (h *harness) consumedEverything(t *testing.T, published int64) bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), adminWindow)
	defer cancel()

	ends, err := h.admin.ListEndOffsets(ctx, h.topic)
	if err != nil {
		return false
	}
	var end int64
	ends.Each(func(o kadm.ListedOffset) { end += o.Offset })
	if end < published {
		return false
	}

	committed, err := h.admin.FetchOffsets(ctx, h.group)
	if err != nil {
		return false
	}
	var at int64
	committed.Each(func(o kadm.OffsetResponse) {
		if o.Topic == h.topic && o.At > 0 {
			at += o.At
		}
	})
	return at >= end
}
