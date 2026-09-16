// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (TRP-22/23/24/24b, SQS-08b) que o símbolo realiza, dentro do limite de 3 linhas.

package channel

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// MaxVisibility is the ceiling on accumulated visibility extensions, counted
// from receipt (SQS-08b): past that point the message returns to the queue
// whatever the consumer does.
const MaxVisibility = 12 * time.Hour

// The formulas of TRP-24, one per transport, with their variables named.
const (
	FormulaKafka  = "kafka: min(retentionByTime, retentionBySize as time) under cleanupPolicy, remoteStorage and initialOffset"
	FormulaSQS    = "sqs: min(maxReceiveCount × min(visibilityBase, 12h), retention)"
	FormulaSNSSQS = "sns-sqs: subscriptionDeliveryWindow + sqs.upperBound"
)

var requiredParams = map[string][]string{
	FormulaKafka:  {"retentionByTime", "retentionBySize", "cleanupPolicy", "remoteStorage", "initialOffset"},
	FormulaSQS:    {"maxReceiveCount", "visibilityBase", "retention"},
	FormulaSNSSQS: {"subscriptionDeliveryWindow", "sqs.upperBound"},
}

var formulaTransport = map[string]Transport{
	FormulaKafka:  Kafka,
	FormulaSQS:    SQS,
	FormulaSNSSQS: SNSSQS,
}

// ErrInvalidWindow is a redelivery window without a closed upper bound or with
// a parameter of its formula missing: "depends on configuration" is not a
// formula (TRP-24, TRP-24b).
var ErrInvalidWindow = errors.New("channel: redelivery window is not verifiable (TRP-24)")

// RedeliveryWindow is the horizon of automatic redelivery of a channel — the
// third clock of TRP-22 — with the formula and the named parameters that
// produce its closed upper bound (TRP-23, TRP-24, TRP-24b).
type RedeliveryWindow struct {
	Formula    string
	Params     map[string]string
	UpperBound time.Duration
}

// IsZero reports whether no window was declared at all.
func (w RedeliveryWindow) IsZero() bool {
	return w.Formula == "" && len(w.Params) == 0 && w.UpperBound == 0
}

// Transport is the transport whose formula produced the window, or empty for
// an unknown formula.
func (w RedeliveryWindow) Transport() Transport { return formulaTransport[w.Formula] }

// Validate refuses an unknown formula, an upper bound that is not positive and
// any parameter of the formula absent or empty (TRP-24, TRP-24b).
func (w RedeliveryWindow) Validate() error {
	required, known := requiredParams[w.Formula]
	if !known {
		return fmt.Errorf("%w: formula %q is unknown", ErrInvalidWindow, w.Formula)
	}
	for _, name := range required {
		if strings.TrimSpace(w.Params[name]) == "" {
			return fmt.Errorf("%w: parameter %s absent (TRP-24b)", ErrInvalidWindow, name)
		}
	}
	if w.UpperBound <= 0 {
		return fmt.Errorf("%w: upper bound is %v", ErrInvalidWindow, w.UpperBound)
	}
	return nil
}

// KafkaRetention are the five parameters TRP-24b requires of a Kafka channel.
// SizeHorizon is the time the size retention translates to when the operator
// has measured it smaller than the time retention; zero leaves time deciding.
type KafkaRetention struct {
	RetentionByTime time.Duration
	RetentionBySize int64
	SizeHorizon     time.Duration
	CleanupPolicy   string
	RemoteStorage   bool
	InitialOffset   string
}

// KafkaWindow is the Kafka line of TRP-24: the effective retention of the
// record. A policy without delete never expires a record, so the bound stays
// zero and Validate refuses it; retentionBySize −1 declares no size limit.
func KafkaWindow(r KafkaRetention) RedeliveryWindow {
	params := map[string]string{
		"cleanupPolicy": r.CleanupPolicy,
		"remoteStorage": strconv.FormatBool(r.RemoteStorage),
		"initialOffset": r.InitialOffset,
	}
	if r.RetentionByTime > 0 {
		params["retentionByTime"] = r.RetentionByTime.String()
	}
	if r.RetentionBySize != 0 {
		params["retentionBySize"] = strconv.FormatInt(r.RetentionBySize, 10) + " bytes"
	}

	var bound time.Duration
	if strings.Contains(r.CleanupPolicy, "delete") {
		bound = r.RetentionByTime
		if r.SizeHorizon > 0 && r.SizeHorizon < bound {
			bound = r.SizeHorizon
			params["sizeHorizon"] = r.SizeHorizon.String()
		}
	}

	return RedeliveryWindow{Formula: FormulaKafka, Params: params, UpperBound: bound}
}

// SQSWindow is the SQS line of TRP-24: receipts times the visibility of each
// attempt under the 12h ceiling of SQS-08b, all bounded by the queue's
// retention — the smaller of the two is the horizon.
func SQSWindow(maxReceiveCount int, visibilityBase, retention time.Duration) RedeliveryWindow {
	params := map[string]string{}
	if maxReceiveCount > 0 {
		params["maxReceiveCount"] = strconv.Itoa(maxReceiveCount)
	}
	if visibilityBase > 0 {
		params["visibilityBase"] = visibilityBase.String()
	}
	if retention > 0 {
		params["retention"] = retention.String()
	}

	perAttempt := min(visibilityBase, MaxVisibility)
	bound := min(time.Duration(maxReceiveCount)*perAttempt, retention)
	if maxReceiveCount <= 0 || visibilityBase <= 0 || retention <= 0 {
		bound = 0
	}

	return RedeliveryWindow{Formula: FormulaSQS, Params: params, UpperBound: bound}
}

// SNSSQSWindow is the SNS → SQS line of TRP-24: the sum of two windows, the
// subscription's delivery policy and the queue's own. Taking only the queue's
// underestimates the horizon.
func SNSSQSWindow(subscriptionDeliveryWindow time.Duration, sqs RedeliveryWindow) RedeliveryWindow {
	params := map[string]string{}
	if subscriptionDeliveryWindow > 0 {
		params["subscriptionDeliveryWindow"] = subscriptionDeliveryWindow.String()
	}
	for name, value := range sqs.Params {
		params["sqs."+name] = value
	}

	var bound time.Duration
	if sqs.Formula == FormulaSQS && sqs.Validate() == nil && subscriptionDeliveryWindow > 0 {
		params["sqs.upperBound"] = sqs.UpperBound.String()
		bound = subscriptionDeliveryWindow + sqs.UpperBound
	}

	return RedeliveryWindow{Formula: FormulaSNSSQS, Params: params, UpperBound: bound}
}
