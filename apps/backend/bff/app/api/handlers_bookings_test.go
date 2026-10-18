package api_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	bookingsv1 "github.com/mateusmacedo/dmpf/apps/backend/bookings/contract/gen/go/company/bookings/service/v1"
)

func TestEveryBookingsRouteAnswersItsSuccess(t *testing.T) {
	f := newFixture(t, &fakeContexts{})
	cases := []struct {
		method, path, body string
		status             int
		want               string
	}{
		{http.MethodPost, "/bookings/booking", `{"bookingId":"b-1","resourceId":"room-1","quantity":2}`, http.StatusCreated, `{"bookingId":"b-1"}`},
		{http.MethodPost, "/bookings/booking/b-1/cancel", "", http.StatusOK, `{"bookingId":"b-1"}`},
		{http.MethodPost, "/bookings/resource", `{"code":"room-1","name":"Sala 1"}`, http.StatusCreated, `{"code":"room-1"}`},
		{http.MethodGet, "/bookings/booking/b-1", "", http.StatusOK, `{"id":"b-1","resourceId":"room-1","quantity":2,"status":"reserved","reservedAt":1755432000000000000}`},
		{http.MethodGet, "/bookings/booking?resourceId=room-1", "", http.StatusOK, `[{"id":"b-1","resourceId":"room-1","quantity":2,"status":"cancelled","reservedAt":1755432000000000000}]`},
	}
	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			rec := f.do(t, tc.method, tc.path, strings.NewReader(tc.body), "Idempotency-Key", "k-1")
			if rec.Code != tc.status || strings.TrimSpace(rec.Body.String()) != tc.want {
				t.Fatalf("%d %s, want %d %s", rec.Code, rec.Body.String(), tc.status, tc.want)
			}
		})
	}
}

func TestReserveBookingForwardsTheRequestOfTheContract(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.post(t, "/bookings/booking", `{"bookingId":"b-1","resourceId":"room-1","quantity":2}`)

	calls := f.fake.callsTo("ReserveBooking")
	req, ok := calls[0].request.(*bookingsv1.ReserveBookingRequest)
	if len(calls) != 1 || !ok || req.GetBookingId() != "b-1" || req.GetResourceId() != "room-1" || req.GetQuantity() != 2 {
		t.Fatalf("ReserveBooking request = %v, want b-1, room-1, 2", calls)
	}
}

func TestRegisterResourceSendsTheCodeAsTheResourceIDAndTheName(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.post(t, "/bookings/resource", `{"code":"room-1","name":"Sala 1"}`)

	calls := f.fake.callsTo("RegisterResource")
	req, ok := calls[0].request.(*bookingsv1.RegisterResourceRequest)
	if len(calls) != 1 || !ok || req.GetResourceId() != "room-1" || req.GetName() != "Sala 1" {
		t.Fatalf("RegisterResource request = %v, want resource room-1 named Sala 1", calls)
	}
}

func TestANameOf200CharactersOutsideASCIIIsAccepted(t *testing.T) {
	f := newFixture(t, &fakeContexts{})
	name := strings.Repeat("á", 200)

	rec := f.post(t, "/bookings/resource", `{"code":"room-1","name":"`+name+`"}`)

	calls := f.fake.callsTo("RegisterResource")
	if rec.Code != http.StatusCreated || len(calls) != 1 {
		t.Fatalf("status = %d with %d calls, want 201 with 1 (body %s)", rec.Code, len(calls), rec.Body.String())
	}
	if req, ok := calls[0].request.(*bookingsv1.RegisterResourceRequest); !ok || req.GetName() != name {
		t.Fatalf("RegisterResource request = %v, want the name intact", calls[0].request)
	}
}

func TestRegisterResourceAppliesTheNameRule(t *testing.T) {
	for _, tc := range nameRuleCases {
		t.Run(tc.desc, func(t *testing.T) {
			f := newFixture(t, &fakeContexts{})

			rec := f.post(t, "/bookings/resource", registerBody(t, tc.name))

			calls := f.fake.callsTo("RegisterResource")
			if !tc.accepted {
				if rec.Code != http.StatusBadRequest || len(calls) != 0 {
					t.Fatalf("status = %d with %d calls, want 400 with none for %q (body %s)", rec.Code, len(calls), tc.name, rec.Body.String())
				}
				return
			}
			if rec.Code != http.StatusCreated || len(calls) != 1 {
				t.Fatalf("status = %d with %d calls, want 201 with 1 for %q (body %s)", rec.Code, len(calls), tc.name, rec.Body.String())
			}
			if req, ok := calls[0].request.(*bookingsv1.RegisterResourceRequest); !ok || req.GetName() != tc.name {
				t.Fatalf("RegisterResource request = %v, want the name %q intact", calls[0].request, tc.name)
			}
		})
	}
}

