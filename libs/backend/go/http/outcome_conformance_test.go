package http_test

import (
	"net/http"
	"testing"

	provider "github.com/mateusmacedo/dmpf/libs/backend/go/http"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

func TestOutcomeTableConformsToTheKit(t *testing.T) {
	v := providerkit.ExternalCommand(func() providerkit.ExternalCommandSubject {
		table := outcomeTable()
		return providerkit.ExternalCommandSubject{
			Table: table.Status,
			Classify: func(status int) ports.ExternalClass {
				return table.Classify(&http.Response{StatusCode: status}, nil)
			},
			Validate: func(status map[int]ports.ExternalClass) error {
				return provider.OutcomeTable{Version: table.Version, Status: status}.Validate()
			},
		}
	})
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("skipped = %v, want none", v.Skipped)
	}
}
