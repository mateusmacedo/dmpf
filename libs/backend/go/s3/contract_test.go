//go:build integration

package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials/stscreds"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/aws-sdk-go-v2/service/sts"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/admin"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/sdk"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/worm"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/providerkit"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

// The contract runs against real S3 in hmg (.github/workflows/dmpf-contract-aws.yml).
// Outside it the variables are absent and every TestContract skips; with
// S3_CONTRACT_REQUIRED=1, as the workflow sets, an absent variable fails, so
// the contract never passes without having proved anything.
const defaultContractUnit = 10 * time.Second

func contractEnv(t *testing.T, name string) string {
	t.Helper()
	value := os.Getenv(name)
	if value != "" {
		return value
	}
	if os.Getenv("S3_CONTRACT_REQUIRED") == "1" {
		t.Fatalf("%s is empty and S3_CONTRACT_REQUIRED=1: the contract must not pass without proving anything", name)
	}
	t.Skipf("%s not set: the contract runs only against real S3 in hmg", name)
	return ""
}

func contractClient(t *testing.T, roleARN string) (aws.Config, *awss3.Client) {
	t.Helper()
	cfg := awsConfig(t)
	if roleARN != "" {
		cfg.Credentials = aws.NewCredentialsCache(stscreds.NewAssumeRoleProvider(sts.NewFromConfig(cfg), roleARN))
	}
	return cfg, sdk.Client(cfg, "", false, nil)
}

func contractStoreConfig(cfg aws.Config, bucket string, decl ports.ObjectStoreDeclaration) Config {
	return Config{AWS: cfg, Bucket: bucket, Declaration: decl, Clock: clock.System(), Sheet: resilience.Defaults("s3")}
}

func contractBucket(t *testing.T, client *awss3.Client, region, prefix string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	name := fmt.Sprintf("%s-%d-%d", prefix, time.Now().UnixNano()%1_000_000_000_000, bucketSequence.Add(1))
	in := &awss3.CreateBucketInput{Bucket: &name, ObjectLockEnabledForBucket: aws.Bool(true)}
	if region != "us-east-1" {
		in.CreateBucketConfiguration = &types.CreateBucketConfiguration{LocationConstraint: types.BucketLocationConstraint(region)}
	}
	if _, err := client.CreateBucket(ctx, in); err != nil {
		t.Fatalf("CreateBucket(%s): %v", name, err)
	}
	t.Cleanup(func() { removeAfterExpiry(t, client, name) })
	return name
}

// removeAfterExpiry deletes what the retentions of the test let go; a version
// still held or retained stays, and the bucket with it, until the next run.
func removeAfterExpiry(t *testing.T, client *awss3.Client, bucket string) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pages := awss3.NewListObjectVersionsPaginator(client, &awss3.ListObjectVersionsInput{Bucket: &bucket})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			t.Logf("cleanup %s: %v", bucket, err)
			return
		}
		for _, v := range page.Versions {
			if _, err := client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: &bucket, Key: v.Key, VersionId: v.VersionId}); err != nil {
				t.Logf("cleanup %s: version left: %s", bucket, sdk.Code(err))
			}
		}
	}
	if _, err := client.DeleteBucket(ctx, &awss3.DeleteBucketInput{Bucket: &bucket}); err != nil {
		t.Logf("cleanup %s: bucket left: %s", bucket, sdk.Code(err))
	}
}

// sweepLeftovers removes the buckets an earlier run left with versions still
// retained; an hour is far past any retention the contract writes.
func sweepLeftovers(t *testing.T, client *awss3.Client, prefix string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	out, err := client.ListBuckets(ctx, &awss3.ListBucketsInput{Prefix: aws.String(prefix + "-")})
	if err != nil {
		t.Logf("sweep: %s", sdk.Code(err))
		return
	}
	for _, b := range out.Buckets {
		if b.CreationDate != nil && time.Since(*b.CreationDate) > time.Hour {
			removeAfterExpiry(t, client, *b.Name)
		}
	}
}

func lockedWrite(t *testing.T, client *awss3.Client, bucket string, mode types.ObjectLockMode, until time.Time, withChecksum bool) (string, error) {
	t.Helper()
	body := []byte("contract " + string(mode))
	in := &awss3.PutObjectInput{
		Bucket: &bucket, Key: aws.String("contract"), Body: bytes.NewReader(body),
		ObjectLockMode: mode, ObjectLockRetainUntilDate: aws.Time(until),
	}
	if withChecksum {
		sum := sha256.Sum256(body)
		in.ChecksumSHA256 = aws.String(worm.Checksum(sum))
	}
	out, err := client.PutObject(context.Background(), in)
	if err != nil {
		return "", err
	}
	return aws.ToString(out.VersionId), nil
}

