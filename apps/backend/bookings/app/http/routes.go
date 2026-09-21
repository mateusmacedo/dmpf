package httpedge

import (
	"net/http"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

const IdempotencyHeader = "Idempotency-Key"

const contract = "contracts/openapi/bookings/v1/openapi.yaml#/paths/"

// Routes declares the five operations this edge serves. The budget is an
// argument because the deadline belongs to the edge policy resolved at startup,
// never to anything the caller sends (FND-07 §3.5).
func Routes(budget deadline.Budget) [5]provider.Route {
	return [5]provider.Route{
		{
			Name:           "reserveBooking",
			Method:         http.MethodPost,
			Path:           "/bookings/booking",
			ContractRef:    contract + "~1bookings~1booking/post",
			Budget:         budget,
			IdempotencyKey: IdempotencyHeader,
		},
		{
			Name:           "cancelBooking",
			Method:         http.MethodPost,
			Path:           "/bookings/booking/{id}/cancel",
			ContractRef:    contract + "~1bookings~1booking~1{id}~1cancel/post",
			Budget:         budget,
			IdempotencyKey: IdempotencyHeader,
		},
		{
			Name:           "registerResource",
			Method:         http.MethodPost,
			Path:           "/bookings/resource",
			ContractRef:    contract + "~1bookings~1resource/post",
			Budget:         budget,
			IdempotencyKey: IdempotencyHeader,
		},
		{
			Name:        "findBooking",
			Method:      http.MethodGet,
			Path:        "/bookings/booking/{id}",
			ContractRef: contract + "~1bookings~1booking~1{id}/get",
			Budget:      budget,
		},
		{
			Name:        "findBookingByResource",
			Method:      http.MethodGet,
			Path:        "/bookings/booking",
			ContractRef: contract + "~1bookings~1booking/get",
			Budget:      budget,
		},
	}
}
