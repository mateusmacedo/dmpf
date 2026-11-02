package admin

import (
	"context"
	"crypto/sha256"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/worm"
)

const second = ports.Instant(time.Second)

var testNow = time.Unix(1_760_000_000, 0)

func now() ports.Instant { return ports.Instant(testNow.UnixNano()) }

func tenantCtx(t *testing.T, tenant ports.TenantID) context.Context {
	t.Helper()
	spec := ports.ExecutionContextSpec{
		RequestID: "r-test", CorrelationID: "c-test", TraceContext: "t-test",
		Deadline: ports.Instant(1_755_432_000_000_000_000), Locale: "en",
	}
	if tenant != "" {
		spec.Tenant = &tenant
	}
	execution, err := ports.NewExecutionContext(spec)
	if err != nil {
		t.Fatalf("NewExecutionContext: %v", err)
	}
	return ports.WithExecutionContext(context.Background(), execution)
}

func validConfig() Config {
	sheet := resilience.Defaults("s3-admin")
	sheet.Retry = resilience.Declare(false)
	return Config{AWS: aws.Config{Region: "us-east-1"}, Bucket: "evidence", Scope: ports.ScopeTenant, Clock: clock.NewFake(testNow), Sheet: sheet}
}

func head(storedAt ports.Instant, retention *ports.Retention, hold bool) *awss3.HeadObjectOutput {
	sum := sha256.Sum256([]byte("evidence"))
	out := &awss3.HeadObjectOutput{
		VersionId: aws.String("v1"), ChecksumSHA256: aws.String(worm.Checksum(sum)), ContentLength: aws.Int64(8),
		Metadata: worm.StoredAt(storedAt), ServerSideEncryption: types.ServerSideEncryptionAes256,
	}
	out.LastModified = aws.Time(time.Unix(0, int64(storedAt-storedAt%second)).UTC())
	if retention != nil {
		out.ObjectLockMode = types.ObjectLockMode(retention.Mode)
		out.ObjectLockRetainUntilDate = worm.RetainUntil(retention.Until)
	}
	if hold {
		out.ObjectLockLegalHoldStatus = types.ObjectLockLegalHoldStatusOn
	}
	return out
}

type fakeAPI struct {
	t          *testing.T
	head       *awss3.HeadObjectOutput
	headErr    error
	retentions []*awss3.PutObjectRetentionInput
	deletes    []*awss3.DeleteObjectInput
}

func (f *fakeAPI) HeadObject(context.Context, *awss3.HeadObjectInput, ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error) {
	return f.head, f.headErr
}

func (f *fakeAPI) PutObjectRetention(_ context.Context, in *awss3.PutObjectRetentionInput, _ ...func(*awss3.Options)) (*awss3.PutObjectRetentionOutput, error) {
	f.retentions = append(f.retentions, in)
	return &awss3.PutObjectRetentionOutput{}, nil
}

func (f *fakeAPI) DeleteObject(_ context.Context, in *awss3.DeleteObjectInput, _ ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error) {
	f.deletes = append(f.deletes, in)
	return &awss3.DeleteObjectOutput{}, nil
}

func newAdmin(t *testing.T, api *fakeAPI) *Admin {
	t.Helper()
	api.t = t
	a, err := build(validConfig(), api)
	if err != nil {
		t.Fatalf("build() = %v", err)
	}
	return a
}

var ref = ports.ObjectRef{Key: "doc", Version: "v1"}