func refusedWith(t *testing.T, what string, err error, code string) {
	t.Helper()
	if got := sdk.Code(err); got != code {
		t.Errorf("%s = %v (code %q), want %s", what, err, got, code)
	}
}

func TestContractObjectStoreConforms(t *testing.T) {
	prefix := contractEnv(t, "S3_CONTRACT_BUCKET_PREFIX")
	keyARN := contractEnv(t, "S3_CONTRACT_KMS_KEY_ARN")
	unit := retentionUnit(t, "S3_CONTRACT_RETENTION_UNIT", defaultContractUnit)
	cfg, client := contractClient(t, "")
	sweepLeftovers(t, client, prefix)
	connect := func(bucket string, decl ports.ObjectStoreDeclaration) Config {
		return contractStoreConfig(cfg, bucket, decl)
	}
	v := providerkit.ObjectStore(s3Subject(connect, client, keyARN, unit, func() string {
		return contractBucket(t, client, cfg.Region, prefix)
	}))
	t.Logf("kit against real S3: %d diagnostics, skipped %v, retention unit %s", len(v.Diagnostics), v.Skipped, unit)
	tb.Require(t, v)
	if len(v.Skipped) != 0 {
		t.Fatalf("skipped %v: every clause runs against real S3", v.Skipped)
	}
	time.Sleep(5 * unit)
}

// TestContractComplianceResistsABroadPrincipal proves on real S3 what the
// emulator accepts: no principal, however broad, deletes or shortens a
// COMPLIANCE version, and S3 itself refuses a lock without checksum or date.
func TestContractComplianceResistsABroadPrincipal(t *testing.T) {
	prefix := contractEnv(t, "S3_CONTRACT_BUCKET_PREFIX")
	unit := retentionUnit(t, "S3_CONTRACT_RETENTION_UNIT", defaultContractUnit)
	cfg, client := contractClient(t, "")
	bucket := contractBucket(t, client, cfg.Region, prefix)
	ctx := context.Background()
	until := time.Now().Add(3 * unit).Truncate(time.Second)

	control, err := lockedWrite(t, client, bucket, types.ObjectLockModeGovernance, until, true)
	if err != nil {
		t.Fatalf("PutObject(GOVERNANCE control) = %v", err)
	}
	if _, err := client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: &bucket, Key: aws.String("contract"), VersionId: &control, BypassGovernanceRetention: aws.Bool(true)}); err != nil {
		t.Fatalf("DeleteObject(GOVERNANCE, bypass) = %v: the runner lacks s3:BypassGovernanceRetention, so an AccessDenied on COMPLIANCE would prove nothing", err)
	}

	compliance, err := lockedWrite(t, client, bucket, types.ObjectLockModeCompliance, until, true)
	if err != nil {
		t.Fatalf("PutObject(COMPLIANCE) = %v", err)
	}
	for _, bypass := range []bool{false, true} {
		_, err := client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: &bucket, Key: aws.String("contract"), VersionId: &compliance, BypassGovernanceRetention: aws.Bool(bypass)})
		refusedWith(t, fmt.Sprintf("DeleteObject(COMPLIANCE, bypass=%v)", bypass), err, "AccessDenied")
		_, err = client.PutObjectRetention(ctx, &awss3.PutObjectRetentionInput{Bucket: &bucket, Key: aws.String("contract"), VersionId: &compliance,
			BypassGovernanceRetention: aws.Bool(bypass),
			Retention:                 &types.ObjectLockRetention{Mode: types.ObjectLockRetentionModeCompliance, RetainUntilDate: aws.Time(until.Add(-unit))}})
		refusedWith(t, fmt.Sprintf("PutObjectRetention(shorter COMPLIANCE, bypass=%v)", bypass), err, "AccessDenied")
	}
	_, err = client.PutObjectRetention(ctx, &awss3.PutObjectRetentionInput{Bucket: &bucket, Key: aws.String("contract"), VersionId: &compliance,
		Retention: &types.ObjectLockRetention{Mode: types.ObjectLockRetentionModeGovernance, RetainUntilDate: aws.Time(until.Add(unit))}})
	refusedWith(t, "PutObjectRetention(demote COMPLIANCE)", err, "AccessDenied")

	governance, err := lockedWrite(t, client, bucket, types.ObjectLockModeGovernance, until, true)
	if err != nil {
		t.Fatalf("PutObject(GOVERNANCE) = %v", err)
	}
	if _, err := client.PutObjectRetention(ctx, &awss3.PutObjectRetentionInput{Bucket: &bucket, Key: aws.String("contract"), VersionId: &governance,
		Retention: &types.ObjectLockRetention{Mode: types.ObjectLockRetentionModeCompliance, RetainUntilDate: aws.Time(until)}}); err != nil {
		t.Errorf("PutObjectRetention(promote GOVERNANCE on the same date) = %v, want nil", err)
	}

	_, err = lockedWrite(t, client, bucket, types.ObjectLockModeCompliance, until, false)
	if sdk.Code(err) == "" {
		t.Errorf("PutObject(retention without checksum) = %v, want the refusal S3 documents", err)
	}
	_, err = lockedWrite(t, client, bucket, types.ObjectLockModeCompliance, time.Now().Add(-time.Hour), true)
	if sdk.Code(err) == "" {
		t.Errorf("PutObject(retention in the past) = %v, want the refusal S3 documents", err)
	}
	time.Sleep(4 * unit)
}

