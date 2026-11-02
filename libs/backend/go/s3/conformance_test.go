//go:build integration

package s3

import (
	"context"
	"errors"
	"os"
	"slices"
	"testing"
	"time"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// defaultRetentionUnit leaves the kit's one-unit retentions room for the calls
// between the probe write and the write that dates them, against a real clock.
const defaultRetentionUnit = 5 * time.Second

func retentionUnit(t *testing.T, variable string, fallback time.Duration) time.Duration {
	t.Helper()
	raw := os.Getenv(variable)
	if raw == "" {
		return fallback
	}
	unit, err := time.ParseDuration(raw)
	if err != nil || unit < time.Second || unit%time.Second != 0 {
		t.Fatalf("%s = %q, want a whole number of seconds", variable, raw)
	}
	return unit
}

// s3Subject opens every store of one kit clause over a bucket of its own, born
// with Object Lock; the clock advances by sleeping, as the server's does.
func s3Subject(connect func(bucket string, decl ports.ObjectStoreDeclaration) Config, client *awss3.Client, keyARN string, unit time.Duration, newBucket func() string) func() providerkit.ObjectStoreSubject {
	return func() providerkit.ObjectStoreSubject {
		bucket := newBucket()
		return providerkit.ObjectStoreSubject{
			Open: func(decl ports.ObjectStoreDeclaration) (ports.ObjectStore, error) {
				store, err := New(connect(bucket, decl))
				if err != nil {
					return nil, err
				}
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				if err := store.Ready(ctx); err != nil {
					return nil, err
				}
				return store, nil
			},
			Advance: func(nanos int64) error {
				time.Sleep(time.Duration(nanos))
				return nil
			},
			TenantUnresolved: ErrTenantUnresolved,
			Encryption:       ports.Encryption{Algorithm: "aws:kms", KeyRef: keyARN},
			OtherEncryption:  ports.Encryption{Algorithm: "AES256"},
			PhysicalKeys:     func() ([]string, error) { return physicalKeys(client, bucket) },
			RetentionUnit:    ports.RetentionSpan(unit),
		}
	}
}

func physicalKeys(client *awss3.Client, bucket string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	var keys []string
	pages := awss3.NewListObjectVersionsPaginator(client, &awss3.ListObjectVersionsInput{Bucket: &bucket})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, err
		}
		for _, v := range page.Versions {
			keys = append(keys, *v.Key)
		}
	}
	slices.Sort(keys)
	return slices.Compact(keys), nil
}

func TestObjectStoreConformsOnTheEmulator(t *testing.T) {
	endpoint := emulator(t)
	client := rawS3(t, endpoint)
	unit := retentionUnit(t, "S3_KIT_RETENTION_UNIT", defaultRetentionUnit)
	started := time.Now()
	connect := func(bucket string, decl ports.ObjectStoreDeclaration) Config {
		return integrationConfig(t, endpoint, bucket, decl)
	}
	v := providerkit.ObjectStore(s3Subject(connect, client, kmsKey(t, endpoint), unit, func() string {
		return createBucket(t, client, lockedBucket)
	}))
	t.Logf("kit over the emulator: %s with a retention unit of %s", time.Since(started).Round(time.Second), unit)
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("skipped %v: every clause runs against the emulator", v.Skipped)
	}
}

func TestReadyAgainstTheEmulator(t *testing.T) {
	endpoint := emulator(t)
	client := rawS3(t, endpoint)
	decl := ports.ObjectStoreDeclaration{Scope: ports.ScopeTenant, Encryption: ports.Encryption{Algorithm: "AES256"}}
	cases := map[string]struct {
		shape bucketShape
		want  error
	}{
		"locked bucket":             {lockedBucket, nil},
		"versioned, no lock":        {versionedBucket, ErrBucketNotImmutable},
		"neither versions nor lock": {plainBucket, ErrBucketNotImmutable},
		"default retention":         {defaultRetentionBucket, ErrBucketNotImmutable},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			store, err := New(integrationConfig(t, endpoint, createBucket(t, client, tc.shape), decl))
			if err != nil {
				t.Fatalf("New() = %v", err)
			}
			err = store.Ready(context.Background())
			if (tc.want == nil && err != nil) || (tc.want != nil && !errors.Is(err, tc.want)) {
				t.Fatalf("Ready() = %v, want %v", err, tc.want)
			}
		})
	}
}
