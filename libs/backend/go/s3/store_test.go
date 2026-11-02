package s3

import (
	"context"
	"errors"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
	smithyhttp "github.com/aws/smithy-go/transport/http"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/worm"
)

const unit = ports.Instant(time.Second)

func wholeNow() ports.Instant { return nowInstant() - nowInstant()%unit }

func putOK(in *awss3.PutObjectInput) (*awss3.PutObjectOutput, error) {
	return &awss3.PutObjectOutput{VersionId: aws.String("v1"), ChecksumSHA256: in.ChecksumSHA256}, nil
}

func TestPutWritesTheVersionAsDeclared(t *testing.T) {
	api := &fakeAPI{putObject: putOK}
	store := readyStore(t, unitConfig(), api)
	body := []byte("evidence")
	digest, sum := digestOf(body)
	until := wholeNow() + 3*unit
	got, err := store.Put(tenantCtx(t, "acme"), ports.PutRequest{
		Key: "doc", Body: body, ContentSHA256: digest, ContentType: "application/x-test",
		Retention: &ports.Retention{Mode: ports.RetentionCompliance, Until: until + 999},
	})
	if err != nil {
		t.Fatalf("Put() = %v", err)
	}
	want := ports.ObjectVersion{Key: "doc", Version: "v1", ContentSHA256: digest, Size: 8, StoredAt: nowInstant()}
	if got != want {
		t.Fatalf("Put() = %+v, want %+v", got, want)
	}
	in := api.puts[0]
	switch {
	case aws.ToString(in.Key) != "tenant/acme/doc":
		t.Fatalf("physical key = %q", aws.ToString(in.Key))
	case aws.ToString(in.ChecksumSHA256) != worm.Checksum(sum):
		t.Fatalf("checksum = %q", aws.ToString(in.ChecksumSHA256))
	case in.ServerSideEncryption != types.ServerSideEncryptionAwsKms || aws.ToString(in.SSEKMSKeyId) != testKeyARN:
		t.Fatalf("encryption = %q %q", in.ServerSideEncryption, aws.ToString(in.SSEKMSKeyId))
	case in.ObjectLockMode != types.ObjectLockModeCompliance || !in.ObjectLockRetainUntilDate.Equal(*worm.RetainUntil(until)):
		t.Fatalf("retention = %q %v, want the whole second", in.ObjectLockMode, in.ObjectLockRetainUntilDate)
	case in.Metadata[worm.StoredAtKey] == "" || aws.ToString(in.ContentType) != "application/x-test":
		t.Fatalf("metadata = %v, content type %q", in.Metadata, aws.ToString(in.ContentType))
	}
}

func TestPutRefusesLocallyWithoutIO(t *testing.T) {
	body := []byte("evidence")
	digest, _ := digestOf(body)
	ceilingLimit := nowInstant() + ports.Instant(400*24*time.Hour)
	cases := map[string]struct {
		ctx  func(*testing.T) context.Context
		req  ports.PutRequest
		want error
	}{
		"invalid key":       {func(t *testing.T) context.Context { return tenantCtx(t, "acme") }, ports.PutRequest{Key: "/doc", Body: body, ContentSHA256: digest}, ports.ErrObjectKeyInvalid},
		"no tenant":         {func(t *testing.T) context.Context { return tenantCtx(t, "") }, ports.PutRequest{Key: "doc", Body: body, ContentSHA256: digest}, ErrTenantUnresolved},
		"divergent hash":    {func(t *testing.T) context.Context { return tenantCtx(t, "acme") }, ports.PutRequest{Key: "doc", Body: body, ContentSHA256: strings.Repeat("0", 64)}, ports.ErrIntegrity},
		"expired retention": {func(t *testing.T) context.Context { return tenantCtx(t, "acme") }, ports.PutRequest{Key: "doc", Body: body, ContentSHA256: digest, Retention: &ports.Retention{Mode: ports.RetentionCompliance, Until: wholeNow()}}, ports.ErrRetentionUnbounded},
		"above the ceiling": {func(t *testing.T) context.Context { return tenantCtx(t, "acme") }, ports.PutRequest{Key: "doc", Body: body, ContentSHA256: digest, Retention: &ports.Retention{Mode: ports.RetentionCompliance, Until: ceilingLimit + 2*unit}}, ports.ErrRetentionAboveCeiling},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{}
			store := readyStore(t, unitConfig(), api)
			got, err := store.Put(tc.ctx(t), tc.req)
			if !errors.Is(err, tc.want) || got != (ports.ObjectVersion{}) {
				t.Fatalf("Put() = %+v, %v; want the zero value and %v", got, err, tc.want)
			}
			if calls := api.called(); len(calls) != 0 {
				t.Fatalf("calls = %v, want none", calls)
			}
		})
	}
}

