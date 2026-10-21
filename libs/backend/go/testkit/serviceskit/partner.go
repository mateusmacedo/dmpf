package serviceskit

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/stable"
)

const PartnerProvider = "partner"

var (
	ErrNoQuery = errors.New("serviceskit: partner has no state query")

	// ErrScriptExhausted comes with a zero class from Submit, the provider defect
	// the port names, so a call the test did not foresee cannot pass as an outcome.
	ErrScriptExhausted = errors.New("serviceskit: partner script exhausted")
)

// Reply is one scripted Submit: Applied marks the effect as done on the partner
// whatever the class says, and a zero Finding returns no observation.
type Reply struct {
	Class       ports.ExternalClass
	Finding     ports.Finding
	Applied     bool
	ExternalRef string
	Err         error
}

func LostAfterEffect() Reply {
	return Reply{Class: ports.ClassUnknownSent, Applied: true}
}

type Answer struct {
	Finding     ports.Finding
	FromEffect  bool
	ExternalRef string
	Err         error
}

type PartnerOption func(*Partner)

func WithProvider(name string) PartnerOption { return func(p *Partner) { p.provider = name } }

func WithoutQuery() PartnerOption { return func(p *Partner) { p.noQuery = true } }

type Partner struct {
	clock    ports.Clock
	ids      ports.IDGenerator
	provider string
	noQuery  bool

	mu      sync.Mutex
	replies map[ports.OutboundKey][]Reply
	answers map[ports.OutboundKey][]Answer
	applied map[ports.OutboundKey]bool

	submitted stable.Sequence[ports.OutboundKey]
	queried   stable.Sequence[ports.OutboundKey]
	exhausted stable.Sequence[ports.OutboundKey]
}

func NewPartner(clock ports.Clock, ids ports.IDGenerator, opts ...PartnerOption) *Partner {
	p := &Partner{
		clock:    clock,
		ids:      ids,
		provider: PartnerProvider,
		replies:  map[ports.OutboundKey][]Reply{},
		answers:  map[ports.OutboundKey][]Answer{},
		applied:  map[ports.OutboundKey]bool{},
	}
	for _, opt := range opts {
		opt(p)
	}
	return p
}

func (p *Partner) Script(key ports.OutboundKey, replies ...Reply) *Partner {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.replies[key] = append(p.replies[key], replies...)
	return p
}

func (p *Partner) ScriptQuery(key ports.OutboundKey, answers ...Answer) *Partner {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.answers[key] = append(p.answers[key], answers...)
	return p
}

func (p *Partner) Submit(_ context.Context, key ports.OutboundKey, _ []byte) (ports.ExternalClass, ports.Observation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.submitted.Append(key)
	queue := p.replies[key]
	if len(queue) == 0 {
		p.exhausted.Append(key)
		return 0, ports.Observation{}, fmt.Errorf("%w: submit %q", ErrScriptExhausted, key)
	}
	reply := queue[0]
	p.replies[key] = queue[1:]
	if reply.Applied {
		p.applied[key] = true
	}
	var obs ports.Observation
	if reply.Finding != 0 {
		obs = p.observe(key, reply.Finding, reply.ExternalRef)
	}
	return reply.Class, obs, reply.Err
}

func (p *Partner) Query(_ context.Context, key ports.OutboundKey) (ports.Observation, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queried.Append(key)
	if p.noQuery {
		return ports.Observation{}, ErrNoQuery
	}
	queue := p.answers[key]
	if len(queue) == 0 {
		p.exhausted.Append(key)
		return ports.Observation{}, fmt.Errorf("%w: query %q", ErrScriptExhausted, key)
	}
	answer := queue[0]
	p.answers[key] = queue[1:]
	if answer.Err != nil {
		return ports.Observation{}, answer.Err
	}
	finding := answer.Finding
	if answer.FromEffect {
		finding = ports.FindingAbsent
		if p.applied[key] {
			finding = ports.FindingApplied
		}
	}
	return p.observe(key, finding, answer.ExternalRef), nil
}

// DuplicateNotification is one webhook delivered twice: both copies share the
// observation ID, which is what Resolve recognizes as a duplicate.
func (p *Partner) DuplicateNotification(key ports.OutboundKey, finding ports.Finding, externalRef string) (first, second ports.Observation) {
	p.mu.Lock()
	defer p.mu.Unlock()
	obs := p.observe(key, finding, externalRef)
	return obs, obs
}

func (p *Partner) QueryAvailable() bool { return !p.noQuery }

func (p *Partner) Applied(key ports.OutboundKey) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.applied[key]
}

func (p *Partner) Submitted() []ports.OutboundKey { return p.submitted.Items() }

func (p *Partner) Queried() []ports.OutboundKey { return p.queried.Items() }

func (p *Partner) Exhausted() []ports.OutboundKey { return p.exhausted.Items() }

func (p *Partner) observe(key ports.OutboundKey, finding ports.Finding, externalRef string) ports.Observation {
	return ports.Observation{
		Key:         key,
		Finding:     finding,
		SourceAt:    p.clock.Now(),
		Provider:    p.provider,
		ExternalRef: externalRef,
		ID:          string(p.ids.NewMessageID()),
	}
}
