package serviceskit_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/ids"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/serviceskit"
)

const partnerStart = ports.Instant(1_755_432_000_000_000_000)

func newPartner(opts ...serviceskit.PartnerOption) (*serviceskit.Partner, *clock.Fake) {
	c := clock.New(partnerStart)
	return serviceskit.NewPartner(c, &ids.Sequence{Prefix: "obs-"}, opts...), c
}

func TestPartnerAnswersEachOfTheNineClasses(t *testing.T) {
	classes := []ports.ExternalClass{
		ports.ClassAccepted,
		ports.ClassRetryableTransport,
		ports.ClassRetryableUpstream,
		ports.ClassFinalValidation,
		ports.ClassFinalBusiness,
		ports.ClassFinalAuthorization,
		ports.ClassUnknownSent,
		ports.ClassUnknownAmbiguous,
		ports.ClassSchemaMismatch,
	}
	for _, class := range classes {
		t.Run(class.String(), func(t *testing.T) {
			p, _ := newPartner()
			key := ports.OutboundKey("k-" + class.String())
			cause := errors.New("cause")
			p.Script(key, serviceskit.Reply{Class: class, Err: cause})

			got, obs, err := p.Submit(context.Background(), key, []byte("payload"))
			if got != class {
				t.Fatalf("class = %v, want %v", got, class)
			}
			if !errors.Is(err, cause) {
				t.Fatalf("err = %v, want the scripted cause", err)
			}
			if obs != (ports.Observation{}) {
				t.Fatalf("observation = %+v, want none without a scripted finding", obs)
			}
		})
	}
}

func TestPartnerFollowsTheScriptAttemptByAttempt(t *testing.T) {
	p, _ := newPartner()
	key := ports.OutboundKey("k-1")
	p.Script(key,
		serviceskit.Reply{Class: ports.ClassRetryableUpstream},
		serviceskit.Reply{Class: ports.ClassAccepted, Finding: ports.FindingApplied, Applied: true, ExternalRef: "ref-1"},
	)

	first, _, _ := p.Submit(context.Background(), key, nil)
	second, obs, err := p.Submit(context.Background(), key, nil)
	if first != ports.ClassRetryableUpstream || second != ports.ClassAccepted || err != nil {
		t.Fatalf("attempts = %v, %v (err %v), want retryable_upstream then accepted", first, second, err)
	}
	want := ports.Observation{
		Key: key, Finding: ports.FindingApplied, SourceAt: partnerStart,
		Provider: serviceskit.PartnerProvider, ExternalRef: "ref-1", ID: "obs-000001",
	}
	if obs != want {
		t.Fatalf("observation = %+v, want %+v", obs, want)
	}
	if !p.Applied(key) {
		t.Fatal("Applied = false after an attempt that applied the effect")
	}
}

func TestPartnerKeepsEachKeyOnItsOwnScript(t *testing.T) {
	p, _ := newPartner()
	p.Script("a", serviceskit.Reply{Class: ports.ClassFinalBusiness})
	p.Script("b", serviceskit.Reply{Class: ports.ClassAccepted})

	gotB, _, _ := p.Submit(context.Background(), "b", nil)
	gotA, _, _ := p.Submit(context.Background(), "a", nil)
	if gotA != ports.ClassFinalBusiness || gotB != ports.ClassAccepted {
		t.Fatalf("a = %v, b = %v, want final_business and accepted", gotA, gotB)
	}
	if p.Applied("a") || p.Applied("b") {
		t.Fatal("Applied = true without a scripted effect")
	}
}

func TestPartnerQueryAnswersEachFinding(t *testing.T) {
	findings := []ports.Finding{
		ports.FindingApplied,
		ports.FindingAbsent,
		ports.FindingRejected,
		ports.FindingPending,
		ports.FindingInconclusive,
	}
	for _, finding := range findings {
		t.Run(finding.String(), func(t *testing.T) {
			p, c := newPartner(serviceskit.WithProvider("acme"))
			key := ports.OutboundKey("k-" + finding.String())
			p.ScriptQuery(key, serviceskit.Answer{Finding: finding, ExternalRef: "ref"})
			c.Advance(5)

			obs, err := p.Query(context.Background(), key)
			if err != nil {
				t.Fatalf("Query: %v", err)
			}
			want := ports.Observation{
				Key: key, Finding: finding, SourceAt: partnerStart + 5,
				Provider: "acme", ExternalRef: "ref", ID: "obs-000001",
			}
			if obs != want {
				t.Fatalf("observation = %+v, want %+v", obs, want)
			}
		})
	}
}

