// comment-discipline-ok-file: arquivo de declarações; cada godoc é contrato de API pública com referência normativa (ENV-08, FND-05 §causationid, FND-07 §8.6), dentro do limite de 3 linhas.

package ports

import "context"

// MessageContext is the trio of ENV-08 attributes with no column of their own:
// the adapter authors it at the edge (FND-07 §8.6 item 3) and the outbox
// persists it, so the relay can drain the record into the envelope.
type MessageContext struct {
	CorrelationID string
	CausationID   string
	Traceparent   string
}

// IsZero reports whether no attribute was authored; the provider writes an
// empty metadata object in that case rather than inventing values (OBX-02).
func (mc MessageContext) IsZero() bool { return mc == MessageContext{} }

type messageContextKey struct{}

// WithMessageContext attaches the attributes to the context so the application
// service can copy them into every OutboxEntry it authors (FND-07 §8.6).
func WithMessageContext(ctx context.Context, mc MessageContext) context.Context {
	return context.WithValue(ctx, messageContextKey{}, mc)
}

// MessageContextFrom returns the attributes attached by WithMessageContext;
// ok is false when the context carries none.
func MessageContextFrom(ctx context.Context) (mc MessageContext, ok bool) {
	mc, ok = ctx.Value(messageContextKey{}).(MessageContext)
	return mc, ok
}
