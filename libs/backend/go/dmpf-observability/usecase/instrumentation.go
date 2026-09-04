package usecase

import (
	"context"
	"log/slog"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/audit"
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

const (
	spanPrefix = "dmpf.usecase."

	attrTrafficClass    = "dmpf.traffic_class"
	attrOutcomeCategory = "dmpf.outcome_category"

	trafficWrite = "write"
	trafficRead  = "read"
)

// SubjectFunc resolves the authenticated subject from the context. The identity
// of FND-07 has no realization in the kernel, so the composition root injects
// this; an absent subject is recorded as absent, never invented.
type SubjectFunc func(ctx context.Context) string

// Instrumentation realizes the instrumentation port over OpenTelemetry: it opens
// the use case span, closes it with the outcome category and forwards the audit
// record to a sink.
type Instrumentation struct {
	tracer  trace.Tracer
	sink    audit.Sink
	subject SubjectFunc
	reads   map[string]struct{}
}

// New builds the realization. readOperations names the operations that carry read
// traffic; every other operation is write. The names come from the use case
// package as exported constants, so the composition root declares the class
// instead of this package guessing it from the operation string.
func New(tracer trace.Tracer, sink audit.Sink, subject SubjectFunc, readOperations ...string) *Instrumentation {
	reads := make(map[string]struct{}, len(readOperations))
	for _, operation := range readOperations {
		reads[operation] = struct{}{}
	}
	return &Instrumentation{tracer: tracer, sink: sink, subject: subject, reads: reads}
}

func (i *Instrumentation) BeginOperation(ctx context.Context, operation string) (context.Context, dmpfports.EndOperation) {
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
	}
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
		slog.ErrorContext(ctx, "dmpf: the audit sink rejected the record",
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
