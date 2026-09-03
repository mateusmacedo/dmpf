// comment-discipline-ok-file: arquivo de declarações; o godoc de ResolveIdentity é o contrato do passo 2 de FND-04 §3.2 e o rationale de UOW-09, dentro do limite de 3 linhas.

package dmpfapplication

import (
	dmpfports "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-ports"
)

// Identity is the identity and time of the facts a single use case execution
// may produce, resolved before the transaction opens.
type Identity struct {
	OccurredAt dmpfports.Instant
	MessageIDs []dmpfports.MessageID
}

// ResolveIdentity is step 2 of FND-04 §3.2: it reads the clock once and takes
// one identifier per event the use case declares it may produce. It runs before
// Within, because a re-execution would mint new identity for the same fact.
//
// An empty or repeated identifier is a provider defect and panics: two facts
// sharing one message_id would break deduplication downstream. A negative
// events count is a programming defect and panics as well.
func ResolveIdentity(clock dmpfports.Clock, ids dmpfports.IDGenerator, events int) Identity {
	if events < 0 {
		panic("dmpfapplication: ResolveIdentity requires a non-negative event count")
	}

	identity := Identity{OccurredAt: clock.Now(), MessageIDs: make([]dmpfports.MessageID, 0, events)}
	for range events {
		next := ids.NewMessageID()
		if next == "" {
			panic("dmpfapplication: IDGenerator produced an empty MessageID")
		}
		for _, taken := range identity.MessageIDs {
			if taken == next {
				panic("dmpfapplication: IDGenerator repeated a MessageID within one resolution")
			}
		}
		identity.MessageIDs = append(identity.MessageIDs, next)
	}
	return identity
}
