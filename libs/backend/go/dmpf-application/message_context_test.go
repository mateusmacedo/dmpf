package dmpfapplication_test

import (
	"context"
	"testing"

	dmpfapplication "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-application"
	dmpfports "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-ports"
)

const traceparent = "00-0af7651916cd43dd8448eb211c80319c-b7ad6b7169203331-01"

func TestMessageContextForCopiesTheAuthoredAttributesAndFillsOnlyTheCausation(t *testing.T) {
	tests := []struct {
		name string
		ctx  context.Context
		want dmpfports.MessageContext
	}{
		{
			name: "nothing authored: the message starts the chain",
			ctx:  context.Background(),
			want: dmpfports.MessageContext{CausationID: "m-000001"},
		},
		{
			name: "origin authored correlation and trace: causation is the message itself",
			ctx:  dmpfports.WithMessageContext(context.Background(), dmpfports.MessageContext{CorrelationID: "corr-1", Traceparent: traceparent}),
			want: dmpfports.MessageContext{CorrelationID: "corr-1", CausationID: "m-000001", Traceparent: traceparent},
		},
		{
			name: "consumption authored the causing message: it is kept",
			ctx:  dmpfports.WithMessageContext(context.Background(), dmpfports.MessageContext{CorrelationID: "corr-1", CausationID: "m-ext-1", Traceparent: traceparent}),
			want: dmpfports.MessageContext{CorrelationID: "corr-1", CausationID: "m-ext-1", Traceparent: traceparent},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := dmpfapplication.MessageContextFor(tt.ctx, "m-000001"); got != tt.want {
				t.Fatalf("MessageContextFor() = %+v, want %+v (FND-05 ENV-08: causationid = id when the message starts the chain)", got, tt.want)
			}
		})
	}
}
