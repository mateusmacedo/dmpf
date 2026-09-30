package application

import (
	"github.com/mateusmacedo/dmpf/apps/backend/reservations/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
)

var reservedCodec = usecase.OutcomeCodec[domain.ReservedResponse]{
	Encode: func(e *usecase.Encoder, r domain.ReservedResponse) {
		e.String(string(r.Order))
		e.Int(int64(r.Items))
	},
	Decode: func(d *usecase.Decoder) domain.ReservedResponse {
		return domain.ReservedResponse{Order: domain.OrderID(d.String()), Items: int(d.Int())}
	},
}

var cancelledCodec = usecase.OutcomeCodec[domain.CancelledResponse]{
	Encode: func(e *usecase.Encoder, r domain.CancelledResponse) { e.String(string(r.Order)) },
	Decode: func(d *usecase.Decoder) domain.CancelledResponse {
		return domain.CancelledResponse{Order: domain.OrderID(d.String())}
	},
}
