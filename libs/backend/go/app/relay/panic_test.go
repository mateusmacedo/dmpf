package relay

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/postgres"
)

const relayPanicValue = "kafka: nil producer, sasl password hunter2"

type panickingPublisher struct{}

func (panickingPublisher) Publish(context.Context, string, []byte) error { panic(relayPanicValue) }

type panickingClaims struct{ *fakeStore }

func (panickingClaims) Claim(context.Context, string, int, time.Duration) ([]postgres.Claimed, error) {
	panic(relayPanicValue)
}

func TestAPanickingDeliveryEndsRunWithTheErrorAndHandsTheClaimBack(t *testing.T) {
	store := newFakeStore(batchOf(t, 1))
	relay := loopRelay(store, panickingPublisher{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := relay.Run(ctx)

	if !errors.Is(err, ErrPanicked) || strings.Contains(err.Error(), relayPanicValue) {
		t.Fatalf("Run() = %v, want ErrPanicked without the panic value: cmd/main.go writes it to stderr and exits non-zero (RF-A1, ERR-20, ERR-23)", err)
	}
	if ctx.Err() != nil || store.claims.Load() != 1 {
		t.Fatalf("Run() returned after %d claims, ctx = %v, want the scan of the panic to be the last one", store.claims.Load(), ctx.Err())
	}
	transitions := store.recorded()
	if len(transitions) != 1 || transitions[0].kind != "rescheduled" || transitions[0].id != 1 || transitions[0].lastError != releaseReason {
		t.Fatalf("transitions = %+v, want the unfinished claim released (OBX-13)", transitions)
	}
}

func TestAPanickingClaimEndsRunWithTheError(t *testing.T) {
	relay := loopRelay(panickingClaims{newFakeStore()}, &fakePublisher{})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := relay.Run(ctx); !errors.Is(err, ErrPanicked) || strings.Contains(err.Error(), relayPanicValue) {
		t.Fatalf("Run() = %v, want ErrPanicked without the panic value (RF-A1, ERR-20, ERR-23)", err)
	}
}