func TestPartnerQueryCarriesTheScriptedError(t *testing.T) {
	p, _ := newPartner()
	cause := errors.New("timeout")
	p.ScriptQuery("k", serviceskit.Answer{Err: cause})

	obs, err := p.Query(context.Background(), "k")
	if !errors.Is(err, cause) || obs != (ports.Observation{}) {
		t.Fatalf("Query = %+v, %v, want no observation and the scripted cause", obs, err)
	}
}

func TestPartnerWithoutQueryRefusesEveryQuery(t *testing.T) {
	p, _ := newPartner(serviceskit.WithoutQuery())
	p.ScriptQuery("k", serviceskit.Answer{Finding: ports.FindingApplied})

	obs, err := p.Query(context.Background(), "k")
	if !errors.Is(err, serviceskit.ErrNoQuery) || obs != (ports.Observation{}) {
		t.Fatalf("Query = %+v, %v, want ErrNoQuery", obs, err)
	}
	if got := p.Queried(); !slices.Equal(got, []ports.OutboundKey{"k"}) {
		t.Fatalf("Queried = %v, want the refused key recorded", got)
	}
}

func TestPartnerLosesTheResponseAfterApplyingTheEffect(t *testing.T) {
	p, _ := newPartner()
	key := ports.OutboundKey("k-lost")
	p.Script(key, serviceskit.LostAfterEffect())
	p.ScriptQuery(key, serviceskit.Answer{FromEffect: true})

	class, obs, _ := p.Submit(context.Background(), key, nil)
	if class != ports.ClassUnknownSent || obs != (ports.Observation{}) {
		t.Fatalf("Submit = %v, %+v, want unknown_sent with no observation", class, obs)
	}
	if !p.Applied(key) {
		t.Fatal("Applied = false; the lost response must still apply the effect")
	}
	got, err := p.Query(context.Background(), key)
	if err != nil || got.Finding != ports.FindingApplied {
		t.Fatalf("Query = %v, %v, want applied", got.Finding, err)
	}
}

func TestPartnerQueryFromEffectSeesAbsenceWhenNothingApplied(t *testing.T) {
	p, _ := newPartner()
	p.ScriptQuery("k", serviceskit.Answer{FromEffect: true})

	got, err := p.Query(context.Background(), "k")
	if err != nil || got.Finding != ports.FindingAbsent {
		t.Fatalf("Query = %v, %v, want absent", got.Finding, err)
	}
}

func TestPartnerDuplicateNotificationIsTheSameObservation(t *testing.T) {
	p, _ := newPartner()
	first, second := p.DuplicateNotification("k", ports.FindingApplied, "ref-9")

	if first != second {
		t.Fatalf("deliveries differ: %+v and %+v", first, second)
	}
	if first.ID == "" || first.Key != "k" || first.Finding != ports.FindingApplied || first.ExternalRef != "ref-9" {
		t.Fatalf("notification = %+v, want an identified applied observation for k", first)
	}
	next, _ := p.DuplicateNotification("k", ports.FindingApplied, "ref-9")
	if next.ID == first.ID {
		t.Fatalf("a second notification reused the ID %q", first.ID)
	}
}

