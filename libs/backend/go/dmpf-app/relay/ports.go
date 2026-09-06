// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (FND-04 §5.4, OBX-08, OBX-10, BLK-04), dentro do limite de 3 linhas.

package relay

import (
	"context"
	"time"

	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
	dmpfpostgres "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-provider-postgres"
)

// Store is the drain side of the outbox as this loop needs it. It is declared
// here, in the consumer, and satisfied structurally by dmpfpostgres.OutboxStore.
type Store interface {
	// Claim acquires up to limit eligible records under claimID, writing the
	// four fields of OBX-16 in one commit.
	Claim(ctx context.Context, claimID string, limit int, lease time.Duration) ([]dmpfpostgres.Claimed, error)

	// MarkPublished, Reschedule and Fail are the three outcomes of step 3. All
	// three report rows affected; zero means the claim was replaced (OBX-10).
	MarkPublished(ctx context.Context, id int64, claimID string) (int64, error)
	Reschedule(ctx context.Context, id int64, claimID string, availableAt dmpfports.Instant, lastError string) (int64, error)
	Fail(ctx context.Context, id int64, claimID string, lastError string) (int64, error)

	// OutboxSignals is the drain-side reading of OBX-12, on the same store
	// because all four signals come from the same table.
	OutboxSignals(ctx context.Context) (dmpfpostgres.OutboxHealth, error)
}

// Publisher hands the serialized envelope to a transport. It is declared here
// rather than in dmpf-ports because publishing happens after the commit and
// outside the unit of work, which that block's own godoc already decided.
type Publisher interface {
	// Publish routes by destination, the logical flow name the application
	// service authored (BLK-04); the destination never enters the envelope.
	//
	// It must return when ctx is done. The loop waits for every publisher in
	// flight before releasing the claims it holds, so one that blocks past
	// cancellation holds the shutdown open for as long as it blocks (OBX-13).
	Publish(ctx context.Context, destination string, message []byte) error
}

// ClaimIDs is the source of claim identity. OBX-08 makes it the identity of one
// acquisition, not of the process: two acquisitions of the same record by the
// same worker must not share it.
type ClaimIDs interface {
	NewClaimID() string
}