// TestContractBypassIsTheAdministratorsOnly proves the segregation of the
// bypass on the dedicated GOVERNANCE bucket, whose policy denies it to every
// principal but the administrative one.
func TestContractBypassIsTheAdministratorsOnly(t *testing.T) {
	bucket := contractEnv(t, "S3_CONTRACT_GOVERNANCE_BUCKET")
	appRole := contractEnv(t, "S3_CONTRACT_APP_ROLE_ARN")
	adminRole := contractEnv(t, "S3_CONTRACT_ADMIN_ROLE_ARN")
	unit := retentionUnit(t, "S3_CONTRACT_RETENTION_UNIT", defaultContractUnit)
	appCfg, appClient := contractClient(t, appRole)
	adminCfg, _ := contractClient(t, adminRole)
	ctx := tenantCtx(t, "contract")

	store, err := New(contractStoreConfig(appCfg, bucket, ports.ObjectStoreDeclaration{
		Scope: ports.ScopeTenant, Encryption: ports.Encryption{Algorithm: "AES256"},
		Ceiling: &ports.RetentionCeiling{Span: ports.RetentionSpan(10 * unit), Origin: "contract"},
	}))
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	if err := store.Ready(ctx); err != nil {
		t.Fatalf("Ready() = %v", err)
	}
	body := []byte("contract governance")
	sum := sha256.Sum256(body)
	version, err := store.Put(ctx, ports.PutRequest{Key: "governed", Body: body, ContentSHA256: hex.EncodeToString(sum[:]),
		Retention: &ports.Retention{Mode: ports.RetentionGovernance, Until: worm.InstantOf(time.Now().Add(8 * unit))}})
	if err != nil {
		t.Fatalf("Put(GOVERNANCE) = %v", err)
	}
	physical := "tenant/contract/governed"
	_, err = appClient.DeleteObject(context.Background(), &awss3.DeleteObjectInput{Bucket: &bucket, Key: &physical,
		VersionId: aws.String(string(version.Version)), BypassGovernanceRetention: aws.Bool(true)})
	refusedWith(t, "DeleteObject(app role with bypass)", err, "AccessDenied")

	administrator, err := admin.New(admin.Config{AWS: adminCfg, Bucket: bucket, Scope: ports.ScopeTenant, Clock: clock.System(), Sheet: resilience.Defaults("s3-admin")})
	if err != nil {
		t.Fatalf("admin.New() = %v", err)
	}
	receipt, err := administrator.Shorten(ctx, version.Ref(), ports.RetentionCeiling{Span: ports.RetentionSpan(4 * unit), Origin: "contract-ruling"})
	if err != nil || receipt.To.Until >= receipt.From.Until {
		t.Fatalf("Shorten() = %+v, %v", receipt, err)
	}
	time.Sleep(time.Duration(unit) + time.Second)
	purged, err := administrator.Purge(ctx, version.Ref(), ports.RetentionCeiling{Span: ports.RetentionSpan(unit), Origin: "contract-ruling"})
	if err != nil || purged.Ref != version.Ref() {
		t.Fatalf("Purge() = %+v, %v", purged, err)
	}
	if _, err := store.Get(ctx, version.Ref()); !errors.Is(err, ports.ErrObjectNotFound) {
		t.Fatalf("Get() after the administrative purge = %v, want ErrObjectNotFound", err)
	}
}
