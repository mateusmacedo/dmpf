package httpedge

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

var tightBudget = deadline.Budget{
	Dependency:        "postgres",
	Method:            "route",
	Limit:             40 * time.Millisecond,
	Slack:             5 * time.Millisecond,
	EstimatedDuration: 10 * time.Millisecond,
}

func TestTheEdgeGovernsTheTimeItDeclares(t *testing.T) {
	var governed bool
	var remaining time.Duration
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		at, ok := r.Context().Deadline()
		governed, remaining = ok, time.Until(at)
	})

	request := httptest.NewRequest(http.MethodGet, "/bookings/booking/b-1", nil)
	withRouteDeadline(tightBudget, next).ServeHTTP(httptest.NewRecorder(), request)

	if !governed {
		t.Fatalf("the handler ran with no deadline: CTX-01 makes the field mandatory and the edge is what regenerates it")
	}
	if remaining <= 0 || remaining > tightBudget.Limit {
		t.Fatalf("remaining = %v, want a positive span within the declared limit of %v", remaining, tightBudget.Limit)
	}
}

func TestTheGovernedTimeExpires(t *testing.T) {
	var expired error
	next := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		expired = r.Context().Err()
	})

	request := httptest.NewRequest(http.MethodGet, "/bookings/booking/b-1", nil)
	withRouteDeadline(tightBudget, next).ServeHTTP(httptest.NewRecorder(), request)

	if expired == nil {
		t.Fatalf("the context never expired: a limit that does not stop the operation governs nothing")
	}
}

func TestTheRequestCarriesNoDeadlineOfItsOwn(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/bookings/booking/b-1", nil)

	if _, governed := request.Context().Deadline(); governed {
		t.Fatalf("the incoming request already carries a deadline, so the tests above would prove nothing about the edge")
	}
}
