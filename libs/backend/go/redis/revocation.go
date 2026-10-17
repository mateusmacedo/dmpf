package redis

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/redis/go-redis/v9"

	"github.com/mateusmacedo/dmpf/libs/backend/go/authn"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

var ErrUnreadableEntry = errors.New("redis: revocation entry is unreadable")

const unreadableReply = "DMPF_UNREADABLE revocation entry"

const (
	sessionPrefix = "dmpf:revocation:sid:"
	subjectPrefix = "dmpf:revocation:sub:"
)

// The value is "<revoked_at_ms>:<expires_at_ms>": the expiry travels in the
// value because PEXPIRETIME is outside the ACL, and the script keeps the later.
var revokeScript = redis.NewScript(`
local current = redis.call('GET', KEYS[1])
local expires = ARGV[2]
if current then
  local sep = string.find(current, ':', 1, true)
  local currentRevoked = sep and tonumber(string.sub(current, 1, sep - 1))
  local currentExpires = sep and tonumber(string.sub(current, sep + 1))
  if not (currentRevoked and currentExpires) then
    return redis.error_reply(ARGV[3])
  end
  if tonumber(ARGV[1]) <= currentRevoked then
    return 0
  end
  if currentExpires > tonumber(expires) then
    expires = string.sub(current, sep + 1)
  end
end
redis.call('SET', KEYS[1], ARGV[1] .. ':' .. expires, 'PXAT', expires)
return 1
`)

type Client interface {
	redis.Scripter
	MGet(ctx context.Context, keys ...string) *redis.SliceCmd
}

type RevocationStore struct {
	client Client
	clock  clock.Clock
}

func NewRevocationStore(client Client, c clock.Clock) *RevocationStore {
	if c == nil {
		c = clock.System()
	}
	return &RevocationStore{client: client, clock: c}
}

func (s *RevocationStore) Revoked(ctx context.Context, ref authn.SessionRef) (authn.RevocationKind, error) {
	keys := []string{subjectKey(ref.Subject)}
	if ref.ID != "" {
		keys = append(keys, sessionKey(ref.ID))
	}
	values, err := s.client.MGet(ctx, keys...).Result()
	if err != nil {
		return "", fmt.Errorf("redis: revocation lookup: %w", err)
	}
	if len(values) != len(keys) {
		return "", ErrUnreadableEntry
	}

	if ref.ID != "" && values[1] != nil {
		if _, err := revokedAt(values[1]); err != nil {
			return "", err
		}
		return authn.RevocationSession, nil
	}
	if values[0] == nil {
		return "", nil
	}
	at, err := revokedAt(values[0])
	if err != nil {
		return "", err
	}
	if ref.IssuedAt.UnixMilli() <= at {
		return authn.RevocationSubject, nil
	}
	return "", nil
}

func (s *RevocationStore) Revoke(ctx context.Context, r authn.Revocation) (bool, error) {
	key, err := keyOf(r)
	if err != nil {
		return false, err
	}
	if !r.ExpiresAt.After(s.clock.Now()) {
		return false, nil
	}
	changed, err := revokeScript.Run(ctx, s.client, []string{key}, r.OccurredAt.UnixMilli(), r.ExpiresAt.UnixMilli(), unreadableReply).Int()
	var reply redis.Error
	if errors.As(err, &reply) && reply.Error() == unreadableReply {
		return false, ErrUnreadableEntry
	}
	if err != nil {
		return false, fmt.Errorf("redis: revocation write: %w", err)
	}
	return changed == 1, nil
}

func keyOf(r authn.Revocation) (string, error) {
	switch {
	case r.Kind == authn.RevocationSession && r.SessionID != "":
		return sessionKey(r.SessionID), nil
	case r.Kind == authn.RevocationSubject && r.Subject != "":
		return subjectKey(r.Subject), nil
	}
	return "", fmt.Errorf("%w: kind %q", authn.ErrInvalidRevocation, r.Kind)
}

func sessionKey(id string) string { return sessionPrefix + digest(id) }

func subjectKey(subject ports.SubjectID) string { return subjectPrefix + digest(string(subject)) }

func digest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func revokedAt(value any) (int64, error) {
	text, ok := value.(string)
	if !ok {
		return 0, ErrUnreadableEntry
	}
	head, _, found := strings.Cut(text, ":")
	if !found {
		return 0, ErrUnreadableEntry
	}
	at, err := strconv.ParseInt(head, 10, 64)
	if err != nil {
		return 0, ErrUnreadableEntry
	}
	return at, nil
}
