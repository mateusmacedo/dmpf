package s3

import (
	"context"
	"errors"
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
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/worm"
)

const bypassHeader = "X-Amz-Bypass-Governance-Retention"

// bypassProbe is a Finalize middleware of the SDK: it sees each request as it
// leaves, signed, and records per operation whether it carried the bypass.
type bypassProbe struct {
	mu   sync.Mutex
	seen map[string]bool
}

func (p *bypassProbe) option(stack *middleware.Stack) error {
	return stack.Finalize.Add(middleware.FinalizeMiddlewareFunc("dmpf-bypass-probe",
		func(ctx context.Context, in middleware.FinalizeInput, next middleware.FinalizeHandler) (middleware.FinalizeOutput, middleware.Metadata, error) {
			if req, ok := in.Request.(*smithyhttp.Request); ok {
				p.mu.Lock()
				p.seen[awsmiddleware.GetOperationName(ctx)] = p.seen[awsmiddleware.GetOperationName(ctx)] || req.Header.Get(bypassHeader) != ""
				p.mu.Unlock()
			}
			return next.HandleFinalize(ctx, in)
		}), middleware.After)
}

// fakeS3 answers each operation the store sends the way S3 does, for a version
// whose retention has expired and that has no hold, so every path is taken.
func fakeS3(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	_, sum := digestOf(body)
	checksum := worm.Checksum(sum)
	expired := time.Unix(1_700_000_000, 0).UTC().Format(time.RFC3339)
	version := func(w http.ResponseWriter) {
		h := w.Header()
		h.Set("x-amz-version-id", "v1")
		h.Set("x-amz-checksum-sha256", checksum)
		h.Set("Content-Type", "application/x-test")
		h.Set("x-amz-meta-dmpf-stored-at", strconv.FormatInt(int64(nowInstant()), 10))
		h.Set("x-amz-object-lock-mode", "GOVERNANCE")
		h.Set("x-amz-object-lock-retain-until-date", expired)
		h.Set("x-amz-server-side-encryption", "aws:kms")
		h.Set("x-amz-server-side-encryption-aws-kms-key-id", testKeyARN)
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		q := r.URL.Query()
		switch {
		case r.Method == http.MethodGet && q.Has("versioning"):
			_, _ = io.WriteString(w, `<VersioningConfiguration><Status>Enabled</Status></VersioningConfiguration>`)
		case r.Method == http.MethodGet && q.Has("object-lock"):
			_, _ = io.WriteString(w, `<ObjectLockConfiguration><ObjectLockEnabled>Enabled</ObjectLockEnabled></ObjectLockConfiguration>`)
		case r.Method == http.MethodPut && (q.Has("retention") || q.Has("legal-hold")):
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut:
			w.Header().Set("x-amz-version-id", "v1")
			w.Header().Set("x-amz-checksum-sha256", r.Header.Get("x-amz-checksum-sha256"))
		case r.Method == http.MethodHead:
			version(w)
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		case r.Method == http.MethodGet:
			version(w)
			w.Header().Set("Content-Length", strconv.Itoa(len(body)))
			_, _ = w.Write(body)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			t.Errorf("fake S3: unexpected %s %s", r.Method, r.URL)
			w.WriteHeader(http.StatusNotImplemented)
		}
	}))
}

func httpConfig(server *httptest.Server, probe *bypassProbe) Config {
	cfg := unitConfig()
	cfg.Endpoint = server.URL
	cfg.InsecureForDevelopmentOnly = true
	cfg.UsePathStyle = true
	cfg.AWS.Credentials = aws.CredentialsProviderFunc(func(context.Context) (aws.Credentials, error) {
		return aws.Credentials{AccessKeyID: "test", SecretAccessKey: "test"}, nil
	})
	cfg.AWS.APIOptions = append(cfg.AWS.APIOptions, probe.option)
	return cfg
}

// TestStoreNeverSendsTheGovernanceBypass drives every operation of Store and
// Ready through the real SDK client and proves none sends the bypass header.
func TestStoreNeverSendsTheGovernanceBypass(t *testing.T) {
	body := []byte("evidence")
	server := fakeS3(t, body)
	defer server.Close()
	probe := &bypassProbe{seen: map[string]bool{}}
	store, err := New(httpConfig(server, probe))
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	ctx := tenantCtx(t, "acme")
	if err := store.Ready(ctx); err != nil {
		t.Fatalf("Ready() = %v", err)
	}
	digest, _ := digestOf(body)
	if _, err := store.Put(ctx, ports.PutRequest{Key: "doc", Body: body, ContentSHA256: digest,
		Retention: &ports.Retention{Mode: ports.RetentionGovernance, Until: wholeNow() + unit}}); err != nil {
		t.Fatalf("Put() = %v", err)
	}
	ref := ports.ObjectRef{Key: "doc", Version: "v1"}
	if _, err := store.Get(ctx, ref); err != nil {
		t.Fatalf("Get() = %v", err)
	}
	if _, err := store.Retain(ctx, ref, ports.Retention{Mode: ports.RetentionCompliance, Until: wholeNow() + unit}); err != nil {
		t.Fatalf("Retain() = %v", err)
	}
	if _, err := store.SetLegalHold(ctx, ref, ports.LegalHold{On: true, Reason: "case"}); err != nil {
		t.Fatalf("SetLegalHold() = %v", err)
	}
	if _, err := store.Purge(ctx, ref); err != nil {
		t.Fatalf("Purge() = %v", err)
	}
	want := []string{"GetBucketVersioning", "GetObjectLockConfiguration", "PutObject", "GetObject", "HeadObject", "PutObjectRetention", "PutObjectLegalHold", "DeleteObject"}
	for _, op := range want {
		sent, seen := probe.seen[op]
		if !seen {
			t.Errorf("%s never went through the stack: the probe proves nothing about it", op)
		}
		if sent {
			t.Errorf("%s sent %s: Store must never bypass GOVERNANCE (DAT-11)", op, bypassHeader)
		}
	}
}

// TestAServerFaultLeavesThePutOutcomeUnknown drives a real 500 through the SDK,
// whose deserializer leaves the fault unknown: the write may be stored, so the
// caller must learn its outcome is unknown, not that S3 refused it.
func TestAServerFaultLeavesThePutOutcomeUnknown(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.Header().Set("Content-Type", "application/xml")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `<Error><Code>InternalError</Code><Message>We encountered an internal error.</Message></Error>`)
	}))
	defer server.Close()
	store, err := New(httpConfig(server, &bypassProbe{seen: map[string]bool{}}))
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	store.ready.Store(true)
	body := []byte("evidence")
	digest, _ := digestOf(body)
	_, err = store.Put(tenantCtx(t, "acme"), ports.PutRequest{Key: "doc", Body: body, ContentSHA256: digest})
	if !errors.Is(err, ErrPutOutcomeUnknown) {
		t.Fatalf("Put() against a 500 = %v, want ErrPutOutcomeUnknown", err)
	}
}
