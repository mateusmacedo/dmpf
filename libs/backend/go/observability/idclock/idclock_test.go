package idclock_test

import (
	"encoding/hex"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/idclock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func TestSystemClockAnswersInNanosecondsOfTheWallClock(t *testing.T) {
	before := ports.Instant(time.Now().UnixNano())

	now := idclock.SystemClock{}.Now()

	after := ports.Instant(time.Now().UnixNano())
	if now < before || now > after {
		t.Fatalf("Now() = %d, want an instant within [%d, %d]", now, before, after)
	}
}

func TestMintedIdentitiesAreSixteenRandomBytesInHex(t *testing.T) {
	claim := idclock.NewClaimIDs("test").NewClaimID()
	message := string(idclock.NewMessageIDs("test").NewMessageID())

	for name, identity := range map[string]string{"claim": claim, "message": message} {
		decoded, err := hex.DecodeString(identity)
		if err != nil {
			t.Fatalf("%s identity %q is not hex: %v", name, identity, err)
		}
		if len(decoded) != 16 {
			t.Fatalf("%s identity carries %d bytes, want 16", name, len(decoded))
		}
	}
}

func TestEveryAcquisitionMintsItsOwnIdentity(t *testing.T) {
	claims := idclock.NewClaimIDs("test")
	messages := idclock.NewMessageIDs("test")

	if first, second := claims.NewClaimID(), claims.NewClaimID(); first == second {
		t.Fatal("two acquisitions got the same claim identity; the claim of OBX-08 is per acquisition")
	}
	if first, second := messages.NewMessageID(), messages.NewMessageID(); first == second {
		t.Fatal("two messages got the same message_id; FND-04 §4.1 mints one per message")
	}
}

func TestTheRealizationsSatisfyTheDomainPorts(t *testing.T) {
	var clock ports.Clock = idclock.SystemClock{}
	var ids ports.IDGenerator = idclock.NewMessageIDs("test")

	if clock.Now() == 0 {
		t.Fatal("the wall clock answered zero")
	}
	if ids.NewMessageID() == "" {
		t.Fatal("the generator answered an empty message_id")
	}
}
