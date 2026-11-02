//go:build integration

package s3

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/kms"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// emulator is the endpoint of the S3 under test; outside the CI an unset
// variable skips, inside it fails, so the suite never passes without a backend.
func emulator(t *testing.T) string {
	t.Helper()
	return tb.Env(t, "S3_ENDPOINT")
}

func awsConfig(t *testing.T) aws.Config {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		t.Fatalf("LoadDefaultConfig: %v", err)
	}
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return cfg
}

// rawS3 is a client outside the store, for what the store does not do:
// creating buckets, listing versions and probing the emulator.
func rawS3(t *testing.T, endpoint string) *awss3.Client {
	t.Helper()
	return awss3.NewFromConfig(awsConfig(t), func(o *awss3.Options) {
		o.BaseEndpoint = &endpoint
		o.UsePathStyle = true
	})
}

var bucketSequence atomic.Int64

func uniqueBucket(prefix string) string {
	return fmt.Sprintf("dmpf-%s-%d-%d", prefix, time.Now().UnixNano()%1_000_000_000_000, bucketSequence.Add(1))
}

// bucketShape is how a test bucket is born: with Object Lock, which S3 only
// grants at creation and which turns versioning on, or plain with or without it.
type bucketShape int

const (
	lockedBucket bucketShape = iota
	versionedBucket
	plainBucket
	defaultRetentionBucket
)

func createBucket(t *testing.T, client *awss3.Client, shape bucketShape) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := uniqueBucket("it")
	in := &awss3.CreateBucketInput{Bucket: &name}
	if shape == lockedBucket || shape == defaultRetentionBucket {
		in.ObjectLockEnabledForBucket = aws.Bool(true)
	}
	if _, err := client.CreateBucket(ctx, in); err != nil {
		t.Fatalf("CreateBucket(%s): %v", name, err)
	}
	if shape == versionedBucket {
		if _, err := client.PutBucketVersioning(ctx, &awss3.PutBucketVersioningInput{
			Bucket: &name, VersioningConfiguration: &types.VersioningConfiguration{Status: types.BucketVersioningStatusEnabled},
		}); err != nil {
			t.Fatalf("PutBucketVersioning(%s): %v", name, err)
		}
	}
	if shape == defaultRetentionBucket {
		if _, err := client.PutObjectLockConfiguration(ctx, &awss3.PutObjectLockConfigurationInput{
			Bucket: &name, ObjectLockConfiguration: &types.ObjectLockConfiguration{
				ObjectLockEnabled: types.ObjectLockEnabledEnabled,
				Rule: &types.ObjectLockRule{DefaultRetention: &types.DefaultRetention{
					Mode: types.ObjectLockRetentionModeGovernance, Days: aws.Int32(1),
				}},
			},
		}); err != nil {
			t.Fatalf("PutObjectLockConfiguration(%s): %v", name, err)
		}
	}
	return name
}

// kmsKey creates a key in the emulator and returns its ARN, the form S3 reports
// the key of an SSE-KMS version in.
func kmsKey(t *testing.T, endpoint string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client := kms.NewFromConfig(awsConfig(t), func(o *kms.Options) { o.BaseEndpoint = &endpoint })
	out, err := client.CreateKey(ctx, &kms.CreateKeyInput{Description: aws.String("dmpf s3 integration")})
	if err != nil {
		t.Fatalf("CreateKey: %v", err)
	}
	return aws.ToString(out.KeyMetadata.Arn)
}

func integrationConfig(t *testing.T, endpoint, bucket string, decl ports.ObjectStoreDeclaration) Config {
	t.Helper()
	sheet := resilience.Defaults("s3")
	return Config{
		AWS:                        awsConfig(t),
		Bucket:                     bucket,
		Endpoint:                   endpoint,
		InsecureForDevelopmentOnly: !strings.HasPrefix(endpoint, "https://"),
		UsePathStyle:               true,
		Declaration:                decl,
		Clock:                      clock.System(),
		Sheet:                      sheet,
	}
}

func apiErrorCode(err error) string {
	var api smithy.APIError
	if errors.As(err, &api) {
		return api.ErrorCode()
	}
	if err == nil {
		return "ok"
	}
	return "non-api: " + err.Error()
}
