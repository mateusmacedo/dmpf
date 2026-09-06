package metrics

// Kind is the instrument shape of a series, so the catalogue documents what a
// reader will find and a construction error is caught by a test.
type Kind string

const (
	Counter   Kind = "counter"
	Gauge     Kind = "gauge"
	Histogram Kind = "histogram"
)

// Unit values follow the OpenTelemetry convention: "1" is dimensionless.
const (
	UnitDimensionless = "1"
	UnitSeconds       = "s"
)

// Threshold is the invariant a series must respect. It is nil for a counter
// with no invariant of its own: MET-05 asks for a threshold where one is
// derivable from the baseline, not for one everywhere.
type Threshold struct {
	Condition string
}

// Metric declares a series: the name a dashboard queries, the unit, the shape,
// the formula that gives it meaning and who answers for it.
type Metric struct {
	Name      string
	Unit      string
	Kind      Kind
	Formula   string
	Labels    []string
	Threshold *Threshold
	Owner     string
}

// Names of the platform series (MET-02). They are constants because a
// dashboard, an alert and a test all reference the same string.
const (
	RetriesTotal            = "dmpf_dependency_retries_total"
	BudgetExhaustedTotal    = "dmpf_dependency_budget_exhausted_total"
	BreakerState            = "dmpf_dependency_breaker_state"
	DeadlineExceededTotal   = "dmpf_dependency_deadline_exceeded_total"
	CancellationsTotal      = "dmpf_dependency_cancellations_total"
	RequestDurationSeconds  = "dmpf_service_request_duration_seconds"
	RequestsTotal           = "dmpf_service_requests_total"
	ErrorsTotal             = "dmpf_service_errors_total"
	DegradedTotal           = "dmpf_service_degraded_total"
	OmittedTotal            = "dmpf_service_omitted_total"
	BulkheadRejectionsTotal = "dmpf_dependency_bulkhead_rejections_total"
	SpansDroppedTotal       = "dmpf_otel_spans_dropped_total"
)

const ownerService = "serviço"

// Catalog is the ten mandatory series of MET-02 plus the two local ones: the
// bulkhead rejections and the spans the processor had to drop. Returning a copy
// keeps a caller from rewriting the catalogue it is reading.
func Catalog() []Metric {
	catalogue := []Metric{
		{
			Name:    RetriesTotal,
			Unit:    UnitDimensionless,
			Kind:    Counter,
			Formula: "soma das tentativas repetidas por dependência e categoria de erro",
			Labels:  []string{KeyDependency, KeyErrorCategory},
			Owner:   ownerService,
		},
		{
			Name:    BudgetExhaustedTotal,
			Unit:    UnitDimensionless,
			Kind:    Counter,
			Formula: "soma das execuções que esgotaram o orçamento de retry, uma vez por execução",
			Labels:  []string{KeyDependency},
			Owner:   ownerService,
		},
		{
			Name:      BreakerState,
			Unit:      UnitDimensionless,
			Kind:      Gauge,
			Formula:   "estado atual do disjuntor: 0 fechado, 1 meio-aberto, 2 aberto",
			Labels:    []string{KeyDependency},
			Threshold: &Threshold{Condition: "aberto por mais de um cooldown"},
			Owner:     ownerService,
		},
		{
			Name:    DeadlineExceededTotal,
			Unit:    UnitDimensionless,
			Kind:    Counter,
			Formula: "soma das chamadas que estouraram o prazo efetivo",
			Labels:  []string{KeyDependency, KeyOperation},
			Owner:   ownerService,
		},
		{
			Name:    CancellationsTotal,
			Unit:    UnitDimensionless,
			Kind:    Counter,
			Formula: "soma das chamadas encerradas por cancelamento do chamador",
			Labels:  []string{KeyDependency, KeyOperation},
			Owner:   ownerService,
		},
		{
			Name:    RequestDurationSeconds,
			Unit:    UnitSeconds,
			Kind:    Histogram,
			Formula: "distribuição da duração das operações do serviço, em segundos",
			Labels:  []string{KeyService, KeyOperation, KeyOutcomeCategory},
			Owner:   ownerService,
		},
		{
			Name:    RequestsTotal,
			Unit:    UnitDimensionless,
			Kind:    Counter,
			Formula: "soma das operações do serviço por categoria de desfecho",
			Labels:  []string{KeyService, KeyOperation, KeyOutcomeCategory},
			Owner:   ownerService,
		},
		{
			Name:    ErrorsTotal,
			Unit:    UnitDimensionless,
			Kind:    Counter,
			Formula: "soma das operações que falharam tecnicamente, por categoria de erro",
			Labels:  []string{KeyService, KeyOperation, KeyErrorCategory},
			Owner:   ownerService,
		},
		{
			Name:    DegradedTotal,
			Unit:    UnitDimensionless,
			Kind:    Counter,
			Formula: "soma das respostas entregues em modo degradado",
			Labels:  []string{KeyDependency},
			Owner:   ownerService,
		},
		{
			Name:    OmittedTotal,
			Unit:    UnitDimensionless,
			Kind:    Counter,
			Formula: "soma das dependências omitidas da resposta por decisão de degradação",
			Labels:  []string{KeyDependency},
			Owner:   ownerService,
		},
		{
			Name:    BulkheadRejectionsTotal,
			Unit:    UnitDimensionless,
			Kind:    Counter,
			Formula: "soma das chamadas recusadas por saturação do bulkhead",
			Labels:  []string{KeyDependency},
			Owner:   ownerService,
		},
		{
			Name:    SpansDroppedTotal,
			Unit:    UnitDimensionless,
			Kind:    Counter,
			Formula: "soma dos spans descartados pelo processador por fila cheia",
			Labels:  []string{KeyService},
			Owner:   ownerService,
		},
	}
	return catalogue
}
