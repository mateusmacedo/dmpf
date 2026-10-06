package metrics

// Kind is the instrument shape of a series, so the catalogue documents what a
// reader will find and a construction error is caught by a test.
type Kind string

const (
	Counter   Kind = "counter"
	Gauge     Kind = "gauge"
	Histogram Kind = "histogram"
)

// Unit values are UCUM, as OpenTelemetry declares them: "1" is only for a
// ratio, a count names what it counts in braces (RF-D1).
const (
	UnitDimensionless = "1"
	UnitSeconds       = "s"
	UnitRetry         = "{retry}"
	UnitExecution     = "{execution}"
	UnitCall          = "{call}"
	UnitResponse      = "{response}"
	UnitRequest       = "{request}"
	UnitMessage       = "{message}"
	UnitState         = "{state}"
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

// Names of the platform series (MET-02, RF-D1), in the OpenTelemetry form: the
// exporter adds the unit and the type suffix. They are constants because a
// dashboard, an alert and a test all reference the same string.
const (
	RetriesTotal             = "dmpf.dependency.retries"
	BudgetExhaustedTotal     = "dmpf.dependency.budget.exhausted"
	BreakerState             = "dmpf.dependency.breaker.state"
	DeadlineExceededTotal    = "dmpf.dependency.deadline_exceeded"
	CancellationsTotal       = "dmpf.dependency.cancellations"
	RequestDurationSeconds   = "dmpf.operation.duration"
	DegradedTotal            = "dmpf.dependency.degraded"
	OmittedTotal             = "dmpf.dependency.omitted"
	BulkheadRejectionsTotal  = "dmpf.dependency.bulkhead.rejections"
	PoolUtilization          = "dmpf.consumer.pool.utilization"
	QueueDepth               = "dmpf.consumer.queue.depth"
	AdmissionRejectionsTotal = "dmpf.admission.rejections"
)

const ownerService = "serviço"

// Catalog is the series of RF-D1: the dependency series of MET-02, the duration
// of the use case and the three of MET-11 and MET-12 the transport providers
// record. Returning a copy keeps a caller from rewriting the catalogue.
func Catalog() []Metric {
	catalogue := []Metric{
		{
			Name:    RetriesTotal,
			Unit:    UnitRetry,
			Kind:    Counter,
			Formula: "soma das tentativas repetidas por dependência e categoria de erro",
			Labels:  []string{KeyDependency, KeyErrorType},
			Owner:   ownerService,
		},
		{
			Name:    BudgetExhaustedTotal,
			Unit:    UnitExecution,
			Kind:    Counter,
			Formula: "soma das execuções que esgotaram o orçamento de retry, uma vez por execução",
			Labels:  []string{KeyDependency},
			Owner:   ownerService,
		},
		{
			Name:      BreakerState,
			Unit:      UnitState,
			Kind:      Gauge,
			Formula:   "estado atual do disjuntor: 0 fechado, 1 meio-aberto, 2 aberto",
			Labels:    []string{KeyDependency},
			Threshold: &Threshold{Condition: "aberto por mais de um cooldown"},
			Owner:     ownerService,
		},
		{
			Name:    DeadlineExceededTotal,
			Unit:    UnitCall,
			Kind:    Counter,
			Formula: "soma das chamadas que estouraram o prazo efetivo",
			Labels:  []string{KeyDependency, KeyOperation},
			Owner:   ownerService,
		},
		{
			Name:    CancellationsTotal,
			Unit:    UnitCall,
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
			Labels:  []string{KeyOperation, KeyOutcomeCategory, KeyErrorType},
			Owner:   ownerService,
		},
		{
			Name:    DegradedTotal,
			Unit:    UnitResponse,
			Kind:    Counter,
			Formula: "soma das respostas entregues em modo degradado",
			Labels:  []string{KeyDependency},
			Owner:   ownerService,
		},
		{
			Name:    OmittedTotal,
			Unit:    UnitResponse,
			Kind:    Counter,
			Formula: "soma das dependências omitidas da resposta por decisão de degradação",
			Labels:  []string{KeyDependency},
			Owner:   ownerService,
		},
		{
			Name:    BulkheadRejectionsTotal,
			Unit:    UnitCall,
			Kind:    Counter,
			Formula: "soma das chamadas recusadas por saturação do bulkhead",
			Labels:  []string{KeyDependency},
			Owner:   ownerService,
		},
		{
			Name:    PoolUtilization,
			Unit:    UnitDimensionless,
			Kind:    Gauge,
			Formula: "ocupação média do pool de trabalho na janela dividida pela capacidade declarada (razão)",
			Owner:   ownerService,
		},
		{
			Name:    QueueDepth,
			Unit:    UnitMessage,
			Kind:    Gauge,
			Formula: "profundidade atual da fila interna de trabalho (contagem)",
			Owner:   ownerService,
		},
		{
			Name:    AdmissionRejectionsTotal,
			Unit:    UnitRequest,
			Kind:    Counter,
			Formula: "soma das recusas por admissão, por rota e por tenant declarado na allowlist",
			Labels:  []string{KeyRoute, KeyRPCMethod, KeyTenant},
			Owner:   ownerService,
		},
	}
	return catalogue
}
