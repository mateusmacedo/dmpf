package tb_test

import (
	"errors"
	"strings"
	"testing"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/domainkit"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/golden"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-testkit/tb"
)

const projectionDoc = `{
  "format_version": "1",
  "identity": {"context": "orders", "aggregate": "order"},
  "cases": [{
    "name": "place-accepted", "doc": "d",
    "state_before": {"status": "open", "items.count": "1"},
    "command": {"upr": "place", "at": "10"},
    "expected": {
      "branch": "accepted",
      "response": {"order": "o-1"},
      "events": [{"name": "orders.order-placed", "fields": {"order": "o-1", "items": "1", "at": "10"}}],
      "state_after": {"status": "placed", "items.count": "1"}
    }
  }]
}`

func TestProjectionDecodesAndProjects(t *testing.T) {
	f, err := tb.DecodeProjection([]byte(projectionDoc))
	if err != nil {
		t.Fatal(err)
	}
	if f.Identity.Aggregate != "order" || len(f.Cases) != 1 {
		t.Fatalf("fixture = %+v", f)
	}
	p := f.Cases[0].Expected.Projection()
	if p.Branch != domainkit.Accepted || p.Response["order"] != "o-1" || len(p.Events) != 1 || p.Events[0].Fields["items"] != "1" || p.StateAfter["status"] != "placed" {
		t.Fatalf("projection = %+v", p)
	}
}

func TestProjectionRejectsUnknownVersionAndNumbers(t *testing.T) {
	_, err := tb.DecodeProjection([]byte(strings.Replace(projectionDoc, `"format_version": "1"`, `"format_version": "9"`, 1)))
	if !errors.Is(err, tb.ErrProjectionFormatVersion) {
		t.Fatalf("err = %v, want ErrProjectionFormatVersion", err)
	}
	_, err = tb.DecodeProjection([]byte(strings.Replace(projectionDoc, `"at": "10"`, `"at": 10`, 1)))
	if !errors.Is(err, golden.ErrNonStringScalar) || !strings.Contains(err.Error(), "cases[0].command.at") {
		t.Fatalf("err = %v, want ErrNonStringScalar at cases[0].command.at", err)
	}
}
