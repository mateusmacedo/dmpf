package bookingsapp

import (
	"net/http"

	dmpfhttp "github.com/mateusmacedo/dmpf/libs/backend/go/dmpf-provider-http"
)

const IdempotencyHeader = "Idempotency-Key"

const contract = "contracts/openapi/bookings/v1/openapi.yaml#/paths/"

func Routes() [5]dmpfhttp.Route {
	return [5]dmpfhttp.Route{
		{
			Name:           "reserveBooking",
			Method:         http.MethodPost,
			Path:           "/bookings/booking",
			ContractRef:    contract + "~1bookings~1booking/post",
			IdempotencyKey: IdempotencyHeader,
		},
		{
			Name:           "cancelBooking",
			Method:         http.MethodPost,
			Path:           "/bookings/booking/{id}/cancel",
			ContractRef:    contract + "~1bookings~1booking~1{id}~1cancel/post",
			IdempotencyKey: IdempotencyHeader,
		},
		{
			Name:           "registerResource",
			Method:         http.MethodPost,
			Path:           "/bookings/resource",
			ContractRef:    contract + "~1bookings~1resource/post",
			IdempotencyKey: IdempotencyHeader,
		},
		{
			Name:        "findBooking",
			Method:      http.MethodGet,
			Path:        "/bookings/booking/{id}",
			ContractRef: contract + "~1bookings~1booking~1{id}/get",
		},
		{
			Name:        "findBookingByResource",
			Method:      http.MethodGet,
			Path:        "/bookings/booking",
			ContractRef: contract + "~1bookings~1booking/get",
		},
	}
}
