package redis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/redis"
)

var _ authn.RevocationStore = (*provider.RevocationStore)(nil)

func unreachableClient(t *testing.T) *redis.Client {
	t.Helper()
	client := redis.NewClient(&redis.Options{Addr: "127.0.0.1:1", DialTimeout: 200 * time.Millisecond, MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func TestRevokeRefusesAnEntryWithoutTheKeyItsKindRequires(t *testing.T) {
	store := provider.NewRevocationStore(unreachableClient(t), clock.System())
	now := time.Now()
	for _, r := range []authn.Revocation{
		{Kind: authn.RevocationSession, Subject: "user-1", OccurredAt: now, ExpiresAt: now.Add(time.Minute)},
		{Kind: authn.RevocationSubject, SessionID: "sid-1", OccurredAt: now, ExpiresAt: now.Add(time.Minute)},
		{Kind: "other", Subject: "user-1", SessionID: "sid-1", OccurredAt: now, ExpiresAt: now.Add(time.Minute)},
	} {
		if _, err := store.Revoke(context.Background(), r); !errors.Is(err, authn.ErrInvalidRevocation) {
			t.Fatalf("Revoke(%+v) = %v", r, err)
		}
	}
}

func TestRevokeOfAnExpiredEntryReachesNoServer(t *testing.T) {
	fake := clock.NewFake(time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC))
	store := provider.NewRevocationStore(unreachableClient(t), fake)
	changed, err := store.Revoke(context.Background(), authn.Revocation{
		Kind: authn.RevocationSession, SessionID: "sid-1",
		OccurredAt: fake.Now().Add(-time.Hour), ExpiresAt: fake.Now(),
	})
	if err != nil || changed {
		t.Fatalf("Revoke = %v, %v", changed, err)
	}
}

func TestUnreachableServerIsAnError(t *testing.T) {
	store := provider.NewRevocationStore(unreachableClient(t), clock.System())
	now := time.Now()
	if _, err := store.Revoked(context.Background(), authn.SessionRef{Subject: "user-1", ID: "sid-1", IssuedAt: now}); err == nil {
		t.Fatal("Revoked answered without a server")
	}
	if _, err := store.Revoke(context.Background(), authn.Revocation{
		Kind: authn.RevocationSubject, Subject: "user-1", OccurredAt: now, ExpiresAt: now.Add(time.Minute),
	}); err == nil {
		t.Fatal("Revoke answered without a server")
	}
}