func TestConfigValidate(t *testing.T) {
	cases := map[string]struct {
		mutate func(*Config)
		want   error
	}{
		"complete":            {func(*Config) {}, nil},
		"without region":      {func(c *Config) { c.AWS.Region = "" }, ErrIncompleteConfig},
		"without bucket":      {func(c *Config) { c.Bucket = "" }, ErrIncompleteConfig},
		"without scope":       {func(c *Config) { c.Scope = "" }, ports.ErrScopeUndeclared},
		"plaintext endpoint":  {func(c *Config) { c.Endpoint = "http://localhost:4566" }, ErrTLSRequired},
		"path style, no host": {func(c *Config) { c.UsePathStyle = true }, ErrIncompleteConfig},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			cfg := validConfig()
			tc.mutate(&cfg)
			err := cfg.Validate()
			if (tc.want == nil && err != nil) || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("Validate() = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestShortenBringsGovernanceToTheNewCeiling(t *testing.T) {
	stored := now() - 10*second
	current := ports.Retention{Mode: ports.RetentionGovernance, Until: now() + 100*second}
	api := &fakeAPI{head: head(stored, &current, false)}
	ceiling := ports.RetentionCeiling{Span: ports.RetentionSpan(30 * time.Second), Origin: "lgpd-ruling-7"}
	receipt, err := newAdmin(t, api).Shorten(tenantCtx(t, "acme"), ref, ceiling)
	want := RetentionReceipt{Ref: ref, From: current, To: ports.Retention{Mode: ports.RetentionGovernance, Until: stored + 30*second}, Reason: "lgpd-ruling-7", At: now()}
	if err != nil || receipt != want {
		t.Fatalf("Shorten() = %+v, %v; want %+v", receipt, err, want)
	}
	in := api.retentions[0]
	if !aws.ToBool(in.BypassGovernanceRetention) || aws.ToString(in.Key) != "tenant/acme/doc" || !in.Retention.RetainUntilDate.Equal(*worm.RetainUntil(stored + 30*second)) {
		t.Fatalf("PutObjectRetention(%+v): want the bypass on the physical key and the new date", in)
	}
}

func TestShortenRefuses(t *testing.T) {
	stored := now() - 10*second
	ceiling := ports.RetentionCeiling{Span: ports.RetentionSpan(30 * time.Second), Origin: "ruling"}
	cases := map[string]struct {
		head    *awss3.HeadObjectOutput
		ceiling ports.RetentionCeiling
		want    error
	}{
		"compliance in force":      {head(stored, &ports.Retention{Mode: ports.RetentionCompliance, Until: now() + 100*second}, false), ceiling, ErrComplianceImmutable},
		"already within the limit": {head(stored, &ports.Retention{Mode: ports.RetentionGovernance, Until: now() + 5*second}, false), ceiling, ErrWithinCeiling},
		"free version":             {head(stored, nil, false), ceiling, ErrWithinCeiling},
		"outlived the ceiling":     {head(now()-60*second, &ports.Retention{Mode: ports.RetentionGovernance, Until: now() + 100*second}, false), ceiling, ErrOutlivedCeiling},
		"ceiling without origin":   {head(stored, nil, false), ports.RetentionCeiling{Span: ceiling.Span}, ports.ErrRetentionCeilingUndeclared},
		"ceiling without span":     {head(stored, nil, false), ports.RetentionCeiling{Origin: "ruling"}, ports.ErrRetentionCeilingUndeclared},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{head: tc.head}
			receipt, err := newAdmin(t, api).Shorten(tenantCtx(t, "acme"), ref, tc.ceiling)
			if !errors.Is(err, tc.want) || receipt != (RetentionReceipt{}) || len(api.retentions) != 0 {
				t.Fatalf("Shorten() = %+v, %v (calls %d); want %v and no call", receipt, err, len(api.retentions), tc.want)
			}
		})
	}
}