func registerBody(t *testing.T, name string) string {
	t.Helper()
	body, err := json.Marshal(registerResource{Code: "room-1", Name: name})
	if err != nil {
		t.Fatalf("json.Marshal() = %v", err)
	}
	return string(body)
}

type registerResource struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

func TestMalformedBookingsRequestsNeverCallTheContext(t *testing.T) {
	cases := map[string]struct {
		method, path, body string
	}{
		"quantity above the ceiling": {http.MethodPost, "/bookings/booking", `{"bookingId":"b-1","resourceId":"room-1","quantity":101}`},
		"booking id with a space":    {http.MethodPost, "/bookings/booking", `{"bookingId":"b 1","resourceId":"room-1","quantity":1}`},
		"unknown field":              {http.MethodPost, "/bookings/booking", `{"bookingId":"b-1","resourceId":"room-1","quantity":1,"extra":true}`},
		"empty code":                 {http.MethodPost, "/bookings/resource", `{"code":"","name":"Sala 12"}`},
		"code with a space":          {http.MethodPost, "/bookings/resource", `{"code":"sala 12","name":"Sala 12"}`},
		"missing name":               {http.MethodPost, "/bookings/resource", `{"code":"room-1"}`},
		"empty name":                 {http.MethodPost, "/bookings/resource", `{"code":"room-1","name":""}`},
		"name above 200 characters":  {http.MethodPost, "/bookings/resource", `{"code":"room-1","name":"` + strings.Repeat("á", 201) + `"}`},
		"name with a control":        {http.MethodPost, "/bookings/resource", `{"code":"room-1","name":"Sala\u000712"}`},
		"name with a line break":     {http.MethodPost, "/bookings/resource", `{"code":"room-1","name":"Sala\n12"}`},
		"missing resource filter":    {http.MethodGet, "/bookings/booking", ""},
		"resource filter with space": {http.MethodGet, "/bookings/booking?resourceId=sala%2012", ""},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t, &fakeContexts{})

			rec := f.do(t, tc.method, tc.path, strings.NewReader(tc.body), "Idempotency-Key", "k-1", "Content-Type", "application/json")

			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400 (body %s)", rec.Code, rec.Body.String())
			}
			if n := f.fake.total(); n != 0 {
				t.Fatalf("calls = %d, want 0", n)
			}
		})
	}
}

func TestADomainRefusalOfTheBookingsContextIs422(t *testing.T) {
	f := newFixture(t, (&fakeContexts{}).on("CancelBooking", func(int) (any, error) {
		return &bookingsv1.CancelBookingResponse{Result: &bookingsv1.CancelBookingResponse_Rejection{
			Rejection: &bookingsv1.Rejection{Code: "resource-scheduling/booking/not-reserved", Message: "the booking is not reserved"},
		}}, nil
	}))

	rec := f.do(t, http.MethodPost, "/bookings/booking/b-1/cancel", nil, "Idempotency-Key", "k-1")

	requireRejection(t, rec, http.StatusUnprocessableEntity, "resource-scheduling/booking/not-reserved")
}

func TestADuplicateResourceNameIs422(t *testing.T) {
	f := newFixture(t, (&fakeContexts{}).on("RegisterResource", func(int) (any, error) {
		return &bookingsv1.RegisterResourceResponse{Result: &bookingsv1.RegisterResourceResponse_Rejection{
			Rejection: &bookingsv1.Rejection{Code: "resource-scheduling/resource/duplicate-name", Message: "another resource of the tenant has this name"},
		}}, nil
	}))

	rec := f.post(t, "/bookings/resource", `{"code":"room-2","name":"Sala 1"}`)

	requireRejection(t, rec, http.StatusUnprocessableEntity, "resource-scheduling/resource/duplicate-name")
}

func TestAnUnknownBookingIs404(t *testing.T) {
	f := newFixture(t, (&fakeContexts{}).on("FindBooking", func(int) (any, error) {
		return nil, status.Error(codes.NotFound, "not found")
	}))

	rec := f.do(t, http.MethodGet, "/bookings/booking/b-none", nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 (body %s)", rec.Code, rec.Body.String())
	}
}
