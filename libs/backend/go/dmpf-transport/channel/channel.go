// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (ASY-02, TRP-31/32, KFK-*, SQS-04) que o símbolo realiza, dentro do limite de 3 linhas.

// Package channel is the in-process form of the catalogue entry a provider
// must resolve before operating an asynchronous channel (FND-06 §16, ASY-01):
// the seven items of ASY-02 and the redelivery-window formulas of TRP-24.
package channel

import (
	"errors"
	"fmt"
)

// Transport names the asynchronous transport a channel is bound to.
type Transport string

const (
	Kafka  Transport = "kafka"
	SQS    Transport = "sqs"
	SNSSQS Transport = "sns-sqs"
)

// Unit is the scope within which a channel promises order (ASY-02): a Kafka
// partition, an SQS message group, or none at all.
type Unit string

const (
	Partition Unit = "partition"
	Group     Unit = "group"
	None      Unit = "none"
)

// Strategy is the retry strategy of a channel and, with it, the effect on order
// the strategy declares (TRP-31).
type Strategy string

const (
	InlineWithLimit Strategy = "inline-with-limit"
	SeparateChannel Strategy = "separate-channel"
)

var (
	// ErrMissingItem is a channel that lacks one of the items of ASY-02; the
	// message names the item.
	ErrMissingItem = errors.New("channel: catalogue item absent (ASY-02)")

	// ErrUnknownTransport is a channel bound to a transport this package does
	// not know.
	ErrUnknownTransport = errors.New("channel: unknown transport")

	// ErrInvalidOrdering is an ordering whose unit is unknown, or whose key
	// contradicts the unit: none carries no key, and a unit carries one
	// (KFK-06, SQS-04).
	ErrInvalidOrdering = errors.New("channel: ordering key and unit disagree")

	// ErrOrderedChannelWithSeparateRetry is a channel that promises order and
	// declares retry in a separate channel at once (TRP-32, KFK-11).
	ErrOrderedChannelWithSeparateRetry = errors.New("channel: ordered channel must not retry in a separate channel (TRP-32)")

	// ErrUnitTransportMismatch is a unit the transport cannot honour: Kafka
	// orders by partition and SQS by group (KFK-06, SQS-04).
	ErrUnitTransportMismatch = errors.New("channel: ordering unit is not one the transport provides")

	// ErrWindowTransportMismatch is a redelivery window computed by the formula
	// of another transport (TRP-24).
	ErrWindowTransportMismatch = errors.New("channel: redelivery window formula belongs to another transport")

	// ErrUnknownChannel is what Resolve reports for a destination the catalogue
	// does not carry: the provider does not operate it (ASY-01).
	ErrUnknownChannel = errors.New("channel: destination is not catalogued (ASY-01)")

	// ErrDuplicateChannel is a catalogue whose key and channel name disagree.
	ErrDuplicateChannel = errors.New("channel: catalogue key does not match the channel name")
)

// Ordering is the promise of order a channel makes: the key it orders by and
// the unit the promise holds within. None with an empty key declares no order.
type Ordering struct {
	Key  string
	Unit Unit
}

// Validate refuses an unknown unit, a key without a unit and a unit without a
// key (KFK-06, SQS-04).
func (o Ordering) Validate() error {
	switch o.Unit {
	case None:
		if o.Key != "" {
			return fmt.Errorf("%w: unit none with key %q", ErrInvalidOrdering, o.Key)
		}
	case Partition, Group:
		if o.Key == "" {
			return fmt.Errorf("%w: unit %s without a key", ErrInvalidOrdering, o.Unit)
		}
	default:
		return fmt.Errorf("%w: unit %q is unknown", ErrInvalidOrdering, o.Unit)
	}
	return nil
}

// Ordered reports whether the channel promises any order.
func (o Ordering) Ordered() bool { return o.Unit != None }

// Retry is the retry strategy of a channel and its attempt limit; the limit is
// mandatory because a poison message must not hold a partition or a group
// (TRP-31).
type Retry struct {
	Strategy    Strategy
	MaxAttempts int
}

// Validate refuses an unknown strategy and a limit that is not positive.
func (r Retry) Validate() error {
	switch r.Strategy {
	case InlineWithLimit, SeparateChannel:
	default:
		return fmt.Errorf("%w: retry strategy %q is unknown", ErrMissingItem, r.Strategy)
	}
	if r.MaxAttempts <= 0 {
		return fmt.Errorf("%w: retry.maxAttempts must be positive (TRP-31)", ErrMissingItem)
	}
	return nil
}

// Channel is one catalogued channel: the seven items of ASY-02 plus the Kafka
// topology KFK-04 and KFK-07 demand. Address is the only concrete name and it
// lives here, in configuration, never in code (TRP-07).
type Channel struct {
	Name          string
	Transport     Transport
	Address       string
	EventType     string
	ContractMajor int
	ContractRef   string
	Ordering      Ordering
	Redelivery    RedeliveryWindow
	Containment   string
	Retry         Retry

	// Kafka topology (KFK-04, KFK-07); ignored by the other transports.
	Group       string
	Partitions  int
	Partitioner string
	KeyEncoding string

	// EnvironmentPrefix is the environment segment the address carries when
	// clusters are not isolated (KFK-02); empty means isolation exists.
	EnvironmentPrefix string
}

