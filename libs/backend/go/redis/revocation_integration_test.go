//go:build integration

package redis_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	provider "github.com/mateusmacedo/dmpf/libs/backend/go/redis"
)

func integrationClient(t *testing.T) *redis.Client {
	t.Helper()
	if os.Getenv("REDIS_URL") == "" {
		t.Skip("REDIS_URL is not set")
	}
	cfg, err := provider.FromEnv(os.Getenv)
	if err != nil {
		t.Fatalf("FromEnv: %v", err)
	}
	client, err := provider.NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	t.Cleanup(func() { _ = client.Close() })
	return client
}

func integrationStore(t *testing.T) (*provider.RevocationStore, *redis.Client) {
	t.Helper()
	client := integrationClient(t)
	return provider.NewRevocationStore(client, clock.System()), client
}

func unique(prefix string) string { return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano()) }

func revoke(t *testing.T, store *provider.RevocationStore, r authn.Revocation) bool {
	t.Helper()
	changed, err := store.Revoke(context.Background(), r)
	if err != nil {
		t.Fatalf("Revoke: %v", err)
	}
	return changed
}

func revoked(t *testing.T, store *provider.RevocationStore, ref authn.SessionRef) authn.RevocationKind {
	t.Helper()
	kind, err := store.Revoked(context.Background(), ref)
	if err != nil {
		t.Fatalf("Revoked: %v", err)
	}
	return kind
}

func sessionEntry(sid string, at time.Time) authn.Revocation {
	return authn.Revocation{EventID: unique("ev"), Kind: authn.RevocationSession, SessionID: sid, OccurredAt: at, ExpiresAt: time.Now().Add(time.Minute)}
}

func subjectEntry(sub ports.SubjectID, at time.Time) authn.Revocation {
	return authn.Revocation{EventID: unique("ev"), Kind: authn.RevocationSubject, Subject: sub, OccurredAt: at, ExpiresAt: time.Now().Add(time.Minute)}
}

func TestIntegrationSessionEntryDeniesOnlyThatSession(t *testing.T) {
	store, _ := integrationStore(t)
	sub, sid := ports.SubjectID(unique("user")), unique("sid")
	now := time.Now()

	if !revoke(t, store, sessionEntry(sid, now)) {
		t.Fatal("first Revoke reported no change")
	}
	if got := revoked(t, store, authn.SessionRef{Subject: sub, ID: sid, IssuedAt: now}); got != authn.RevocationSession {
		t.Fatalf("revoked session = %q", got)
	}
	if got := revoked(t, store, authn.SessionRef{Subject: sub, ID: unique("other"), IssuedAt: now}); got != "" {
		t.Fatalf("another session = %q", got)
	}
	if got := revoked(t, store, authn.SessionRef{Subject: sub, IssuedAt: now}); got != "" {
		t.Fatalf("token without sid = %q", got)
	}
}

func TestIntegrationSubjectEntryDeniesTokensIssuedUpToTheRevocation(t *testing.T) {
	store, _ := integrationStore(t)
	sub := ports.SubjectID(unique("user"))
	at := time.Now().Truncate(time.Second)
	revoke(t, store, subjectEntry(sub, at))

	if got := revoked(t, store, authn.SessionRef{Subject: sub, ID: unique("sid"), IssuedAt: at}); got != authn.RevocationSubject {
		t.Fatalf("iat equal to the revocation = %q", got)
	}
	if got := revoked(t, store, authn.SessionRef{Subject: sub, IssuedAt: at.Add(-time.Second)}); got != authn.RevocationSubject {
		t.Fatalf("iat before the revocation = %q", got)
	}
	if got := revoked(t, store, authn.SessionRef{Subject: sub, IssuedAt: at.Add(time.Second)}); got != "" {
		t.Fatalf("iat after the revocation = %q", got)
	}
}

func TestIntegrationSubjectEntryDeniesATokenIssuedInTheSameSecondAsAFractionalRevocation(t *testing.T) {
	store, _ := integrationStore(t)
	sub := ports.SubjectID(unique("user"))
	at := time.Now().Truncate(time.Second)
	revoke(t, store, subjectEntry(sub, at.Add(300*time.Millisecond)))

	if got := revoked(t, store, authn.SessionRef{Subject: sub, IssuedAt: at}); got != authn.RevocationSubject {
		t.Fatalf("iat in the second of the revocation = %q", got)
	}
	if got := revoked(t, store, authn.SessionRef{Subject: sub, IssuedAt: at.Add(time.Second)}); got != "" {
		t.Fatalf("iat in the next second = %q", got)
	}
}

func TestIntegrationSessionTakesPrecedenceOverSubject(t *testing.T) {
	store, _ := integrationStore(t)
	sub, sid := ports.SubjectID(unique("user")), unique("sid")
	at := time.Now().Truncate(time.Second)
	revoke(t, store, subjectEntry(sub, at))
	revoke(t, store, sessionEntry(sid, at))

	if got := revoked(t, store, authn.SessionRef{Subject: sub, ID: sid, IssuedAt: at}); got != authn.RevocationSession {
		t.Fatalf("both entries = %q", got)
	}
}

