//go:build integration

package admin

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3"
	"github.com/mateusmacedo/dmpf/libs/backend/go/testkit/tb"
)

func integrationAWS(t *testing.T) aws.Config {
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

func lockedBucket(t *testing.T, endpoint string) string {
	t.Helper()
	client := awss3.NewFromConfig(integrationAWS(t), func(o *awss3.Options) { o.BaseEndpoint = &endpoint; o.UsePathStyle = true })
	name := fmt.Sprintf("dmpf-admin-%d", time.Now().UnixNano()%1_000_000_000_000)
	if _, err := client.CreateBucket(context.Background(), &awss3.CreateBucketInput{Bucket: &name, ObjectLockEnabledForBucket: aws.Bool(true)}); err != nil {
		t.Fatalf("CreateBucket: %v", err)
	}
	return name
}

func TestAdministrativePathOnTheEmulator(t *testing.T) {
	endpoint := tb.Env(t, "S3_ENDPOINT")
	bucket := lockedBucket(t, endpoint)
	insecure := !strings.HasPrefix(endpoint, "https://")
	store, err := s3.New(s3.Config{
		AWS: integrationAWS(t), Bucket: bucket, Endpoint: endpoint, InsecureForDevelopmentOnly: insecure, UsePathStyle: true,
		Declaration: ports.ObjectStoreDeclaration{
			Scope: ports.ScopeTenant, Encryption: ports.Encryption{Algorithm: "AES256"},
			Ceiling: &ports.RetentionCeiling{Span: ports.RetentionSpan(400 * 24 * time.Hour), Origin: "test"},
		},
		Clock: clock.System(), Sheet: resilience.Defaults("s3"),
	})
	if err != nil {
		t.Fatalf("s3.New() = %v", err)
	}
	ctx := tenantCtx(t, "acme")
	if err := store.Ready(ctx); err != nil {
		t.Fatalf("Ready() = %v", err)
	}
	admin, err := New(Config{
		AWS: integrationAWS(t), Bucket: bucket, Endpoint: endpoint, InsecureForDevelopmentOnly: insecure, UsePathStyle: true,
		Scope: ports.ScopeTenant, Clock: clock.System(), Sheet: resilience.Defaults("s3-admin"),
	})
	if err != nil {
		t.Fatalf("New() = %v", err)
	}
	write := func(mode ports.RetentionMode) ports.ObjectRef {
		body := []byte("evidence " + string(mode))
		sum := sha256.Sum256(body)
		until := ports.Instant(time.Now().Add(time.Minute).UnixNano())
		v, err := store.Put(ctx, ports.PutRequest{Key: "doc", Body: body, ContentSHA256: hex.EncodeToString(sum[:]),
			Retention: &ports.Retention{Mode: mode, Until: until}})
		if err != nil {
			t.Fatalf("Put(%s) = %v", mode, err)
		}
		return v.Ref()
	}

	governed := write(ports.RetentionGovernance)
	if _, err := store.Purge(ctx, governed); !errors.Is(err, ports.ErrObjectLocked) {
		t.Fatalf("store.Purge(GOVERNANCE) = %v, want ErrObjectLocked: the store has no bypass", err)
	}
	receipt, err := admin.Shorten(ctx, governed, ports.RetentionCeiling{Span: ports.RetentionSpan(30 * time.Second), Origin: "ruling-7"})
	if err != nil || receipt.Reason != "ruling-7" || receipt.To.Until >= receipt.From.Until {
		t.Fatalf("Shorten() = %+v, %v", receipt, err)
	}
	info, err := store.Head(ctx, governed)
	if err != nil || info.Retention == nil || *info.Retention != receipt.To {
		t.Fatalf("Head() after Shorten = %+v, %v; want the retention %+v", info.Retention, err, receipt.To)
	}
	if _, err := admin.Purge(ctx, governed, ports.RetentionCeiling{Span: ports.RetentionSpan(30 * time.Second), Origin: "ruling-7"}); !errors.Is(err, ErrWithinCeiling) {
		t.Fatalf("Purge(within the ceiling) = %v, want ErrWithinCeiling", err)
	}
	time.Sleep(2 * time.Second)
	purged, err := admin.Purge(ctx, governed, ports.RetentionCeiling{Span: ports.RetentionSpan(time.Second), Origin: "ruling-8"})
	if err != nil || purged.Ref != governed || purged.RetainedUntil != receipt.To.Until {
		t.Fatalf("Purge() = %+v, %v", purged, err)
	}
	if _, err := store.Get(ctx, governed); !errors.Is(err, ports.ErrObjectNotFound) {
		t.Fatalf("Get() after Purge = %v, want ErrObjectNotFound", err)
	}

	compliant := write(ports.RetentionCompliance)
	ceiling := ports.RetentionCeiling{Span: ports.RetentionSpan(time.Second), Origin: "ruling-9"}
	if _, err := admin.Shorten(ctx, compliant, ceiling); !errors.Is(err, ErrComplianceImmutable) {
		t.Fatalf("Shorten(COMPLIANCE) = %v, want ErrComplianceImmutable", err)
	}
	if _, err := admin.Purge(ctx, compliant, ceiling); !errors.Is(err, ErrComplianceImmutable) {
		t.Fatalf("Purge(COMPLIANCE) = %v, want ErrComplianceImmutable", err)
	}
}
