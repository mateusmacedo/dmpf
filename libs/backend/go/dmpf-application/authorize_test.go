package dmpfapplication_test

import (
	"context"
	"testing"

	dmpfapplication "gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-application"
)

type command struct{ Order string }

func TestAllowAllAuthorizesEveryCommand(t *testing.T) {
	authorize := dmpfapplication.AllowAll[command]()

	if err := authorize(context.Background(), command{Order: "P-100"}); err != nil {
		t.Fatalf("AllowAll() = %v, want nil", err)
	}
}
