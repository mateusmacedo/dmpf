//go:build integration

package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/worm"
)

// rawMatrix sends, outside the store, the five operations that take a version
// to an absent key, an absent version of a present key and the version of
// another key, and returns what the emulator answered to each.
func rawMatrix(t *testing.T) map[string]string {
	t.Helper()
	endpoint := emulator(t)
	client := rawS3(t, endpoint)
	bucket := createBucket(t, client, lockedBucket)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	put := func(key string) string {
		sum := sha256.Sum256([]byte(key))
		out, err := client.PutObject(ctx, &awss3.PutObjectInput{
			Bucket: &bucket, Key: &key, Body: bytes.NewReader([]byte(key)), ChecksumSHA256: aws.String(worm.Checksum(sum)),
		})
		if err != nil {
			t.Fatalf("PutObject(%s): %v", key, err)
		}
		return aws.ToString(out.VersionId)
	}
	other := put("other")
	put("present")
	gone := put("present")
	if _, err := client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: &bucket, Key: aws.String("present"), VersionId: &gone}); err != nil {
		t.Fatalf("DeleteObject(gone): %v", err)
	}
	refs := map[string][2]string{
		"absent key":             {"absent", other},
		"absent version":         {"present", gone},
		"version of another key": {"present", other},
	}
	got := map[string]string{}
	for name, ref := range refs {
		key, version := ref[0], ref[1]
		_, err := client.GetObject(ctx, &awss3.GetObjectInput{Bucket: &bucket, Key: &key, VersionId: &version})
		got["GetObject/"+name] = apiErrorCode(err)
		_, err = client.HeadObject(ctx, &awss3.HeadObjectInput{Bucket: &bucket, Key: &key, VersionId: &version})
		got["HeadObject/"+name] = apiErrorCode(err)
		_, err = client.PutObjectRetention(ctx, &awss3.PutObjectRetentionInput{Bucket: &bucket, Key: &key, VersionId: &version,
			Retention: &types.ObjectLockRetention{Mode: types.ObjectLockRetentionModeGovernance, RetainUntilDate: aws.Time(time.Now().Add(time.Hour))}})
		got["PutObjectRetention/"+name] = apiErrorCode(err)
		_, err = client.PutObjectLegalHold(ctx, &awss3.PutObjectLegalHoldInput{Bucket: &bucket, Key: &key, VersionId: &version,
			LegalHold: &types.ObjectLockLegalHold{Status: types.ObjectLockLegalHoldStatusOn}})
		got["PutObjectLegalHold/"+name] = apiErrorCode(err)
		_, err = client.DeleteObject(ctx, &awss3.DeleteObjectInput{Bucket: &bucket, Key: &key, VersionId: &version})
		got["DeleteObject/"+name] = apiErrorCode(err)
	}
	return got
}

// TestEmulatorErrorMatrix pins what the emulator answers for a version that does
// not resolve, the table translate reads. DeleteObject answers success, as S3
// does for an absent version, which is why Purge reads the version first.
func TestEmulatorErrorMatrix(t *testing.T) {
	want := map[string]string{}
	for _, ref := range []string{"absent key", "absent version", "version of another key"} {
		want["GetObject/"+ref] = "NoSuchVersion"
		want["HeadObject/"+ref] = "NotFound"
		want["PutObjectRetention/"+ref] = "NoSuchVersion"
		want["PutObjectLegalHold/"+ref] = "NoSuchVersion"
		want["DeleteObject/"+ref] = "ok"
	}
	got := rawMatrix(t)
	for op, code := range want {
		if got[op] != code {
			t.Errorf("%s = %s, want %s", op, got[op], code)
		}
		if code != "ok" && translate("probe", &smithyCode{code}) != ports.ErrObjectNotFound {
			t.Errorf("translate(%s) is not ErrObjectNotFound", code)
		}
	}
}

type smithyCode struct{ code string }

func (e *smithyCode) Error() string                 { return e.code }
func (e *smithyCode) ErrorCode() string             { return e.code }
func (e *smithyCode) ErrorMessage() string          { return "" }
func (e *smithyCode) ErrorFault() smithy.ErrorFault { return smithy.FaultClient }
