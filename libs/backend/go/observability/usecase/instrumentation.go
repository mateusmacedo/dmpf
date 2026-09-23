package usecase

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/audit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/otelboot"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

const (
	spanPrefix = "dmpf.usecase."

	attrTrafficClass    = "dmpf.traffic_class"
	attrOutcomeCategory = "dmpf.outcome_category"

	trafficWrite = "write"
	trafficRead  = "read"
)

// ActionCrossTenantAccess names the security event of IDN-12 in the audit trail.
const ActionCrossTenantAccess = "security.cross_tenant_access"

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

func (i *Instrumentation) BeginOperation(ctx context.Context, operation string) (context.Context, ports.EndOperation) {
	started := i.clock.Now()
	ctx, span := i.tracer.Start(ctx, spanPrefix+operation, trace.WithAttributes(
		attribute.String(attrTrafficClass, i.trafficClass(operation)),
	))

	return ctx, func(result ports.Result) {
		span.SetAttributes(attribute.String(attrOutcomeCategory, string(result.Outcome)))
		if result.Outcome == ports.OutcomeFailed {
			// TRC-12: o status carrega a categoria do desfecho, nunca a mensagem
			// do erro, que sairia do processo sem passar por redaction.
			span.SetStatus(codes.Error, "")
		}
		span.End()

		i.record(ctx, operation, result, i.clock.Now().Sub(started))
		i.recordCrossTenant(ctx, result)
	}
}

// WHY: every use case closes here, so the provider's report of a cross-tenant
// access becomes a security event in one place instead of one per edge. The
// load-or-create paths swallow it on purpose: creating in one's own tenant is
// not an access.
func (i *Instrumentation) recordCrossTenant(ctx context.Context, result ports.Result) {
	var access ports.CrossTenantAccess
	if result.Outcome != ports.OutcomeFailed || !errors.As(result.Err, &access) {
		return
	}
	i.emit(ctx, audit.Event{
		Subject:    carrierSubject(ctx),
		Object:     access.Object,
		Action:     ActionCrossTenantAccess,
		Outcome:    string(ports.OutcomeDenied),
		At:         ports.Instant(i.clock.Now().UnixNano()),
		Tenant:     string(access.ContextTenant),
		DataTenant: string(access.DataTenant),
	})
}

// record writes the three service series of MET-08, MET-09 and MET-10. Every
// outcome counts as a request, including a failure; only a failure also counts
// as an error, because a rejection is the refusing branch of the UPR and not a
// fault (DEC-04).
func (i *Instrumentation) record(ctx context.Context, operation string, result ports.Result, elapsed time.Duration) {
	labels := metrics.Labels{}.
		Service(i.service).
		Operation(operation).
		OutcomeCategory(string(result.Outcome))

	// Both series carry the same labels, so the attribute set is built once: it
	// is on the path of every instrumented operation.
	measured := metric.WithAttributes(labels.Attributes()...)
	i.instruments.RequestDuration.Record(ctx, elapsed.Seconds(), measured)
	i.instruments.Requests.Add(ctx, 1, measured)

	if result.Outcome != ports.OutcomeFailed {
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

func (i *Instrumentation) Audit(ctx context.Context, event ports.AuditEvent) {
	i.emit(ctx, audit.Event{
		Subject: i.resolveSubject(ctx),
		Object:  event.Object,
		Action:  event.Action,
		Outcome: string(event.Outcome),
		At:      event.At,
	})
}

func (i *Instrumentation) emit(ctx context.Context, event audit.Event) {
	if i.sink == nil {
		return
	}
	if err := i.sink.Emit(ctx, event); err != nil {
		// A porta não devolve erro, e engolir este seria perder um registro de
		// auditoria em silêncio. Sai a categoria, não o conteúdo do evento.
		i.logger.ErrorContext(ctx, "dmpf: the audit sink rejected the record",
			slog.String("error_category", "audit_sink"),
			slog.String("action", event.Action))
	}
}

// carrierSubject leaves a platform chain's absent subject absent (IDN-20).
func carrierSubject(ctx context.Context) string {
	execution, ok := ports.ExecutionContextFrom(ctx)
	if !ok {
		return ""
	}
	subject, _ := execution.Subject()
	return string(subject)
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
