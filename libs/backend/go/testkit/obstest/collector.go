package obstest

import (
	"context"
	"net"
	"sync"
	"testing"

	collogspb "go.opentelemetry.io/proto/otlp/collector/logs/v1"
	colmetricpb "go.opentelemetry.io/proto/otlp/collector/metrics/v1"
	coltracepb "go.opentelemetry.io/proto/otlp/collector/trace/v1"
	logspb "go.opentelemetry.io/proto/otlp/logs/v1"
	resourcepb "go.opentelemetry.io/proto/otlp/resource/v1"
	"google.golang.org/grpc"
)

type Exports struct {
	mu        sync.Mutex
	resources map[string]*resourcepb.Resource
	logs      []*logspb.LogRecord
}

func (e *Exports) record(signal string, resource *resourcepb.Resource) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.resources[signal] = resource
}

func (e *Exports) keep(records []*logspb.LogRecord) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.logs = append(e.logs, records...)
}

func (e *Exports) LogRecords() []*logspb.LogRecord {
	e.mu.Lock()
	defer e.mu.Unlock()
	return append([]*logspb.LogRecord(nil), e.logs...)
}

// Identity is the resource of the last export of signal ("traces", "metrics"
// or "logs") as string attributes, and false when nothing arrived for it.
func (e *Exports) Identity(signal string) (map[string]string, bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	resource, ok := e.resources[signal]
	if !ok {
		return nil, false
	}
	identity := map[string]string{}
	for _, attribute := range resource.GetAttributes() {
		identity[attribute.GetKey()] = attribute.GetValue().GetStringValue()
	}
	return identity, true
}

type traceSink struct {
	coltracepb.UnimplementedTraceServiceServer
	got *Exports
}

func (s traceSink) Export(_ context.Context, request *coltracepb.ExportTraceServiceRequest) (*coltracepb.ExportTraceServiceResponse, error) {
	for _, spans := range request.GetResourceSpans() {
		s.got.record("traces", spans.GetResource())
	}
	return &coltracepb.ExportTraceServiceResponse{}, nil
}

type metricSink struct {
	colmetricpb.UnimplementedMetricsServiceServer
	got *Exports
}

func (s metricSink) Export(_ context.Context, request *colmetricpb.ExportMetricsServiceRequest) (*colmetricpb.ExportMetricsServiceResponse, error) {
	for _, metrics := range request.GetResourceMetrics() {
		s.got.record("metrics", metrics.GetResource())
	}
	return &colmetricpb.ExportMetricsServiceResponse{}, nil
}

type logSink struct {
	collogspb.UnimplementedLogsServiceServer
	got *Exports
}

func (s logSink) Export(_ context.Context, request *collogspb.ExportLogsServiceRequest) (*collogspb.ExportLogsServiceResponse, error) {
	for _, logs := range request.GetResourceLogs() {
		s.got.record("logs", logs.GetResource())
		for _, scope := range logs.GetScopeLogs() {
			s.got.keep(scope.GetLogRecords())
		}
	}
	return &collogspb.ExportLogsServiceResponse{}, nil
}

// StartCollector serves an OTLP/gRPC receiver of the three signals on a free
// loopback port until the test ends.
func StartCollector(t testing.TB) (string, *Exports) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen() = %v", err)
	}
	got := &Exports{resources: map[string]*resourcepb.Resource{}}
	server := grpc.NewServer()
	coltracepb.RegisterTraceServiceServer(server, traceSink{got: got})
	colmetricpb.RegisterMetricsServiceServer(server, metricSink{got: got})
	collogspb.RegisterLogsServiceServer(server, logSink{got: got})
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(server.Stop)
	return listener.Addr().String(), got
}
