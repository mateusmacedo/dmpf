package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

func accessLogs(t *testing.T, f fixture) []map[string]any {
	t.Helper()
	var records []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(f.logs.String()), "\n") {
		var record map[string]any
		if err := json.Unmarshal([]byte(line), &record); err == nil && record["msg"] == "http request" {
			records = append(records, record)
		}
	}
	return records
}

func TestEveryRequestIsLoggedWithItsRouteAndStatus(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.post(t, "/orders/o-1/items", `{"sku":"A","quantity":1}`)

	records := accessLogs(t, f)
	if len(records) != 1 {
		t.Fatalf("access log = %v, want one record", f.logs.String())
	}
	record := records[0]
	if record["route"] != "addItem" || record["status"] != float64(201) || record["level"] != "DEBUG" {
		t.Fatalf("access log = %v, want addItem, 201 at DEBUG", record)
	}
	if _, timed := record["duration_ms"]; !timed {
		t.Fatalf("access log = %v, want the duration", record)
	}
}

func TestARefusedRequestIsLoggedAtInfo(t *testing.T) {
	f := newFixture(t, &fakeContexts{})

	f.do(t, "GET", "/orders/o-1", nil, "Authorization", "")

	records := accessLogs(t, f)
	if len(records) != 1 || records[0]["status"] != float64(401) || records[0]["level"] != "INFO" {
		t.Fatalf("access log = %v, want one 401 at INFO", f.logs.String())
	}
}

type panickingAuthenticator struct{}

func (panickingAuthenticator) Authenticate(context.Context, ports.Credential) (ports.Identity, error) {
	panic("authenticator bug")
}

func TestAPanickingRequestIsLoggedAsAServerFailure(t *testing.T) {
	f := newFixture(t, &fakeContexts{}, withAuthenticator(panickingAuthenticator{}))

	response := f.do(t, "GET", "/orders/o-1", nil)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	records := accessLogs(t, f)
	if len(records) != 1 || records[0]["status"] != float64(500) || records[0]["level"] != "WARN" {
		t.Fatalf("access log = %v, want one 500 at WARN", f.logs.String())
	}
}
