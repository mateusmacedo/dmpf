package obstest_test

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	otellog "go.opentelemetry.io/otel/log"
	sdklog "go.opentelemetry.io/otel/sdk/log"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

func TestMemoryLogsKeepsEveryRecordItWasHanded(t *testing.T) {
	logs := &obstest.MemoryLogs{}
	provider := sdklog.NewLoggerProvider(sdklog.WithProcessor(sdklog.NewSimpleProcessor(logs)))
	var record otellog.Record
	record.SetBody(attribute.StringValue("hello"))
	record.SetEventName("probe")
	record.AddAttributes(attribute.String("tenant", "acme"))
	provider.Logger("obstest").Emit(context.Background(), record)

	if names := logs.EventNames(); len(names) != 1 || names[0] != "probe" {
		t.Fatalf("EventNames() = %v, want [probe]", names)
	}
	if maps := logs.Maps(); len(maps) != 1 || maps[0]["msg"] != "hello" || maps[0]["tenant"] != "acme" || maps[0]["scope"] != "obstest" {
		t.Fatalf("Maps() = %v, want the body, the attribute and the scope", maps)
	}
}

func TestReadEnvFileSkipsCommentsAndBlankLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte("# comment\n\nOTEL_SERVICE_NAME=orders\nPG_DSN=postgres://x\n"), 0o600); err != nil {
		t.Fatalf("WriteFile() = %v", err)
	}

	env := obstest.ReadEnvFile(t, path)

	if len(env) != 2 || env["OTEL_SERVICE_NAME"] != "orders" || env["PG_DSN"] != "postgres://x" {
		t.Fatalf("ReadEnvFile() = %v, want the two assignments", env)
	}
}

func TestAnUnreachableCollectorRefusesTheConnection(t *testing.T) {
	down := obstest.UnreachableCollector(t)

	conn, err := net.DialTimeout("tcp", down.Addr, time.Second)
	if err == nil {
		_ = conn.Close()
	}
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Fatalf("dial %s = %v, want connection refused", down.Addr, err)
	}
}

func TestAStallingCollectorAcceptsAndNeverAnswers(t *testing.T) {
	down := obstest.StallingCollector(t)

	conn, err := net.DialTimeout("tcp", down.Addr, time.Second)
	if err != nil {
		t.Fatalf("dial %s = %v, want the connection accepted", down.Addr, err)
	}
	defer func() { _ = conn.Close() }()
	_ = conn.SetReadDeadline(time.Now().Add(100 * time.Millisecond))
	var timeout net.Error
	if n, err := conn.Read(make([]byte, 1)); n != 0 || !errors.As(err, &timeout) || !timeout.Timeout() {
		t.Fatalf("Read() = %d, %v, want the read deadline to expire on an open connection", n, err)
	}
}

func TestMetricLabelsReadsTheGaugesOfTheReader(t *testing.T) {
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	gauge, err := provider.Meter("obstest").Int64Gauge("probe.gauge")
	if err != nil {
		t.Fatalf("Int64Gauge() = %v", err)
	}
	gauge.Record(context.Background(), 3)

	labels := obstest.MetricLabels(t, reader)

	if sets, ok := labels["probe.gauge"]; !ok || len(sets) != 1 || sets[0].Len() != 0 {
		t.Fatalf("MetricLabels() = %v, want one unlabelled point of probe.gauge", labels)
	}
}
