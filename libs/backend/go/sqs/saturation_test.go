package sqs_test

import (
	"context"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/sqs"
	sqstypes "github.com/aws/aws-sdk-go-v2/service/sqs/types"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/sqs"
)

func exportedInstruments(t *testing.T) (*metrics.Instruments, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	meters := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader), sdkmetric.WithView(otelboot.NewMetricView()))
	t.Cleanup(func() { _ = meters.Shutdown(context.Background()) })
	instruments, err := metrics.New(meters.Meter("sqs_test"))
	if err != nil {
		t.Fatalf("metrics.New() = %v", err)
	}
	return instruments, reader
}

func exportedLabels(t *testing.T, reader *sdkmetric.ManualReader) map[string][]attribute.Set {
	t.Helper()
	var collected metricdata.ResourceMetrics
	if err := reader.Collect(context.Background(), &collected); err != nil {
		t.Fatalf("Collect() = %v", err)
	}
	labels := map[string][]attribute.Set{}
	for _, scope := range collected.ScopeMetrics {
		for _, m := range scope.Metrics {
			switch data := m.Data.(type) {
			case metricdata.Gauge[float64]:
				for _, point := range data.DataPoints {
					labels[m.Name] = append(labels[m.Name], point.Attributes)
				}
			case metricdata.Gauge[int64]:
				for _, point := range data.DataPoints {
					labels[m.Name] = append(labels[m.Name], point.Attributes)
				}
			}
		}
	}
	return labels
}

func TestTheSaturationOfTheConsumerIsExportedWithoutALabel(t *testing.T) {
	instruments, reader := exportedInstruments(t)
	api := provider.NewFakeSQS()
	sink := newSink(func(ctx context.Context, _ handled, ack ports.Acknowledger) error { return ack.Ack(ctx) })
	consumer := newConsumer(clock.NewFake(start), sink)
	consumer.Config.Instruments = instruments
	stop := run(t, consumer, api)

	raw, _ := validRaw(t, "k1")
	api.Deliver(&sqs.ReceiveMessageOutput{Messages: []sqstypes.Message{message("rh-1", provider.EncodeBody(raw), "1")}})
	awaitCalls(t, api, "DeleteMessage", 1)
	stop()

	exported := exportedLabels(t, reader)
	for _, name := range []string{metrics.PoolUtilization, metrics.QueueDepth} {
		points := exported[name]
		if len(points) == 0 {
			t.Errorf("%s did not reach the export, want one point per receive (MET-11)", name)
		}
		for _, set := range points {
			if set.Len() != 0 {
				t.Errorf("%s carries %v, want no label: the catalogue declares none and service is on the resource (RF-D3)", name, set.ToSlice())
			}
		}
	}
}
