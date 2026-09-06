// comment-discipline-ok-file: arquivo de contrato interno; cada godoc cita a regra de FND-06 (SQS-08..10, TRP-26, TRP-27) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfsqs

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sqs"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/channel"
)

// acknowledger realizes dmpfports.Acknowledger for one receipt: Ack deletes by
// receipt handle (SQS-09), Release shortens the visibility to the backoff
// (SQS-10); both stop the heartbeat first, so no extension follows (SQS-08).
type acknowledger struct {
	api           sqsAPI
	queueURL      string
	receipt       string
	backoff       time.Duration
	stopHeartbeat func()

	mu       sync.Mutex
	disposed bool
}

var _ dmpfports.Acknowledger = (*acknowledger)(nil)

func newAcknowledger(api sqsAPI, queueURL, receipt string, backoff time.Duration, stopHeartbeat func()) *acknowledger {
	if stopHeartbeat == nil {
		stopHeartbeat = func() {}
	}
	return &acknowledger{api: api, queueURL: queueURL, receipt: receipt, backoff: backoff, stopHeartbeat: stopHeartbeat}
}

func (a *acknowledger) Ack(ctx context.Context) error {
	if err := a.dispose(); err != nil {
		return err
	}
	_, err := a.api.DeleteMessage(ctx, &sqs.DeleteMessageInput{
		QueueUrl:      aws.String(a.queueURL),
		ReceiptHandle: aws.String(a.receipt),
	})
	return err
}

func (a *acknowledger) Release(ctx context.Context) error {
	if err := a.dispose(); err != nil {
		return err
	}
	_, err := a.api.ChangeMessageVisibility(ctx, &sqs.ChangeMessageVisibilityInput{
		QueueUrl:          aws.String(a.queueURL),
		ReceiptHandle:     aws.String(a.receipt),
		VisibilityTimeout: visibilitySeconds(a.backoff),
	})
	return err
}

// dispose takes the single gesture (TRP-27) and stops the heartbeat before it,
// waiting for a tick in flight: an extension that landed after a delete would
// fail, and one after a release would undo the backoff.
func (a *acknowledger) dispose() error {
	a.mu.Lock()
	if a.disposed {
		a.mu.Unlock()
		return ErrAlreadyDisposed
	}
	a.disposed = true
	a.mu.Unlock()
	a.stopHeartbeat()
	return nil
}

// MaxVisibilitySeconds is the queue's ceiling on one visibility value: the 12h
// of SQS-08b that channel.MaxVisibility declares once for the workspace.
const MaxVisibilitySeconds = int32(channel.MaxVisibility / time.Second)

// visibilitySeconds rounds a duration up to whole seconds within [0, 12h].
func visibilitySeconds(d time.Duration) int32 {
	if d <= 0 {
		return 0
	}
	seconds := math.Ceil(d.Seconds())
	if seconds > float64(MaxVisibilitySeconds) {
		return MaxVisibilitySeconds
	}
	return int32(seconds)
}
