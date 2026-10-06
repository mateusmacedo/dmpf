package api

import (
	"bytes"
	"context"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLevelDecidesBeforeAnyAttributeIsBuilt(t *testing.T) {
	logger := slog.New(slog.NewJSONHandler(&bytes.Buffer{}, &slog.HandlerOptions{Level: slog.LevelError}))

	if _, enabled := accessLevel(context.Background(), logger, http.StatusOK); enabled {
		t.Fatal("an accepted request is enabled on a logger that only takes errors")
	}
	if level, enabled := accessLevel(context.Background(), logger, http.StatusInternalServerError); !enabled || level != slog.LevelError {
		t.Fatalf("accessLevel(500) = (%v, %v), want (ERROR, true)", level, enabled)
	}
}

func TestLogRequestAppendsTheExtraAttributesToTheAccessRecord(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelError}))
	r := httptest.NewRequest(http.MethodPost, "/orders/o-1/items", nil)

	logRequest(r.Context(), r, logger, slog.LevelError, "/orders/{id}/items", http.StatusInternalServerError, slog.Bool("probe", true))

	for _, want := range []string{`"msg":"http request"`, `"http.route":"/orders/{id}/items"`, `"http.response.status_code":500`, `"probe":true`} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("logged %s, want %s", buf.String(), want)
		}
	}
}
