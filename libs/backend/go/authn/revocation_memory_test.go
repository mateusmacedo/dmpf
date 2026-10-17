package authn_test

import (
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
)

var memoryEpoch = time.Date(2026, 10, 16, 12, 0, 0, 0, time.UTC)

func sessionRevocation(sid string, occurred time.Time) authn.Revocation {
	return authn.Revocation{
		EventID:    "evt-" + sid,
		Kind:       authn.RevocationSession,
		Subject:    "f:0e1a:alice",
		SessionID:  sid,
		OccurredAt: occurred,
		ExpiresAt:  occurred.Add(6 * time.Minute),
	}
}

func subjectRevocation(occurred time.Time) authn.Revocation {
	return authn.Revocation{
		EventID:    "evt-subject",
		Kind:       authn.RevocationSubject,
		Subject:    "f:0e1a:alice",
		OccurredAt: occurred,
		ExpiresAt:  occurred.Add(6 * time.Minute),
	}
}

func TestMemoryStoreAnswersTheSemanticsOfTheEntries(t *testing.T) {
	revokedAt := memoryEpoch.Add(-time.Minute)

	tests := []struct {
		name    string
		entries []authn.Revocation
		ref     authn.SessionRef
		want    authn.RevocationKind
	}{
		{"nothing recorded", nil,
			authn.SessionRef{Subject: "f:0e1a:alice", ID: "s-1", IssuedAt: revokedAt}, ""},
		{"the session entry denies its sid", []authn.Revocation{sessionRevocation("s-1", revokedAt)},
			authn.SessionRef{Subject: "f:0e1a:alice", ID: "s-1", IssuedAt: memoryEpoch}, authn.RevocationSession},
		{"the session entry spares another sid", []authn.Revocation{sessionRevocation("s-1", revokedAt)},
			authn.SessionRef{Subject: "f:0e1a:alice", ID: "s-2", IssuedAt: revokedAt}, ""},
		{"the session entry spares a token without sid", []authn.Revocation{sessionRevocation("s-1", revokedAt)},
			authn.SessionRef{Subject: "f:0e1a:alice", IssuedAt: revokedAt}, ""},
		{"the subject entry denies iat before revoked_at", []authn.Revocation{subjectRevocation(revokedAt)},
			authn.SessionRef{Subject: "f:0e1a:alice", ID: "s-9", IssuedAt: revokedAt.Add(-time.Second)}, authn.RevocationSubject},
		{"the subject entry denies iat equal to revoked_at", []authn.Revocation{subjectRevocation(revokedAt)},
			authn.SessionRef{Subject: "f:0e1a:alice", IssuedAt: revokedAt}, authn.RevocationSubject},
		{"the subject entry spares a later login", []authn.Revocation{subjectRevocation(revokedAt)},
			authn.SessionRef{Subject: "f:0e1a:alice", IssuedAt: revokedAt.Add(time.Second)}, ""},
		{"the subject entry spares another subject", []authn.Revocation{subjectRevocation(revokedAt)},
			authn.SessionRef{Subject: "f:0e1a:bob", IssuedAt: revokedAt}, ""},
		{"session precedes subject when both deny",
			[]authn.Revocation{subjectRevocation(revokedAt), sessionRevocation("s-1", revokedAt)},
			authn.SessionRef{Subject: "f:0e1a:alice", ID: "s-1", IssuedAt: revokedAt}, authn.RevocationSession},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := authn.NewMemoryStore(clock.NewFake(memoryEpoch))
			for _, entry := range tt.entries {
				if _, err := store.Revoke(t.Context(), entry); err != nil {
					t.Fatalf("Revoke(%+v) err = %v", entry, err)
				}
			}

			got, err := store.Revoked(t.Context(), tt.ref)
			if err != nil {
				t.Fatalf("Revoked() err = %v", err)
			}
			if got != tt.want {
				t.Fatalf("Revoked() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMemoryStoreKeepsTheGreatestRevokedAt(t *testing.T) {
	older, newer := memoryEpoch.Add(-2*time.Minute), memoryEpoch.Add(-time.Minute)

	tests := []struct {
		name        string
		first       authn.Revocation
		second      authn.Revocation
		wantChanged bool
	}{
		{"redelivery changes nothing", subjectRevocation(newer), subjectRevocation(newer), false},
		{"an older event arriving late changes nothing", subjectRevocation(newer), subjectRevocation(older), false},
		{"a newer event moves revoked_at forward", subjectRevocation(older), subjectRevocation(newer), true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := authn.NewMemoryStore(clock.NewFake(memoryEpoch))
			if changed, err := store.Revoke(t.Context(), tt.first); err != nil || !changed {
				t.Fatalf("first Revoke() = %v, %v; want a new entry", changed, err)
			}

			changed, err := store.Revoke(t.Context(), tt.second)
			if err != nil {
				t.Fatalf("second Revoke() err = %v", err)
			}
			if changed != tt.wantChanged {
				t.Fatalf("second Revoke() changed = %v, want %v", changed, tt.wantChanged)
			}
			between := authn.SessionRef{Subject: "f:0e1a:alice", IssuedAt: older.Add(30 * time.Second)}
			if got, _ := store.Revoked(t.Context(), between); got != authn.RevocationSubject {
				t.Fatalf("Revoked(iat between the two) = %q, want subject: revoked_at went backwards", got)
			}
		})
	}
}

func TestMemoryStoreForgetsWhatExpired(t *testing.T) {
	fake := clock.NewFake(memoryEpoch)
	store := authn.NewMemoryStore(fake)
	ref := authn.SessionRef{Subject: "f:0e1a:alice", ID: "s-1", IssuedAt: memoryEpoch.Add(-time.Hour)}
	for _, entry := range []authn.Revocation{sessionRevocation("s-1", memoryEpoch), subjectRevocation(memoryEpoch)} {
		if _, err := store.Revoke(t.Context(), entry); err != nil {
			t.Fatal(err)
		}
	}
	if store.Len() != 2 {
		t.Fatalf("Len() = %d, want the two entries", store.Len())
	}

	fake.Advance(6 * time.Minute)

	if got, _ := store.Revoked(t.Context(), ref); got != "" {
		t.Fatalf("Revoked() after expiry = %q, want not revoked", got)
	}
	if _, err := store.Revoke(t.Context(), sessionRevocation("s-2", fake.Now())); err != nil {
		t.Fatal(err)
	}
	if store.Len() != 1 {
		t.Fatalf("Len() = %d, want only the fresh entry: the expired ones must leave the map", store.Len())
	}
}

func TestMemoryStoreIgnoresAnAlreadyExpiredRevocation(t *testing.T) {
	store := authn.NewMemoryStore(clock.NewFake(memoryEpoch))

	changed, err := store.Revoke(t.Context(), sessionRevocation("s-1", memoryEpoch.Add(-time.Hour)))

	if err != nil || changed {
		t.Fatalf("Revoke(expired) = %v, %v; want no change", changed, err)
	}
	if store.Len() != 0 {
		t.Fatalf("Len() = %d, want nothing stored", store.Len())
	}
}

func TestMemoryStoreRefusesARevocationWithoutItsKey(t *testing.T) {
	tests := []struct {
		name string
		r    authn.Revocation
	}{
		{"session without sid", authn.Revocation{Kind: authn.RevocationSession, Subject: "f:0e1a:alice",
			OccurredAt: memoryEpoch, ExpiresAt: memoryEpoch.Add(time.Minute)}},
		{"subject without sub", authn.Revocation{Kind: authn.RevocationSubject,
			OccurredAt: memoryEpoch, ExpiresAt: memoryEpoch.Add(time.Minute)}},
		{"unknown kind", authn.Revocation{Kind: "jti", Subject: "f:0e1a:alice",
			OccurredAt: memoryEpoch, ExpiresAt: memoryEpoch.Add(time.Minute)}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			store := authn.NewMemoryStore(clock.NewFake(memoryEpoch))

			if _, err := store.Revoke(t.Context(), tt.r); err == nil {
				t.Fatal("Revoke() err = nil, want the entry refused")
			}
			if store.Len() != 0 {
				t.Fatalf("Len() = %d, want nothing stored", store.Len())
			}
		})
	}
}
