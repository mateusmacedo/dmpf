package fitness

// Action is one of the four transitions of FND-07 §3.3.
type Action string

const (
	Preserve   Action = "preservar"
	Regenerate Action = "regenerar"
	Reject     Action = "rejeitar"
	Reduce     Action = "reduzir"
)

// Boundary is one of the four crossings CTX-11 governs.
type Boundary string

const (
	Ingress    Boundary = "ingress"
	FanOut     Boundary = "fan-out"
	Retry      Boundary = "retry"
	Downstream Boundary = "downstream"
)

// Crossing is one cell of the CTX-11 matrix: the action the norm fixes for a
// field at a boundary, whether the norm adds a refusal of the input value, and
// the executable proof of the realization ("path::Test", relative to the root).
// ProofEnvelope says the proof is structural: the envelope has no such attribute.
type Crossing struct {
	Field         string
	Boundary      Boundary
	Action        Action
	RejectsInput  bool
	Proof         string
	ProofEnvelope bool
}

const (
	bffIdentity   = "apps/backend/bff/api/identity_test.go::"
	bffHandlers   = "apps/backend/bff/api/handlers_test.go::"
	ordersRPC     = "apps/backend/orders/app/rpc/interceptors_test.go::"
	consumerCtx   = "libs/backend/go/app/consumer_test.go::TestTheHandlerReceivesTheContextRebuiltFromTheEnvelope"
	relayRecord   = "libs/backend/go/app/relay/record_test.go::"
	kernelHTTP    = "libs/backend/go/http/identity_test.go::"
	consumerGold  = "libs/backend/go/app/consumer_golden_test.go::"
	authnIdentity = "libs/backend/go/authn/identity_test.go::"
	grpcTwoHops   = "libs/backend/go/grpc/two_hops_test.go::"
)

// Traversal is the matrix of FND-07 §3.3, transcribed by hand from
// docs/dmpf/contexto-erros-seguranca.md: nine fields by four boundaries.
var Traversal = []Crossing{
	{Field: "request_id", Boundary: Ingress, Action: Regenerate, Proof: bffIdentity + "TestTheEdgeMintsItsRequestIDAndSendsItAsTheCausation"},
	{Field: "request_id", Boundary: FanOut, Action: Reduce, ProofEnvelope: true},
	{Field: "request_id", Boundary: Retry, Action: Regenerate, Proof: consumerCtx},
	{Field: "request_id", Boundary: Downstream, Action: Regenerate, Proof: ordersRPC + "TestTheServerRebuildsTheExecutionContextFromTheMetadata"},

	{Field: "correlation_id", Boundary: Ingress, Action: Preserve, Proof: bffHandlers + "TestTheEdgeSpanParentsTheClientSpanAndTheMetadata"},
	{Field: "correlation_id", Boundary: FanOut, Action: Preserve, Proof: relayRecord + "TestAssembleMapsTheOutboxColumnsOntoTheEnvelope"},
	{Field: "correlation_id", Boundary: Retry, Action: Preserve, Proof: consumerCtx},
	{Field: "correlation_id", Boundary: Downstream, Action: Preserve, Proof: ordersRPC + "TestTheServerRebuildsTheExecutionContextFromTheMetadata"},

	{Field: "causation_id", Boundary: Ingress, Action: Regenerate, Proof: bffIdentity + "TestTheEdgeMintsItsRequestIDAndSendsItAsTheCausation"},
	{Field: "causation_id", Boundary: FanOut, Action: Regenerate, Proof: ordersRPC + "TestMetadataBecomesTheMessageContextOfTheOutbox"},
	{Field: "causation_id", Boundary: Retry, Action: Preserve, Proof: consumerCtx},
	{Field: "causation_id", Boundary: Downstream, Action: Regenerate, Proof: bffIdentity + "TestTheEdgeMintsItsRequestIDAndSendsItAsTheCausation"},

	{Field: "trace_context", Boundary: Ingress, Action: Preserve, Proof: bffHandlers + "TestAnIncomingTraceparentIsContinued"},
	{Field: "trace_context", Boundary: FanOut, Action: Preserve, Proof: ordersRPC + "TestMetadataBecomesTheMessageContextOfTheOutbox"},
	{Field: "trace_context", Boundary: Retry, Action: Preserve, Proof: consumerCtx},
	{Field: "trace_context", Boundary: Downstream, Action: Preserve, Proof: ordersRPC + "TestTheServerSpanContinuesThePropagatedTrace"},

	{Field: "authenticated_subject", Boundary: Ingress, Action: Regenerate, RejectsInput: true, Proof: kernelHTTP + "TestAnAssertedIdentityThatDivergesIsRefused"},
	{Field: "authenticated_subject", Boundary: FanOut, Action: Reduce, ProofEnvelope: true},
	{Field: "authenticated_subject", Boundary: Retry, Action: Regenerate, Proof: consumerCtx},
	{Field: "authenticated_subject", Boundary: Downstream, Action: Reduce, Proof: bffIdentity + "TestTheTenantCrossesTheFanOutAndTheSubjectNeverDoes"},

	{Field: "tenant_id", Boundary: Ingress, Action: Regenerate, RejectsInput: true, Proof: bffIdentity + "TestAHeaderAssertingAnotherTenantIsRefused"},
	{Field: "tenant_id", Boundary: FanOut, Action: Preserve, Proof: relayRecord + "TestAssembleCarriesTheTenantOntoTheEnvelope"},
	{Field: "tenant_id", Boundary: Retry, Action: Preserve, Proof: consumerGold + "TestEveryCanonicalCaseRebuildsTheTenantItCarries"},
	{Field: "tenant_id", Boundary: Downstream, Action: Preserve, Proof: bffIdentity + "TestTheTenantCrossesTheFanOutAndTheSubjectNeverDoes"},

	{Field: "permissions", Boundary: Ingress, Action: Regenerate, Proof: authnIdentity + "TestDevAuthenticatorResolvesTheIdentityTheCredentialDeclares"},
	{Field: "permissions", Boundary: FanOut, Action: Reduce, ProofEnvelope: true},
	{Field: "permissions", Boundary: Retry, Action: Regenerate, Proof: consumerCtx},
	{Field: "permissions", Boundary: Downstream, Action: Reduce, Proof: ordersRPC + "TestMetadataAssertingASubjectResolvesNone"},

	{Field: "deadline", Boundary: Ingress, Action: Regenerate, Proof: bffHandlers + "TestARequestWithoutDeadlineReachesTheContextBelowTheRouteBudget"},
	{Field: "deadline", Boundary: FanOut, Action: Reduce, ProofEnvelope: true},
	{Field: "deadline", Boundary: Retry, Action: Regenerate, Proof: consumerCtx},
	{Field: "deadline", Boundary: Downstream, Action: Reduce, Proof: grpcTwoHops + "TestTwoHopsNeverRestartTheDeadline"},

	{Field: "locale", Boundary: Ingress, Action: Preserve, Proof: bffIdentity + "TestTheDeclaredLocaleCrossesTheFanOut"},
	{Field: "locale", Boundary: FanOut, Action: Reduce, ProofEnvelope: true},
	{Field: "locale", Boundary: Retry, Action: Preserve, Proof: consumerCtx},
	{Field: "locale", Boundary: Downstream, Action: Preserve, Proof: ordersRPC + "TestTheServerPreservesTheLocaleTheEdgeResolved"},
}