func TestPutWithoutCeilingRefusesRetention(t *testing.T) {
	cfg := unitConfig()
	cfg.Declaration.Ceiling = nil
	store := readyStore(t, cfg, &fakeAPI{})
	body := []byte("evidence")
	digest, _ := digestOf(body)
	_, err := store.Put(tenantCtx(t, "acme"), ports.PutRequest{Key: "doc", Body: body, ContentSHA256: digest, Retention: &ports.Retention{Mode: ports.RetentionGovernance, Until: wholeNow() + unit}})
	if !errors.Is(err, ports.ErrRetentionCeilingUndeclared) {
		t.Fatalf("Put() = %v, want ErrRetentionCeilingUndeclared", err)
	}
}

func TestPutFailsClosedOnAnUnexpectedAnswer(t *testing.T) {
	cases := map[string]struct {
		answer func(*awss3.PutObjectInput) (*awss3.PutObjectOutput, error)
		want   error
	}{
		"no version id": {func(in *awss3.PutObjectInput) (*awss3.PutObjectOutput, error) {
			return &awss3.PutObjectOutput{ChecksumSHA256: in.ChecksumSHA256}, nil
		}, ErrPutOutcomeUnknown},
		"another checksum": {func(*awss3.PutObjectInput) (*awss3.PutObjectOutput, error) {
			return &awss3.PutObjectOutput{VersionId: aws.String("v1"), ChecksumSHA256: aws.String("AAAA")}, nil
		}, ErrPutOutcomeUnknown},
		"no checksum": {func(*awss3.PutObjectInput) (*awss3.PutObjectOutput, error) {
			return &awss3.PutObjectOutput{VersionId: aws.String("v1")}, nil
		}, ErrPutOutcomeUnknown},
		"bad digest": {func(*awss3.PutObjectInput) (*awss3.PutObjectOutput, error) {
			return nil, &smithy.GenericAPIError{Code: "BadDigest", Fault: smithy.FaultClient}
		}, ports.ErrIntegrity},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			store := readyStore(t, unitConfig(), &fakeAPI{putObject: tc.answer})
			body := []byte("evidence")
			digest, _ := digestOf(body)
			got, err := store.Put(tenantCtx(t, "acme"), ports.PutRequest{Key: "doc", Body: body, ContentSHA256: digest})
			if !errors.Is(err, tc.want) || got != (ports.ObjectVersion{}) {
				t.Fatalf("Put() = %+v, %v; want %v", got, err, tc.want)
			}
		})
	}
}

func TestPutOutcome(t *testing.T) {
	dial := &net.OpError{Op: "dial", Err: errors.New("connection refused")}
	cases := map[string]struct {
		err     error
		unknown bool
	}{
		"refused by s3":     {&smithy.GenericAPIError{Code: "AccessDenied", Fault: smithy.FaultClient}, false},
		"never connected":   {dial, false},
		"breaker open":      {resilience.ErrBreakerOpen, false},
		"server 500":        {answer(http.StatusInternalServerError, "InternalError"), true},
		"proxy 502":         {answer(http.StatusBadGateway, "BadGateway"), true},
		"refused with 403":  {answer(http.StatusForbidden, "AccessDenied"), false},
		"deadline exceeded": {context.DeadlineExceeded, true},
		"connection reset":  {&net.OpError{Op: "read", Err: errors.New("reset")}, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if got := errors.Is(putOutcome(tc.err), ErrPutOutcomeUnknown); got != tc.unknown {
				t.Fatalf("putOutcome(%v) unknown = %v, want %v", tc.err, got, tc.unknown)
			}
		})
	}
}

