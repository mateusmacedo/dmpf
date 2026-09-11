package usecase

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-observability/otelboot"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

const (
	spanPrefix = "dmpf.usecase."

	attrTrafficClass    = "dmpf.traffic_class"
	attrOutcomeCategory = "dmpf.outcome_category"

	trafficWrite = "write"
	trafficRead  = "read"
)

// CategoryUnclassified is what a failure is counted under when nobody says what
// kind of failure it was. It is a category and never the error message, which
// would put unbounded — and possibly personal — text on a label (MET-07).
const CategoryUnclassified = "unclassified"

// Classifier names the kind of a technical failure, so dmpf_service_errors_total
// carries a bounded error_category. It answers with a category, which is what
// separates it from retry.Classifier: that one answers whether an error is worth
// another attempt. The taxonomy belongs to FND-07 and is injected, never defined
// here; a nil classifier, or one that says nothing, resolves to
// CategoryUnclassified.
type Classifier func(err error) string

// SubjectFunc resolves the authenticated subject from the context. The identity
// of FND-07 has no realization in the kernel, so the composition root injects
// this; an absent subject is recorded as absent, never invented.
type SubjectFunc func(ctx context.Context) string

// Instrumentation realizes the instrumentation port over OpenTelemetry: it opens
// the use case span, closes it with the outcome category and forwards the audit
// record to a sink.
type Instrumentation struct {
	tracer      trace.Tracer
	instruments *metrics.Instruments
	service     string
	logger      *slog.Logger
	clock       clock.Clock
	sink        audit.Sink
	subject     SubjectFunc
	classify    Classifier
	reads       map[string]struct{}
}

// New builds the realization over a started runtime, which supplies the tracer,
// the platform instruments and the service name the three series are labelled
// with. readOperations names the operations that carry read traffic; every other
// operation is write. The names come from the use case package as exported
// constants, so the composition root declares the class instead of this package
// guessing it from the operation string.
func New(rt *otelboot.Runtime, sink audit.Sink, subject SubjectFunc, classify Classifier, readOperations ...string) *Instrumentation {
	reads := make(map[string]struct{}, len(readOperations))
	for _, operation := range readOperations {
		reads[operation] = struct{}{}
	}
	return &Instrumentation{
		tracer:      rt.Tracer(),
		instruments: rt.Instruments(),
		service:     rt.ServiceName(),
		logger:      rt.Logger(),
		clock:       clock.System(),
		sink:        sink,
		subject:     subject,
		classify:    classify,
		reads:       reads,
	}
}

func (i *Instrumentation) BeginOperation(ctx context.Context, operation string) (context.Context, dmpfports.EndOperation) {
	started := i.clock.Now()
	ctx, span := i.tracer.Start(ctx, spanPrefix+operation, trace.WithAttributes(
		attribute.String(attrTrafficClass, i.trafficClass(operation)),
	))

	return ctx, func(result dmpfports.Result) {
		span.SetAttributes(attribute.String(attrOutcomeCategory, string(result.Outcome)))
		if result.Outcome == dmpfports.OutcomeFailed {
			// TRC-12: o status carrega a categoria do desfecho, nunca a mensagem
			// do erro, que sairia do processo sem passar por redaction.
			span.SetStatus(codes.Error, "")
		}
		span.End()

		i.record(ctx, operation, result, i.clock.Now().Sub(started))
	}
}

// record writes the three service series of MET-08, MET-09 and MET-10. Every
// outcome counts as a request, including a failure; only a failure also counts
// as an error, because a rejection is the refusing branch of the UPR and not a
// fault (DEC-04).
func (i *Instrumentation) record(ctx context.Context, operation string, result dmpfports.Result, elapsed time.Duration) {
	labels := metrics.Labels{}.
		Service(i.service).
		Operation(operation).
		OutcomeCategory(string(result.Outcome))

	// Both series carry the same labels, so the attribute set is built once: it
	// is on the path of every instrumented operation.
	measured := metric.WithAttributes(labels.Attributes()...)
	i.instruments.RequestDuration.Record(ctx, elapsed.Seconds(), measured)
	i.instruments.Requests.Add(ctx, 1, measured)

	if result.Outcome != dmpfports.OutcomeFailed {
		return
	}

	failure := metrics.Labels{}.
		Service(i.service).
		Operation(operation).
		ErrorCategory(i.category(result.Err))

	i.instruments.Errors.Add(ctx, 1, metric.WithAttributes(failure.Attributes()...))
}

func (i *Instrumentation) category(err error) string {
	if i.classify == nil {
		return CategoryUnclassified
	}
	if category := i.classify(err); category != "" {
		return category
	}
	return CategoryUnclassified
}

func (i *Instrumentation) Audit(ctx context.Context, event dmpfports.AuditEvent) {
	if i.sink == nil {
		return
	}

	err := i.sink.Emit(ctx, audit.Event{
		Subject: i.resolveSubject(ctx),
		Object:  event.Object,
		Action:  event.Action,
		Outcome: string(event.Outcome),
		At:      event.At,
	})
	if err != nil {
		// A porta não devolve erro, e engolir este seria perder um registro de
		// auditoria em silêncio. Sai a categoria, não o conteúdo do evento.
		i.logger.ErrorContext(ctx, "dmpf: the audit sink rejected the record",
			slog.String("error_category", "audit_sink"),
			slog.String("action", event.Action))
	}
}

func (i *Instrumentation) trafficClass(operation string) string {
	if _, isRead := i.reads[operation]; isRead {
		return trafficRead
	}
	return trafficWrite
}

func (i *Instrumentation) resolveSubject(ctx context.Context) string {
	if i.subject == nil {
		return ""
	}
	return i.subject(ctx)
}
