package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	kernelhttp "github.com/mateusmacedo/dmpf/libs/backend/go/http"
)

func TestRequireIdempotencyKeyDecidesByTheRoute(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	keyed := kernelhttp.Route{Method: http.MethodPost, IdempotencyKey: IdempotencyHeader}
	unkeyed := kernelhttp.Route{Method: http.MethodPost}
	other := kernelhttp.Route{Method: http.MethodPost, IdempotencyKey: "X-Command-Key"}
	cases := map[string]struct {
		route  kernelhttp.Route
		header string
		key    string
		want   int
		code   string
	}{
		"keyed route without a key":            {keyed, IdempotencyHeader, "", http.StatusBadRequest, "missing-idempotency-key"},
		"keyed route with a valid key":         {keyed, IdempotencyHeader, "k-1", http.StatusNoContent, ""},
		"keyed route with an invalid key":      {keyed, IdempotencyHeader, " ", http.StatusBadRequest, "invalid-idempotency-key"},
		"unkeyed route without a key":          {unkeyed, IdempotencyHeader, "", http.StatusNoContent, ""},
		"unkeyed route with an invalid key":    {unkeyed, IdempotencyHeader, "abcé", http.StatusBadRequest, "invalid-idempotency-key"},
		"route header carries the key":         {other, "X-Command-Key", "k-1", http.StatusNoContent, ""},
		"route header absent, default present": {other, IdempotencyHeader, "k-1", http.StatusBadRequest, "X-Command-Key header"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(tc.route.Method, "/probe", nil)
			if tc.key != "" {
				req.Header.Set(tc.header, tc.key)
			}
			rec := httptest.NewRecorder()

			requireIdempotencyKey(tc.route, next).ServeHTTP(rec, req)

			if rec.Code != tc.want {
				t.Fatalf("status = %d, want %d", rec.Code, tc.want)
			}
			if tc.code != "" && !strings.Contains(rec.Body.String(), tc.code) {
				t.Fatalf("body = %s, want it to mention %s", rec.Body.String(), tc.code)
			}
		})
	}
}
