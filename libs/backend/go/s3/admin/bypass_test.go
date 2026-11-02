package admin

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsmiddleware "github.com/aws/aws-sdk-go-v2/aws/middleware"
	"github.com/aws/smithy-go/middleware"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// bypassProbe is a Finalize middleware of the SDK that records, per operation,
// whether the signed request carried the governance bypass.
type bypassProbe struct {
	mu   sync.Mutex
	seen map[string]bool
}

func (p *bypassProbe) option(stack *middleware.Stack) error {
	return stack.Finalize.Add(middleware.FinalizeMiddlewareFunc("dmpf-bypass-probe",
		func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
			if req, ok := in.Request.(*smithyhttp.Request); ok {
				op := awsmiddleware.GetOperationName(ctx)
				p.mu.Lock()
				p.seen[op] = p.seen[op] || req.Header.Get("X-Amz-Bypass-Governance-Retention") == "true"
				p.mu.Unlock()
			}
			return next.HandleFinalize(ctx, in)
		}), middleware.After)
}

// WHY: the store's probe only ever sees the header absent; seeing it present
// here, through the real client, is what proves the probe can tell the two apart.
func TestAdminSendsTheBypassOnlyWhereItShortensOrPurges(t *testing.T) {
	stored := now() - 60*second
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		switch r.Method {
		case http.MethodHead:
			h := head(stored, &ports.Retention{Mode: ports.RetentionGovernance, Until: now() + 100*second}, false)
			w.Header().Set("x-amz-version-id", *h.VersionId)
			w.Header().Set("x-amz-checksum-sha256", *h.ChecksumSHA256)
			w.Header().Set("Content-Length", strconv.FormatInt(*h.ContentLength, 10))
			w.Header().Set("x-amz-meta-dmpf-stored-at", h.Metadata["dmpf-stored-at"])
			w.Header().Set("x-amz-object-lock-mode", "GOVERNANCE")
			w.Header().Set("x-amz-object-lock-retain-until-date", h.ObjectLockRetainUntilDate.Format(time.RFC3339))
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotImplemented)
		}
	}))
	defer server.Close()
	probe := &bypassProbe{seen: map[string]bool{}}
	cfg := validConfig()
	cfg.Endpoint, cfg.InsecureForDevelopmentOnly, cfg.UsePathStyle = server.URL, true, true
	cfg.AWS.Credentials = aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
	})
	cfg.AWS.APIOptions = append(cfg.AWS.APIOptions, probe.option)
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	ctx := tenantCtx(t, "acme")
	if _, err := a.Shorten(ctx, ref, ports.RetentionCeiling{Span: ports.RetentionSpan(90 * time.Second), Origin: "ruling"}); err != nil {
		t.Fatalf("Shorten() = %v", err)
	}
	if _, err := a.Purge(ctx, ref, ports.RetentionCeiling{Span: ports.RetentionSpan(30 * time.Second), Origin: "ruling"}); err != nil {
		t.Fatalf("Purge() = %v", err)
	}
	want := map[string]bool{"HeadObject": false, "PutObjectRetention": true, "DeleteObject": true}
	for op, bypass := range want {
		sent, seen := probe.seen[op]
		if !seen || sent != bypass {
			t.Errorf("%s: bypass sent %v (seen %v), want %v", op, sent, seen, bypass)
		}
	}
}
