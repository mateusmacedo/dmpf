package kafka_test

import (
	"context"
	"testing"

	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"github.com/mateusmacedo/dmpf/libs/backend/go/kafka"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/obstest"
)

func exportedInstruments(t *testing.T) (*metrics.Instruments, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	meters := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithView(otelboot.NewMetricView()))
	t.Cleanup(func() { _ = meters.Shutdown(context.Background()) })
	instruments, err := metrics.New(meters.Meter("kafka_test"))
	if err != nil {
		t.Fatalf("metrics.New() = %v", err)
	}
	return instruments, reader
}

func TestTheSaturationOfTheConsumerIsExportedWithoutALabel(t *testing.T) {
	instruments, reader := exportedInstruments(t)
	sink := newSink()
	sink.on(0, alwaysAck)
	fake := kafka.NewFakeClient()
	fake.Feed(topic, 0, recordsAt(0)...)
	c := newConsumer(sink)
	c.Config.Instruments = instruments

	consume(t, c, fake, func() bool { return len(fake.Commits()) >= 1 && c.PendingOf(topic, 0) == 0 })

	exported := obstest.MetricLabels(t, reader)
	for _, name := range []string{metrics.PoolUtilization, metrics.QueueDepth} {
		points := exported[name]
		if len(points) == 0 {
			t.Errorf("%s did not reach the export, want one point per poll (MET-11)", name)
		}
		for _, set := range points {
			if set.Len() != 0 {
				t.Errorf("%s carries %v, want no label: the catalogue declares none and service is on the resource (RF-D3)", name, set.ToSlice())
			}
		}
	}
}
