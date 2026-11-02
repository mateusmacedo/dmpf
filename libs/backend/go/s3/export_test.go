package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/worm"
)

// fakeAPI answers as an S3 bucket with versions and Object Lock would; every
// method records its input, and one left nil fails the test if called.
type fakeAPI struct {
	t  *testing.T
	mu sync.Mutex

	putObject          func(*awss3.PutObjectInput) (*awss3.PutObjectOutput, error)
	getObject          func(*awss3.GetObjectInput) (*awss3.GetObjectOutput, error)
	headObject         func(*awss3.HeadObjectInput) (*awss3.HeadObjectOutput, error)
	putObjectRetention func(*awss3.PutObjectRetentionInput) (*awss3.PutObjectRetentionOutput, error)
	putObjectLegalHold func(*awss3.PutObjectLegalHoldInput) (*awss3.PutObjectLegalHoldOutput, error)
	deleteObject       func(*awss3.DeleteObjectInput) (*awss3.DeleteObjectOutput, error)
	versioning         func() (*awss3.GetBucketVersioningOutput, error)
	lock               func() (*awss3.GetObjectLockConfigurationOutput, error)

	calls      []string
	puts       []*awss3.PutObjectInput
	retentions []*awss3.PutObjectRetentionInput
	deletes    []*awss3.DeleteObjectInput
}

func (f *fakeAPI) record(name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, name)
}

func (f *fakeAPI) unexpected(name string) {
	f.t.Helper()
	f.t.Fatalf("unexpected %s: the store must refuse before any I/O", name)
}

func (f *fakeAPI) PutObject(_ context.Context, in *awss3.PutObjectInput, _ ...func(*awss3.Options)) (*awss3.PutObjectOutput, error) {
	f.record("PutObject")
	if f.putObject == nil {
		f.unexpected("PutObject")
	}
	f.mu.Lock()
	f.puts = append(f.puts, in)
	f.mu.Unlock()
	return f.putObject(in)
}

func (f *fakeAPI) GetObject(_ context.Context, in *awss3.GetObjectInput, _ ...func(*awss3.Options)) (*awss3.GetObjectOutput, error) {
	f.record("GetObject")
	if f.getObject == nil {
		f.unexpected("GetObject")
	}
	return f.getObject(in)
}

func (f *fakeAPI) HeadObject(_ context.Context, in *awss3.HeadObjectInput, _ ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error) {
	f.record("HeadObject")
	if f.headObject == nil {
		f.unexpected("HeadObject")
	}
	return f.headObject(in)
}

func (f *fakeAPI) PutObjectRetention(_ context.Context, in *awss3.PutObjectRetentionInput, _ ...func(*awss3.Options)) (*awss3.PutObjectRetentionOutput, error) {
	f.record("PutObjectRetention")
	if f.putObjectRetention == nil {
		f.unexpected("PutObjectRetention")
	}
	f.mu.Lock()
	f.retentions = append(f.retentions, in)
	f.mu.Unlock()
	return f.putObjectRetention(in)
}

func (f *fakeAPI) PutObjectLegalHold(_ context.Context, in *awss3.PutObjectLegalHoldInput, _ ...func(*awss3.Options)) (*awss3.PutObjectLegalHoldOutput, error) {
	f.record("PutObjectLegalHold")
	if f.putObjectLegalHold == nil {
		f.unexpected("PutObjectLegalHold")
	}
	return f.putObjectLegalHold(in)
}

func (f *fakeAPI) DeleteObject(_ context.Context, in *awss3.DeleteObjectInput, _ ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error) {
	f.record("DeleteObject")
	if f.deleteObject == nil {
		f.unexpected("DeleteObject")
	}
	f.mu.Lock()
	f.deletes = append(f.deletes, in)
	f.mu.Unlock()
	return f.deleteObject(in)
}

func (f *fakeAPI) GetBucketVersioning(_ context.Context, _ *awss3.GetBucketVersioningInput, _ ...func(*awss3.Options)) (*awss3.GetBucketVersioningOutput, error) {
	f.record("GetBucketVersioning")
	if f.versioning == nil {
		return &awss3.GetBucketVersioningOutput{Status: types.BucketVersioningStatusEnabled}, nil
	}
	return f.versioning()
}

func (f *fakeAPI) GetObjectLockConfiguration(_ context.Context, _ *awss3.GetObjectLockConfigurationInput, _ ...func(*awss3.Options)) (*awss3.GetObjectLockConfigurationOutput, error) {
	f.record("GetObjectLockConfiguration")
	if f.lock == nil {
		return &awss3.GetObjectLockConfigurationOutput{ObjectLockConfiguration: &types.ObjectLockConfiguration{ObjectLockEnabled: types.ObjectLockEnabledEnabled}}, nil
	}
	return f.lock()
}

func (f *fakeAPI) called() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.calls...)
}

var testNow = time.Unix(1_760_000_000, 123_456_789)

func nowInstant() ports.Instant { return ports.Instant(testNow.UnixNano()) }

// unitConfig is validConfig with a fixed clock and the sheet's retry off, so a
// unit test sees each call once unless it turns retry on.
func unitConfig() Config {
	cfg := validConfig()
	cfg.Clock = clock.NewFake(testNow)
	cfg.Sheet.Retry = resilience.Declare(false)
	return cfg
}

func newTestStore(t *testing.T, cfg Config, api *fakeAPI) *Store {
	t.Helper()
	api.t = t
	store, err := build(cfg, api)
	if err != nil {
		t.Fatalf("build() = %v", err)
	}
	return store
}

func readyStore(t *testing.T, cfg Config, api *fakeAPI) *Store {
	t.Helper()
	store := newTestStore(t, cfg, api)
	if err := store.Ready(context.Background()); err != nil {
		t.Fatalf("Ready() = %v", err)
	}
	api.calls = nil
	return store
}

func digestOf(body []byte) (string, [32]byte) {
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:]), sum
}

func storedHead(body []byte, retention *ports.Retention, hold bool) *awss3.HeadObjectOutput {
	_, sum := digestOf(body)
	out := &awss3.HeadObjectOutput{
		VersionId:            aws.String("v1"),
		ChecksumSHA256:       aws.String(worm.Checksum(sum)),
		ContentLength:        aws.Int64(int64(len(body))),
		ContentType:          aws.String("application/x-test"),
		Metadata:             worm.StoredAt(nowInstant()),
		ServerSideEncryption: types.ServerSideEncryptionAwsKms,
		SSEKMSKeyId:          aws.String(testKeyARN),
	}
	if retention != nil {
		out.ObjectLockMode = types.ObjectLockMode(retention.Mode)
		out.ObjectLockRetainUntilDate = worm.RetainUntil(retention.Until)
	}
	if hold {
		out.ObjectLockLegalHoldStatus = types.ObjectLockLegalHoldStatusOn
	}
	return out
}

func storedGet(body []byte) *awss3.GetObjectOutput {
	head := storedHead(body, nil, false)
	return &awss3.GetObjectOutput{
		Body: io.NopCloser(bytes.NewReader(body)), VersionId: head.VersionId, ChecksumSHA256: head.ChecksumSHA256,
		ContentLength: head.ContentLength, ContentType: head.ContentType, Metadata: head.Metadata,
		ServerSideEncryption: head.ServerSideEncryption, SSEKMSKeyId: head.SSEKMSKeyId,
	}
}
