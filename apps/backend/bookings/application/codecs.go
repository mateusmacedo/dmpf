package application

import (
	"github.com/mateusmacedo/dmpf/apps/backend/bookings/domain"
	usecase "github.com/mateusmacedo/dmpf/libs/backend/go/application"
)

var reservedCodec = usecase.OutcomeCodec[domain.ReservedResponse]{
	Encode: func(e *usecase.Encoder, r domain.ReservedResponse) { e.String(string(r.BookingID)) },
	Decode: func(d *usecase.Decoder) domain.ReservedResponse {
		return domain.ReservedResponse{BookingID: domain.BookingID(d.String())}
	},
}

var cancelledCodec = usecase.OutcomeCodec[domain.CancelledResponse]{
	Encode: func(e *usecase.Encoder, r domain.CancelledResponse) { e.String(string(r.BookingID)) },
	Decode: func(d *usecase.Decoder) domain.CancelledResponse {
		return domain.CancelledResponse{BookingID: domain.BookingID(d.String())}
	},
}

var registeredCodec = usecase.OutcomeCodec[domain.RegisteredResponse]{
	Encode: func(e *usecase.Encoder, r domain.RegisteredResponse) { e.String(string(r.Code)) },
	Decode: func(d *usecase.Decoder) domain.RegisteredResponse {
		return domain.RegisteredResponse{Code: domain.ResourceCode(d.String())}
	},
}