// Validate refuses a channel the provider could not operate: any item of
// ASY-02 absent, an ordering the transport cannot honour, an ordered channel
// with separate-channel retry, or a Kafka address outside KFK-01/01c/02.
func (c Channel) Validate() error {
	switch {
	case c.Name == "":
		return fmt.Errorf("%w: name", ErrMissingItem)
	case c.Address == "":
		return fmt.Errorf("%w: %s: address", ErrMissingItem, c.Name)
	case c.EventType == "":
		return fmt.Errorf("%w: %s: eventType", ErrMissingItem, c.Name)
	case c.ContractMajor <= 0:
		return fmt.Errorf("%w: %s: contractMajor", ErrMissingItem, c.Name)
	case c.ContractRef == "":
		return fmt.Errorf("%w: %s: contractRef (ASY-03)", ErrMissingItem, c.Name)
	case c.Containment == "":
		return fmt.Errorf("%w: %s: containment", ErrMissingItem, c.Name)
	}

	switch c.Transport {
	case Kafka, SQS, SNSSQS:
	case "":
		return fmt.Errorf("%w: %s: transport", ErrMissingItem, c.Name)
	default:
		return fmt.Errorf("%w: %s: %q", ErrUnknownTransport, c.Name, c.Transport)
	}

	if err := c.Ordering.Validate(); err != nil {
		return fmt.Errorf("%s: %w", c.Name, err)
	}
	if err := c.Retry.Validate(); err != nil {
		return fmt.Errorf("%s: %w", c.Name, err)
	}
	if c.Ordering.Ordered() && c.Retry.Strategy == SeparateChannel {
		return fmt.Errorf("%w: %s", ErrOrderedChannelWithSeparateRetry, c.Name)
	}

	if c.Redelivery.IsZero() {
		return fmt.Errorf("%w: %s: redelivery (TRP-23)", ErrMissingItem, c.Name)
	}
	if err := c.Redelivery.Validate(); err != nil {
		return fmt.Errorf("%s: %w", c.Name, err)
	}
	if c.Redelivery.Transport() != c.Transport {
		return fmt.Errorf("%w: %s: window is %s, channel is %s", ErrWindowTransportMismatch, c.Name, c.Redelivery.Transport(), c.Transport)
	}

	return c.validateTransport()
}

func (c Channel) validateTransport() error {
	switch c.Transport {
	case Kafka:
		if c.Ordering.Unit == Group {
			return fmt.Errorf("%w: %s: kafka orders by partition (KFK-06)", ErrUnitTransportMismatch, c.Name)
		}
		if err := ValidateAddress(c.Address, c.EnvironmentPrefix); err != nil {
			return fmt.Errorf("%s: %w", c.Name, err)
		}
		switch {
		case c.Group == "":
			return fmt.Errorf("%w: %s: group (KFK-07)", ErrMissingItem, c.Name)
		case c.Partitions <= 0:
			return fmt.Errorf("%w: %s: partitions (KFK-04)", ErrMissingItem, c.Name)
		case c.Partitioner == "":
			return fmt.Errorf("%w: %s: partitioner (KFK-04)", ErrMissingItem, c.Name)
		case c.KeyEncoding == "":
			return fmt.Errorf("%w: %s: keyEncoding (KFK-04)", ErrMissingItem, c.Name)
		}
	case SQS, SNSSQS:
		if c.Ordering.Unit == Partition {
			return fmt.Errorf("%w: %s: sqs orders by message group (SQS-04)", ErrUnitTransportMismatch, c.Name)
		}
	}
	return nil
}

// Catalog is the set of catalogued channels indexed by logical name: the
// destination the application names resolves here to a binding (TRP-08).
type Catalog map[string]Channel

// Resolve returns the channel catalogued under the destination, or
// ErrUnknownChannel.
func (c Catalog) Resolve(destination string) (Channel, error) {
	ch, found := c[destination]
	if !found {
		return Channel{}, fmt.Errorf("%w: %q", ErrUnknownChannel, destination)
	}
	return ch, nil
}

// Validate refuses a catalogue with an invalid channel, a key that differs
// from the channel's name, or two Kafka addresses that differ only by dot and
// underscore (KFK-01c).
func (c Catalog) Validate() error {
	seen := make(map[string]string, len(c))
	for key, ch := range c {
		if key != ch.Name {
			return fmt.Errorf("%w: key %q, name %q", ErrDuplicateChannel, key, ch.Name)
		}
		if err := ch.Validate(); err != nil {
			return err
		}
		if ch.Transport != Kafka {
			continue
		}
		collision := collisionKey(ch.Address)
		if other, clashes := seen[collision]; clashes {
			return fmt.Errorf("%w: %q and %q", ErrAddressCollision, other, ch.Address)
		}
		seen[collision] = ch.Address
	}
	return nil
}
