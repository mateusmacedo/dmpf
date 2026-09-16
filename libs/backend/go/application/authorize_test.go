package application_test

import (
	"context"
	"testing"

	"github.com/mateusmacedo/dmpf/libs/backend/go/application"
)

type command struct{ Order string }

func TestAllowAllAuthorizesEveryCommand(t *testing.T) {
	authorize := application.AllowAll[command]()

	if err := authorize(context.Background(), command{Order: "P-100"}); err != nil {
		t.Fatalf("AllowAll() = %v, want nil", err)
	}
}