func TestPurge(t *testing.T) {
	ceiling := ports.RetentionCeiling{Span: ports.RetentionSpan(30 * time.Second), Origin: "ruling"}
	governance := &ports.Retention{Mode: ports.RetentionGovernance, Until: now() + 100*second}
	cases := map[string]struct {
		head   *awss3.HeadObjectOutput
		want   error
		bypass bool
	}{
		"outlived governance": {head(now()-60*second, governance, false), nil, true},
		"within the ceiling":  {head(now()-10*second, governance, false), ErrWithinCeiling, false},
		"compliance":          {head(now()-60*second, &ports.Retention{Mode: ports.RetentionCompliance, Until: now() + 100*second}, false), ErrComplianceImmutable, false},
		"legal hold":          {head(now()-60*second, governance, true), ports.ErrObjectLocked, false},
		"expired, no bypass":  {head(now()-60*second, &ports.Retention{Mode: ports.RetentionGovernance, Until: now() - second}, false), nil, false},
		"free, outlived":      {head(now()-60*second, nil, false), nil, false},
		"free, within":        {head(now()-10*second, nil, false), ErrWithinCeiling, false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			api := &fakeAPI{head: tc.head}
			receipt, err := newAdmin(t, api).Purge(tenantCtx(t, "acme"), ref, ceiling)
			if (tc.want == nil && err != nil) || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("Purge() = %v, want %v", err, tc.want)
			}
			if tc.want != nil {
				if len(api.deletes) != 0 || receipt != (PurgeReceipt{}) {
					t.Fatalf("Purge() refused but deleted %d or returned %+v", len(api.deletes), receipt)
				}
				return
			}
			if len(api.deletes) != 1 || aws.ToBool(api.deletes[0].BypassGovernanceRetention) != tc.bypass {
				t.Fatalf("DeleteObject = %+v, want one with bypass %v", api.deletes, tc.bypass)
			}
			if receipt.Ref != ref || receipt.PurgedAt != now() || receipt.Reason != "ruling" {
				t.Fatalf("Purge() = %+v", receipt)
			}
		})
	}
}

func TestAbsentVersionIsNotFound(t *testing.T) {
	api := &fakeAPI{headErr: &smithy.GenericAPIError{Code: "NotFound", Fault: smithy.FaultClient}}
	ceiling := ports.RetentionCeiling{Span: ports.RetentionSpan(time.Second), Origin: "ruling"}
	if _, err := newAdmin(t, api).Purge(tenantCtx(t, "acme"), ref, ceiling); !errors.Is(err, ports.ErrObjectNotFound) {
		t.Fatalf("Purge() = %v, want ErrObjectNotFound", err)
	}
}

func TestRefusesWithoutTheCarrierTheScopeNeeds(t *testing.T) {
	ceiling := ports.RetentionCeiling{Span: ports.RetentionSpan(time.Second), Origin: "ruling"}
	if _, err := newAdmin(t, &fakeAPI{}).Shorten(tenantCtx(t, ""), ref, ceiling); !errors.Is(err, worm.ErrTenantUnresolved) {
		t.Fatalf("Shorten(no tenant) = %v, want ErrTenantUnresolved", err)
	}
	if _, err := newAdmin(t, &fakeAPI{}).Purge(tenantCtx(t, "acme"), ports.ObjectRef{Key: "doc"}, ceiling); !errors.Is(err, ports.ErrVersionRequired) {
		t.Fatalf("Purge(no version) = %v, want ErrVersionRequired", err)
	}
}

// TestTheLimitCountsFromTheLaterWrite proves a forged or skewed dmpf-stored-at
// cannot bring the limit forward: the server's LastModified wins when later.
func TestTheLimitCountsFromTheLaterWrite(t *testing.T) {
	out := head(now()-60*second, &ports.Retention{Mode: ports.RetentionGovernance, Until: now() + 100*second}, false)
	out.LastModified = aws.Time(testNow.Add(-10 * time.Second).UTC())
	api := &fakeAPI{head: out}
	ceiling := ports.RetentionCeiling{Span: ports.RetentionSpan(30 * time.Second), Origin: "ruling"}
	if _, err := newAdmin(t, api).Purge(tenantCtx(t, "acme"), ref, ceiling); !errors.Is(err, ErrWithinCeiling) {
		t.Fatalf("Purge() = %v, want ErrWithinCeiling: the server wrote it 10 s ago", err)
	}
	receipt, err := newAdmin(t, api).Shorten(tenantCtx(t, "acme"), ref, ceiling)
	if err != nil || receipt.To.Until != now()+20*second {
		t.Fatalf("Shorten() = %+v, %v; want the limit 30 s after LastModified", receipt, err)
	}
}