func TestIntegrationRedeliveryAndReorderingNeverShortenTheRevocation(t *testing.T) {
	store, _ := integrationStore(t)
	sub := ports.SubjectID(unique("user"))
	early := time.Now().Truncate(time.Second).Add(-time.Minute)
	late := early.Add(30 * time.Second)

	if !revoke(t, store, subjectEntry(sub, late)) {
		t.Fatal("first Revoke reported no change")
	}
	if revoke(t, store, subjectEntry(sub, late)) {
		t.Fatal("redelivery reported a change")
	}
	if revoke(t, store, subjectEntry(sub, early)) {
		t.Fatal("an older revocation reported a change")
	}
	if got := revoked(t, store, authn.SessionRef{Subject: sub, IssuedAt: early.Add(10 * time.Second)}); got != authn.RevocationSubject {
		t.Fatalf("token issued between the two revocations = %q", got)
	}
	if !revoke(t, store, subjectEntry(sub, late.Add(time.Second))) {
		t.Fatal("a newer revocation reported no change")
	}
}

func TestIntegrationEntryExpiresAtItsInstant(t *testing.T) {
	store, _ := integrationStore(t)
	sub, sid := ports.SubjectID(unique("user")), unique("sid")
	now := time.Now()
	entry := sessionEntry(sid, now)
	entry.ExpiresAt = now.Add(time.Second)
	revoke(t, store, entry)

	if got := revoked(t, store, authn.SessionRef{Subject: sub, ID: sid, IssuedAt: now}); got != authn.RevocationSession {
		t.Fatalf("before the expiry = %q", got)
	}
	time.Sleep(time.Until(entry.ExpiresAt) + 200*time.Millisecond)
	if got := revoked(t, store, authn.SessionRef{Subject: sub, ID: sid, IssuedAt: now}); got != "" {
		t.Fatalf("after the expiry = %q", got)
	}
}

func TestIntegrationANewerRevocationKeepsTheLaterExpiry(t *testing.T) {
	store, _ := integrationStore(t)
	sub := ports.SubjectID(unique("user"))
	at := time.Now().Truncate(time.Second)
	first := subjectEntry(sub, at)
	first.ExpiresAt = time.Now().Add(3 * time.Second)
	second := subjectEntry(sub, at.Add(time.Second))
	second.ExpiresAt = time.Now().Add(time.Second)
	revoke(t, store, first)
	if !revoke(t, store, second) {
		t.Fatal("the newer revocation reported no change")
	}

	time.Sleep(time.Until(second.ExpiresAt) + 200*time.Millisecond)
	if got := revoked(t, store, authn.SessionRef{Subject: sub, IssuedAt: at.Add(time.Second)}); got != authn.RevocationSubject {
		t.Fatalf("after the shorter expiry = %q", got)
	}
}

func TestIntegrationKeysAreHashedUnderThePrefix(t *testing.T) {
	store, client := integrationStore(t)
	sub, sid := ports.SubjectID(unique("user")), unique("sid")
	at := time.Now()
	revoke(t, store, sessionEntry(sid, at))
	revoke(t, store, subjectEntry(sub, at))

	for _, key := range []string{"dmpf:revocation:sid:" + digest(sid), "dmpf:revocation:sub:" + digest(string(sub))} {
		value, err := client.Get(context.Background(), key).Result()
		if err != nil {
			t.Fatalf("GET %s: %v", key, err)
		}
		if strings.Contains(value, sid) || strings.Contains(value, string(sub)) {
			t.Fatalf("value %q carries the identifier", value)
		}
	}
}

func TestIntegrationUnreadableEntryIsAnError(t *testing.T) {
	store, client := integrationStore(t)
	sid := unique("sid")
	if err := client.Set(context.Background(), "dmpf:revocation:sid:"+digest(sid), "garbage", time.Minute).Err(); err != nil {
		t.Fatalf("SET: %v", err)
	}
	if _, err := store.Revoked(context.Background(), authn.SessionRef{Subject: "user-1", ID: sid, IssuedAt: time.Now()}); err == nil {
		t.Fatal("an unreadable entry was read as an answer")
	}
}

func TestIntegrationRevokeOverAnUnreadableEntryIsAnErrorAndKeepsIt(t *testing.T) {
	store, client := integrationStore(t)
	ctx := context.Background()
	for _, value := range []string{"garbage", "abc:123", "123:abc"} {
		sid := unique("sid")
		key := "dmpf:revocation:sid:" + digest(sid)
		if err := client.Set(ctx, key, value, time.Minute).Err(); err != nil {
			t.Fatalf("SET: %v", err)
		}
		changed, err := store.Revoke(ctx, sessionEntry(sid, time.Now()))
		if !errors.Is(err, provider.ErrUnreadableEntry) || changed {
			t.Fatalf("Revoke over %q = %v, %v", value, changed, err)
		}
		if got, err := client.Get(ctx, key).Result(); err != nil || got != value {
			t.Fatalf("entry %q became %q (%v)", value, got, err)
		}
	}
}

func TestIntegrationACLRefusesWhatTheDenyListDoesNotNeed(t *testing.T) {
	client := integrationClient(t)
	ctx := context.Background()
	cases := map[string]error{
		"key outside the prefix": client.Set(ctx, unique("dmpf:other"), "x", time.Minute).Err(),
		"CONFIG":                 client.ConfigGet(ctx, "maxmemory-policy").Err(),
		"FLUSHALL":               client.FlushAll(ctx).Err(),
	}
	for name, err := range cases {
		if err == nil || !strings.HasPrefix(err.Error(), "NOPERM") {
			t.Errorf("%s: err = %v, want NOPERM", name, err)
		}
	}
}

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
