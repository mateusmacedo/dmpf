package application

import (
	"github.com/mateusmacedo/dmpf/apps/backend/orders/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
)

var itemAcceptedCodec = usecase.OutcomeCodec[domain.ItemAccepted]{
	Encode: func(e *usecase.Encoder, r domain.ItemAccepted) {
		e.String(string(r.Order))
		e.Int(int64(r.Items))
	},
	Decode: func(d *usecase.Decoder) domain.ItemAccepted {
		return domain.ItemAccepted{Order: domain.OrderID(d.String()), Items: int(d.Int())}
	},
}

var placedCodec = usecase.OutcomeCodec[domain.PlacedResponse]{
	Encode: func(e *usecase.Encoder, r domain.PlacedResponse) { e.String(string(r.Order)) },
	Decode: func(d *usecase.Decoder) domain.PlacedResponse {
		return domain.PlacedResponse{Order: domain.OrderID(d.String())}
	},
}