func TestPutIsNeverRepeatedOnceSent(t *testing.T) {
	cfg := unitConfig()
	cfg.Sheet = resilience.Defaults("s3")
	cfg.Clock = unitConfig().Clock
	api := &fakeAPI{putObject: func(*awss3.PutObjectInput) (*awss3.PutObjectOutput, error) {
		return nil, answer(http.StatusInternalServerError, "InternalError")
	}}
	store := readyStore(t, cfg, api)
	body := []byte("evidence")
	digest, _ := digestOf(body)
	_, err := store.Put(tenantCtx(t, "acme"), ports.PutRequest{Key: "doc", Body: body, ContentSHA256: digest})
	if !errors.Is(err, ErrPutOutcomeUnknown) {
		t.Fatalf("Put() = %v, want ErrPutOutcomeUnknown", err)
	}
	if calls := api.called(); len(calls) != 1 {
		t.Fatalf("calls = %v, want one PutObject: a sent write is never repeated", calls)
	}
}

func TestHeadReportsTheStoredVersion(t *testing.T) {
	body := []byte("evidence")
	retention := &ports.Retention{Mode: ports.RetentionGovernance, Until: wholeNow() + 2*unit}
	api := &fakeAPI{headObject: func(in *awss3.HeadObjectInput) (*awss3.HeadObjectOutput, error) {
		if aws.ToString(in.VersionId) != "v1" || in.ChecksumMode != types.ChecksumModeEnabled || aws.ToString(in.Key) != "platform/doc" {
			t.Errorf("HeadObject(%+v): want the version, the checksum and the platform key", in)
		}
		return storedHead(body, retention, true), nil
	}}
	cfg := unitConfig()
	cfg.Declaration.Scope = ports.ScopePlatform
	store := readyStore(t, cfg, api)
	info, err := store.Head(tenantCtx(t, ""), ports.ObjectRef{Key: "doc", Version: "v1"})
	digest, _ := digestOf(body)
	if err != nil || info.ContentSHA256 != digest || info.StoredAt != nowInstant() || !info.LegalHold || *info.Retention != *retention || info.Encryption != cfg.Declaration.Encryption {
		t.Fatalf("Head() = %+v, %v", info, err)
	}
}

func TestReadsOfAnAbsentVersionAreNotFound(t *testing.T) {
	for _, code := range []string{"NoSuchKey", "NoSuchVersion", "NotFound"} {
		t.Run(code, func(t *testing.T) {
			absent := &smithy.GenericAPIError{Code: code, Fault: smithy.FaultClient}
			api := &fakeAPI{
				headObject: func(*awss3.HeadObjectInput) (*awss3.HeadObjectOutput, error) { return nil, absent },
				getObject:  func(*awss3.GetObjectInput) (*awss3.GetObjectOutput, error) { return nil, absent },
				putObjectLegalHold: func(*awss3.PutObjectLegalHoldInput) (*awss3.PutObjectLegalHoldOutput, error) {
					return nil, absent
				},
			}
			store := readyStore(t, unitConfig(), api)
			ctx, ref := tenantCtx(t, "acme"), ports.ObjectRef{Key: "doc", Version: "v1"}
			if obj, err := store.Get(ctx, ref); !errors.Is(err, ports.ErrNotFound) || len(obj.Body) != 0 {
				t.Fatalf("Get() = %v", err)
			}
			if info, err := store.Head(ctx, ref); !errors.Is(err, ports.ErrObjectNotFound) || info != (ports.ObjectInfo{}) {
				t.Fatalf("Head() = %v", err)
			}
			if _, err := store.Retain(ctx, ref, ports.Retention{Mode: ports.RetentionCompliance, Until: wholeNow() + unit}); !errors.Is(err, ports.ErrObjectNotFound) {
				t.Fatalf("Retain() = %v", err)
			}
			if _, err := store.SetLegalHold(ctx, ref, ports.LegalHold{On: true, Reason: "case"}); !errors.Is(err, ports.ErrObjectNotFound) {
				t.Fatalf("SetLegalHold() = %v", err)
			}
			if _, err := store.Purge(ctx, ref); !errors.Is(err, ports.ErrObjectNotFound) {
				t.Fatalf("Purge() = %v", err)
			}
		})
	}
}