func TestPartnerRecordsEveryKeyInOrder(t *testing.T) {
	p, _ := newPartner()
	key := ports.OutboundKey("k-resend")
	p.Script(key,
		serviceskit.Reply{Class: ports.ClassRetryableTransport},
		serviceskit.Reply{Class: ports.ClassAccepted},
	)
	p.ScriptQuery(key, serviceskit.Answer{Finding: ports.FindingPending})

	_, _, _ = p.Submit(context.Background(), key, nil)
	_, _ = p.Query(context.Background(), key)
	_, _, _ = p.Submit(context.Background(), key, nil)

	if got := p.Submitted(); !slices.Equal(got, []ports.OutboundKey{key, key}) {
		t.Fatalf("Submitted = %v, want the resend under the same key", got)
	}
	if got := p.Queried(); !slices.Equal(got, []ports.OutboundKey{key}) {
		t.Fatalf("Queried = %v, want one query", got)
	}
}

func TestPartnerReportsAnExhaustedScript(t *testing.T) {
	p, _ := newPartner()
	p.Script("k", serviceskit.Reply{Class: ports.ClassAccepted})
	_, _, _ = p.Submit(context.Background(), "k", nil)

	class, _, err := p.Submit(context.Background(), "k", nil)
	if class != 0 || !errors.Is(err, serviceskit.ErrScriptExhausted) {
		t.Fatalf("Submit = %v, %v, want no class and ErrScriptExhausted", class, err)
	}
	if _, err := p.Query(context.Background(), "unscripted"); !errors.Is(err, serviceskit.ErrScriptExhausted) {
		t.Fatalf("Query err = %v, want ErrScriptExhausted", err)
	}
	if got := p.Exhausted(); !slices.Equal(got, []ports.OutboundKey{"k", "unscripted"}) {
		t.Fatalf("Exhausted = %v, want both keys", got)
	}
}

func TestPartnerIsSafeForConcurrentAttempts(t *testing.T) {
	p, _ := newPartner()
	const n = 32
	replies := make([]serviceskit.Reply, n)
	for i := range replies {
		replies[i] = serviceskit.Reply{Class: ports.ClassAccepted, Finding: ports.FindingApplied, Applied: true}
	}
	p.Script("k", replies...)

	var wg sync.WaitGroup
	for range n {
		wg.Go(func() { _, _, _ = p.Submit(context.Background(), "k", nil) })
	}
	wg.Wait()

	if got := len(p.Submitted()); got != n {
		t.Fatalf("Submitted = %d, want %d", got, n)
	}
	if got := p.Exhausted(); len(got) != 0 {
		t.Fatalf("Exhausted = %v, want none", got)
	}
}

func TestPartnerRecordsConcurrentSubmitsInTheOrderItDeliversTheScript(t *testing.T) {
	p, _ := newPartner()
	const keys, perKey = 8, 8
	for k := range keys {
		key := ports.OutboundKey(fmt.Sprintf("k-%d", k))
		replies := make([]serviceskit.Reply, perKey)
		for j := range replies {
			replies[j] = serviceskit.Reply{Class: ports.ClassAccepted, Finding: ports.FindingApplied, ExternalRef: fmt.Sprintf("%s#%d", key, j)}
		}
		p.Script(key, replies...)
	}

	var mu sync.Mutex
	var delivered []ports.Observation
	var wg sync.WaitGroup
	for i := range keys * perKey {
		key := ports.OutboundKey(fmt.Sprintf("k-%d", i%keys))
		wg.Go(func() {
			_, obs, _ := p.Submit(context.Background(), key, nil)
			mu.Lock()
			delivered = append(delivered, obs)
			mu.Unlock()
		})
	}
	wg.Wait()

	slices.SortFunc(delivered, func(a, b ports.Observation) int { return strings.Compare(a.ID, b.ID) })
	order := make([]ports.OutboundKey, len(delivered))
	next := map[ports.OutboundKey]int{}
	for i, obs := range delivered {
		order[i] = obs.Key
		if want := fmt.Sprintf("%s#%d", obs.Key, next[obs.Key]); obs.ExternalRef != want {
			t.Fatalf("delivery %d of %s carried %q, want %q: the script was not consumed in order", i, obs.Key, obs.ExternalRef, want)
		}
		next[obs.Key]++
	}
	if got := p.Submitted(); !slices.Equal(got, order) {
		t.Fatalf("Submitted = %v, want the delivery order %v", got, order)
	}
}
