package dmpfapplication

import (
	"context"

	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

// MessageContextFor is what an application service writes into every
// OutboxEntry it authors: the attributes the adapter attached to the context
// (FND-07 §8.6 item 3), with the one gap only the service can fill — a message
// that starts a chain carries its own id as causationid (FND-05 ENV-08).
func MessageContextFor(ctx context.Context, id dmpfports.MessageID) dmpfports.MessageContext {
	mc, _ := dmpfports.MessageContextFrom(ctx)
	if mc.CausationID == "" {
		mc.CausationID = string(id)
	}
	return mc
}