func TestGetRefusesABodyThatDoesNotMatchTheChecksum(t *testing.T) {
	api := &fakeAPI{getObject: func(*awss3.GetObjectInput) (*awss3.GetObjectOutput, error) {
		out := storedGet([]byte("evidence"))
		out.Body = storedGet([]byte("tampered")).Body
		return out, nil
	}}
	store := readyStore(t, unitConfig(), api)
	obj, err := store.Get(tenantCtx(t, "acme"), ports.ObjectRef{Key: "doc", Version: "v1"})
	if !errors.Is(err, ports.ErrIntegrity) || len(obj.Body) != 0 {
		t.Fatalf("Get() = %v, want ErrIntegrity", err)
	}
	ok := readyStore(t, unitConfig(), &fakeAPI{getObject: func(*awss3.GetObjectInput) (*awss3.GetObjectOutput, error) {
		return storedGet([]byte("evidence")), nil
	}})
	obj, err = ok.Get(tenantCtx(t, "acme"), ports.ObjectRef{Key: "doc", Version: "v1"})
	if err != nil || string(obj.Body) != "evidence" {
		t.Fatalf("Get() = %q, %v", obj.Body, err)
	}
}

func TestRetainAppliesTheRuleThenSendsNoBypass(t *testing.T) {
	current := &ports.Retention{Mode: ports.RetentionGovernance, Until: wholeNow() + 2*unit}
	api := &fakeAPI{
		headObject: func(*awss3.HeadObjectInput) (*awss3.HeadObjectOutput, error) {
			return storedHead([]byte("evidence"), current, false), nil
		},
		putObjectRetention: func(*awss3.PutObjectRetentionInput) (*awss3.PutObjectRetentionOutput, error) {
			return &awss3.PutObjectRetentionOutput{}, nil
		},
	}
	store := readyStore(t, unitConfig(), api)
	ctx, ref := tenantCtx(t, "acme"), ports.ObjectRef{Key: "doc", Version: "v1"}
	if _, err := store.Retain(ctx, ref, ports.Retention{Mode: ports.RetentionGovernance, Until: current.Until - unit}); !errors.Is(err, ports.ErrRetentionNotExtended) {
		t.Fatalf("Retain(shorter) = %v, want ErrRetentionNotExtended", err)
	}
	promoted := ports.Retention{Mode: ports.RetentionCompliance, Until: current.Until}
	info, err := store.Retain(ctx, ref, promoted)
	if err != nil || *info.Retention != promoted {
		t.Fatalf("Retain(promote) = %+v, %v", info.Retention, err)
	}
	if len(api.retentions) != 1 {
		t.Fatalf("PutObjectRetention calls = %d, want only the accepted one", len(api.retentions))
	}
	in := api.retentions[0]
	if in.BypassGovernanceRetention != nil || in.Retention.Mode != types.ObjectLockRetentionModeCompliance || aws.ToString(in.VersionId) != "v1" {
		t.Fatalf("PutObjectRetention(%+v): want COMPLIANCE on the version and no bypass", in)
	}
}

func TestSetLegalHold(t *testing.T) {
	var sent types.ObjectLockLegalHoldStatus
	api := &fakeAPI{putObjectLegalHold: func(in *awss3.PutObjectLegalHoldInput) (*awss3.PutObjectLegalHoldOutput, error) {
		sent = in.LegalHold.Status
		return &awss3.PutObjectLegalHoldOutput{}, nil
	}}
	store := readyStore(t, unitConfig(), api)
	ctx, ref := tenantCtx(t, "acme"), ports.ObjectRef{Key: "doc", Version: "v1"}
	if _, err := store.SetLegalHold(ctx, ref, ports.LegalHold{On: true}); !errors.Is(err, ports.ErrLegalHoldReasonRequired) {
		t.Fatalf("SetLegalHold(no reason) = %v", err)
	}
	receipt, err := store.SetLegalHold(ctx, ref, ports.LegalHold{On: true, Reason: "case 42"})
	want := ports.HoldReceipt{Ref: ref, On: true, Reason: "case 42", At: nowInstant()}
	if err != nil || receipt != want || sent != types.ObjectLockLegalHoldStatusOn {
		t.Fatalf("SetLegalHold() = %+v, %v (sent %q)", receipt, err, sent)
	}
}

