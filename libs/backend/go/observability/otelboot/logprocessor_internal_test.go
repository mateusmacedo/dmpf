package otelboot

import (
	"strconv"
	"testing"

	"go.opentelemetry.io/otel/attribute"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/redact"
)

func TestTheSecretDecisionsStopGrowingAtTheBoundAndKeysBeyondItAreStillDecided(t *testing.T) {
	secrets := new(secretKeys)
	distinct := maxDecidedKeys + 512
	keyAt := func(i int) (attribute.Key, string) {
		if i%2 == 0 {
			return attribute.Key("dmpf.field." + strconv.Itoa(i)), "value"
		}
		return attribute.Key("dmpf.token." + strconv.Itoa(i)), redact.Placeholder
	}

	for pass := range 2 {
		for i := range distinct {
			key, want := keyAt(i)
			if got := secrets.withoutSecrets(attribute.String(string(key), "value")).Value.AsString(); got != want {
				t.Fatalf("pass %d: %s = %q, want %q: a key past the bound of %d is still decided", pass+1, key, got, want, maxDecidedKeys)
			}
		}
	}

	decided := 0
	secrets.decided.Range(func(any, any) bool {
		decided++
		return true
	})
	if decided != maxDecidedKeys {
		t.Fatalf("the cache holds %d decisions after %d distinct keys, want it full at its bound of %d and no larger", decided, distinct, maxDecidedKeys)
	}
}
