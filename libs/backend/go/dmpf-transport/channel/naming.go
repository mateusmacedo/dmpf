// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (KFK-01, KFK-01c, KFK-02) que o símbolo realiza, dentro do limite de 3 linhas.

package channel

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// MaxAddressLength is Kafka's own limit on a topic name, environment prefix
// included (KFK-01).
const MaxAddressLength = 249

var (
	// ErrInvalidAddress is a Kafka address outside the alphabet or the length
	// of KFK-01 and KFK-01c.
	ErrInvalidAddress = errors.New("channel: address is not a valid topic name (KFK-01)")

	// ErrEnvironmentInAddress is an address that names an environment in one
	// of its segments without declaring the prefix (KFK-02).
	ErrEnvironmentInAddress = errors.New("channel: environment appears in the address without a declared prefix (KFK-02)")

	// ErrAddressCollision is two addresses that differ only by dot and
	// underscore, which Kafka's telemetry treats as the same name (KFK-01c).
	ErrAddressCollision = errors.New("channel: addresses collide by dot and underscore (KFK-01c)")
)

var (
	addressAlphabet      = regexp.MustCompile(`^[a-z0-9._-]+$`)
	environmentSegments  = map[string]struct{}{"prod": {}, "staging": {}, "hml": {}}
	collisionReplacement = strings.NewReplacer("_", ".")
)

// ValidateAddress applies KFK-01, KFK-01c and KFK-02 to a Kafka address: the
// alphabet, the 249-character limit, and an environment segment only under a
// declared prefix the address starts with.
func ValidateAddress(address, environmentPrefix string) error {
	if address == "" {
		return fmt.Errorf("%w: empty", ErrInvalidAddress)
	}
	if len(address) > MaxAddressLength {
		return fmt.Errorf("%w: %d characters, limit is %d", ErrInvalidAddress, len(address), MaxAddressLength)
	}
	if !addressAlphabet.MatchString(address) {
		return fmt.Errorf("%w: %q has a character outside [a-z0-9._-] (KFK-01c)", ErrInvalidAddress, address)
	}

	if environmentPrefix != "" {
		if !addressAlphabet.MatchString(environmentPrefix) {
			return fmt.Errorf("%w: environment prefix %q has a character outside [a-z0-9._-] (KFK-01c)", ErrInvalidAddress, environmentPrefix)
		}
		if !strings.HasPrefix(address, environmentPrefix+".") {
			return fmt.Errorf("%w: %q does not start with the declared prefix %q", ErrEnvironmentInAddress, address, environmentPrefix)
		}
		return nil
	}

	for _, segment := range strings.Split(address, ".") {
		if _, isEnvironment := environmentSegments[segment]; isEnvironment {
			return fmt.Errorf("%w: %q carries segment %q", ErrEnvironmentInAddress, address, segment)
		}
	}
	return nil
}

func collisionKey(address string) string { return collisionReplacement.Replace(address) }