func TestPurgeRefusesALockedVersionAndSendsNoBypass(t *testing.T) {
	retention := &ports.Retention{Mode: ports.RetentionCompliance, Until: wholeNow() - unit}
	hold, retained := false, retention
	api := &fakeAPI{
		headObject: func(*awss3.HeadObjectInput) (*awss3.HeadObjectOutput, error) {
			return storedHead([]byte("evidence"), retained, hold), nil
		},
		deleteObject: func(*awss3.DeleteObjectInput) (*awss3.DeleteObjectOutput, error) {
			return &awss3.DeleteObjectOutput{}, nil
		},
	}
	store := readyStore(t, unitConfig(), api)
	ctx, ref := tenantCtx(t, "acme"), ports.ObjectRef{Key: "doc", Version: "v1"}
	hold = true
	if _, err := store.Purge(ctx, ref); !errors.Is(err, ports.ErrObjectLocked) {
		t.Fatalf("Purge(held) = %v", err)
	}
	hold, retained = false, &ports.Retention{Mode: ports.RetentionGovernance, Until: wholeNow() + unit}
	if _, err := store.Purge(ctx, ref); !errors.Is(err, ports.ErrObjectLocked) {
		t.Fatalf("Purge(retained) = %v", err)
	}
	retained = retention
	receipt, err := store.Purge(ctx, ref)
	if err != nil || receipt != (ports.PurgeReceipt{Ref: ref, PurgedAt: nowInstant(), RetainedUntil: retention.Until}) {
		t.Fatalf("Purge() = %+v, %v", receipt, err)
	}
	if len(api.deletes) != 1 || api.deletes[0].BypassGovernanceRetention != nil || aws.ToString(api.deletes[0].VersionId) != "v1" {
		t.Fatalf("DeleteObject calls = %+v: want one, by version, without bypass", api.deletes)
	}
}

// answer is an S3 API error as the SDK delivers it: the deserializer leaves the
// fault unknown, and only the response carries the status.
func answer(status int, code string) error {
	return &smithyhttp.ResponseError{
		Response: &smithyhttp.Response{Response: &http.Response{StatusCode: status}},
		Err:      &smithy.GenericAPIError{Code: code, Message: "tenant/acme/doc"},
	}
}

func TestErrorsReachTheCallerWithoutTheURL(t *testing.T) {
	cfg := unitConfig()
	cfg.Endpoint, cfg.InsecureForDevelopmentOnly, cfg.UsePathStyle = "http://127.0.0.1:1", true, true
	cfg.AWS.Credentials = aws.AnonymousCredentials{}
	store, err := New(cfg)
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	err = store.Ready(context.Background())
	if err == nil || strings.Contains(err.Error(), "127.0.0.1") || strings.Contains(err.Error(), "evidence") {
		t.Fatalf("Ready() = %q, want a failure without the URL", err)
	}
	var network net.Error
	if !errors.As(err, &network) {
		t.Fatalf("Ready() = %v, want it to unwrap to the network error", err)
	}
	store.ready.Store(true)
	body := []byte("evidence")
	digest, _ := digestOf(body)
	_, err = store.Put(tenantCtx(t, "acme"), ports.PutRequest{Key: "secret-key", Body: body, ContentSHA256: digest})
	if err == nil || strings.Contains(err.Error(), "secret-key") || strings.Contains(err.Error(), "tenant/acme") {
		t.Fatalf("Put() = %q, want a failure without the physical key", err)
	}
}

func TestConfigRequiresTheKMSKeyARN(t *testing.T) {
	cfg := validConfig()
	cfg.Declaration.Encryption.KeyRef = "alias/evidence"
	if err := cfg.Validate(); !errors.Is(err, ErrIncompleteConfig) {
		t.Fatalf("Validate(alias) = %v, want ErrIncompleteConfig", err)
	}
	cfg.Declaration.Encryption = ports.Encryption{Algorithm: "AES256"}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate(AES256) = %v, want nil", err)
	}
}
