package reference_test

import (
	"slices"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
)

func newPartner(c *cycle, opts ...serviceskit.PartnerOption) *serviceskit.Partner {
	return serviceskit.NewPartner(clock.New(c.clock.Now()), &ids.Sequence{Prefix: "o-"}, opts...)
}

func TestTheExternalCycleAgainstTheScriptedPartner(t *testing.T) {
	tests := []struct {
		name          string
		opts          []serviceskit.PartnerOption
		run           func(t *testing.T, c *cycle, p *serviceskit.Partner) []application.Disposition
		wantDisp      []application.Disposition
		wantSteps     []application.Step
		wantSubmitted []ports.OutboundKey
		wantQueried   []ports.OutboundKey
	}{
		{
			name: "a retryable submit is resent under the same key and its acceptance applies once",
			run: func(t *testing.T, c *cycle, p *serviceskit.Partner) []application.Disposition {
				p.Script(cycleKey, serviceskit.Reply{Class: ports.ClassRetryableTransport}, serviceskit.Reply{Class: ports.ClassAccepted, Applied: true})
				if _, err := c.dispatchVia(p, "m-a"); err == nil {
					t.Fatal("Dispatch(retryable) = nil, want the transient failure that asks for redelivery")
				}
				redelivered, err := c.dispatchVia(p, "m-a")
				if err != nil {
					t.Fatalf("Dispatch(redelivery) = %v", err)
				}
				return []application.Disposition{redelivered}
			},
			wantDisp:      []application.Disposition{application.R1D1},
			wantSteps:     []application.Step{application.StepConfirm},
			wantSubmitted: []ports.OutboundKey{cycleKey, cycleKey},
		},
		{
			name: "a reply lost after the effect is confirmed by the query and its redelivery calls nobody",
			run: func(t *testing.T, c *cycle, p *serviceskit.Partner) []application.Disposition {
				p.Script(cycleKey, serviceskit.LostAfterEffect()).ScriptQuery(cycleKey, serviceskit.Answer{FromEffect: true})
				first, err := c.dispatchVia(p, "m-a")
				if err != nil {
					t.Fatalf("Dispatch() = %v", err)
				}
				answer, err := p.Query(c.ctx, cycleKey)
				if err != nil || answer.Finding != ports.FindingApplied {
					t.Fatalf("Query() = %+v, %v, want the applied effect", answer, err)
				}
				if got := c.observe(t, answer); got != ports.ObserveRecorded {
					t.Fatalf("Resolve(query) = %v, want recorded", got)
				}
				redelivered, err := c.dispatchVia(p, "m-a")
				if err != nil {
					t.Fatalf("Dispatch(redelivery) = %v", err)
				}
				return []application.Disposition{first, redelivered}
			},
			wantDisp:      []application.Disposition{application.R1D1, application.R2},
			wantSteps:     []application.Step{application.StepQuery, application.StepConfirm},
			wantSubmitted: []ports.OutboundKey{cycleKey},
			wantQueried:   []ports.OutboundKey{cycleKey},
		},
		{
			name: "a webhook delivered twice applies its first copy and reads the second as duplicate",
			run: func(t *testing.T, c *cycle, p *serviceskit.Partner) []application.Disposition {
				p.Script(cycleKey, serviceskit.LostAfterEffect())
				disposition, err := c.dispatchVia(p, "m-a")
				if err != nil {
					t.Fatalf("Dispatch() = %v", err)
				}
				first, second := p.DuplicateNotification(cycleKey, ports.FindingApplied, "ref-1")
				if got := []ports.ObserveResult{c.observe(t, first), c.observe(t, second)}; !slices.Equal(got, []ports.ObserveResult{ports.ObserveRecorded, ports.ObserveDuplicate}) {
					t.Fatalf("resolutions = %v, want recorded then duplicate", got)
				}
				return []application.Disposition{disposition}
			},
			wantDisp:      []application.Disposition{application.R1D1},
			wantSteps:     []application.Step{application.StepQuery, application.StepConfirm},
			wantSubmitted: []ports.OutboundKey{cycleKey},
		},
		{
			name: "an unanswered call to a partner without a query is reconciled, never queried",
			opts: []serviceskit.PartnerOption{serviceskit.WithoutQuery()},
			run: func(t *testing.T, c *cycle, p *serviceskit.Partner) []application.Disposition {
				p.Script(cycleKey, serviceskit.Reply{Class: ports.ClassUnknownSent})
				disposition, err := c.dispatchVia(p, "m-a")
				if err != nil {
					t.Fatalf("Dispatch() = %v", err)
				}
				return []application.Disposition{disposition}
			},
			wantDisp:      []application.Disposition{application.R1D1},
			wantSteps:     []application.Step{application.StepReconcile},
			wantSubmitted: []ports.OutboundKey{cycleKey},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCycle(t)
			p := newPartner(c, tt.opts...)

			got := tt.run(t, c, p)

			if !slices.Equal(got, tt.wantDisp) {
				t.Errorf("dispositions = %v, want %v", got, tt.wantDisp)
			}
			if !slices.Equal(c.steps, tt.wantSteps) {
				t.Errorf("applied steps = %v, want %v", c.steps, tt.wantSteps)
			}
			if submitted := p.Submitted(); !slices.Equal(submitted, tt.wantSubmitted) {
				t.Errorf("submitted = %v, want %v", submitted, tt.wantSubmitted)
			}
			if queried := p.Queried(); !slices.Equal(queried, tt.wantQueried) {
				t.Errorf("queried = %v, want %v", queried, tt.wantQueried)
			}
			if exhausted := p.Exhausted(); len(exhausted) != 0 {
				t.Errorf("exhausted = %v, want every call scripted", exhausted)
			}
		})
	}
}
