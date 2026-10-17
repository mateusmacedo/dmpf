package authn

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var ErrInvalidRevocation = errors.New("auth: revocation lacks the key its kind requires")

// MemoryStore is a RevocationStore for tests and single-process use: replicas
// do not share it, and a restart forgets every entry.
type MemoryStore struct {
	clock    clock.Clock
	mu       sync.Mutex
	sessions map[string]memoryEntry
	subjects map[ports.SubjectID]memoryEntry
}

type memoryEntry struct {
	revokedAt time.Time
	expiresAt time.Time
}

func NewMemoryStore(c clock.Clock) *MemoryStore {
	if c == nil {
		c = clock.System()
	}
	return &MemoryStore{
		clock:    c,
		sessions: make(map[string]memoryEntry),
		subjects: make(map[ports.SubjectID]memoryEntry),
	}
}

func (s *MemoryStore) Revoked(_ context.Context, ref SessionRef) (RevocationKind, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()

	if ref.ID != "" {
		if entry, ok := s.sessions[ref.ID]; ok && entry.live(now) {
			return RevocationSession, nil
		}
	}
	if entry, ok := s.subjects[ref.Subject]; ok && entry.live(now) && !ref.IssuedAt.After(entry.revokedAt) {
		return RevocationSubject, nil
	}
	return "", nil
}

func (s *MemoryStore) Revoke(_ context.Context, r Revocation) (bool, error) {
	if err := r.keyed(); err != nil {
		return false, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.clock.Now()
	s.forgetExpired(now)

	if !r.ExpiresAt.After(now) {
		return false, nil
	}
	if r.Kind == RevocationSession {
		return keepGreatest(s.sessions, r.SessionID, r), nil
	}
	return keepGreatest(s.subjects, r.Subject, r), nil
}

func (s *MemoryStore) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.sessions) + len(s.subjects)
}

func (s *MemoryStore) forgetExpired(now time.Time) {
	for key, entry := range s.sessions {
		if !entry.live(now) {
			delete(s.sessions, key)
		}
	}
	for key, entry := range s.subjects {
		if !entry.live(now) {
			delete(s.subjects, key)
		}
	}
}

func keepGreatest[K comparable](entries map[K]memoryEntry, key K, r Revocation) bool {
	current, ok := entries[key]
	if ok && !r.OccurredAt.After(current.revokedAt) {
		return false
	}
	entries[key] = memoryEntry{revokedAt: r.OccurredAt, expiresAt: later(current.expiresAt, r.ExpiresAt)}
	return true
}

func later(a, b time.Time) time.Time {
	if a.After(b) {
		return a
	}
	return b
}

func (e memoryEntry) live(now time.Time) bool { return now.Before(e.expiresAt) }

func (r Revocation) keyed() error {
	switch {
	case r.Kind == RevocationSession && r.SessionID != "":
		return nil
	case r.Kind == RevocationSubject && r.Subject != "":
		return nil
	}
	return fmt.Errorf("%w: kind %q", ErrInvalidRevocation, r.Kind)
}
