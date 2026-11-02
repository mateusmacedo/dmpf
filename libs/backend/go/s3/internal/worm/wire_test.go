package worm_test

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/worm"
)

const keyARN = "arn:aws:kms:us-east-1:000000000000:key/kit"

func storedHead(t *testing.T) (*awss3.HeadObjectOutput, string, ports.Instant) {
	t.Helper()
	sum := sha256.Sum256([]byte("evidence"))
	at := ports.Instant(1_760_000_000_123_456_789)
	return &awss3.HeadObjectOutput{
		VersionId:                 aws.String("v1"),
		ChecksumSHA256:            aws.String(worm.Checksum(sum)),
		ContentLength:             aws.Int64(8),
		ContentType:               aws.String("application/x-dmpf-kit"),
		Metadata:                  worm.StoredAt(at),
		ObjectLockMode:            types.ObjectLockModeCompliance,
		ObjectLockRetainUntilDate: aws.Time(time.Unix(1_760_000_100, 0).UTC()),
		ObjectLockLegalHoldStatus: types.ObjectLockLegalHoldStatusOn,
		ServerSideEncryption:      types.ServerSideEncryptionAwsKms,
		SSEKMSKeyId:               aws.String(keyARN),
	}, hex.EncodeToString(sum[:]), at
}

func TestInfoFromHead(t *testing.T) {
	out, digest, at := storedHead(t)
	info, err := worm.FromHead(out).Info("doc")
	if err != nil {
		t.Fatalf("Info() = %v", err)
	}
	want := ports.ObjectInfo{
		ObjectVersion: ports.ObjectVersion{Key: "doc", Version: "v1", ContentSHA256: digest, Size: 8, StoredAt: at},
		ContentType:   "application/x-dmpf-kit",
		LegalHold:     true,
		Encryption:    ports.Encryption{Algorithm: "aws:kms", KeyRef: keyARN},
	}
	retention := info.Retention
	info.Retention = nil
	if info != want {
		t.Fatalf("Info() = %+v, want %+v", info, want)
	}
	if retention == nil || *retention != (ports.Retention{Mode: ports.RetentionCompliance, Until: ports.Instant(1_760_000_100 * 1_000_000_000)}) {
		t.Fatalf("Info().Retention = %+v", retention)
	}
}

func TestInfoWithoutRetentionNorHold(t *testing.T) {
	out, _, _ := storedHead(t)
	out.ObjectLockMode, out.ObjectLockRetainUntilDate, out.ObjectLockLegalHoldStatus = "", nil, ""
	out.ServerSideEncryption, out.SSEKMSKeyId = types.ServerSideEncryptionAes256, nil
	info, err := worm.FromHead(out).Info("doc")
	if err != nil || info.Retention != nil || info.LegalHold || info.Encryption != (ports.Encryption{Algorithm: "AES256"}) {
		t.Fatalf("Info() = %+v, %v", info, err)
	}
}

func TestInfoFailsClosed(t *testing.T) {
	cases := map[string]func(*awss3.HeadObjectOutput){
		"no version":                  func(o *awss3.HeadObjectOutput) { o.VersionId = nil },
		"no checksum":                 func(o *awss3.HeadObjectOutput) { o.ChecksumSHA256 = nil },
		"checksum not base64":         func(o *awss3.HeadObjectOutput) { o.ChecksumSHA256 = aws.String("not base64!") },
		"composite checksum":          func(o *awss3.HeadObjectOutput) { o.ChecksumSHA256 = aws.String(*o.ChecksumSHA256 + "-2") },
		"no stored instant":           func(o *awss3.HeadObjectOutput) { o.Metadata = nil },
		"stored instant not a number": func(o *awss3.HeadObjectOutput) { o.Metadata = map[string]string{"dmpf-stored-at": "yesterday"} },
		"no length":                   func(o *awss3.HeadObjectOutput) { o.ContentLength = nil },
		"mode without date":           func(o *awss3.HeadObjectOutput) { o.ObjectLockRetainUntilDate = nil },
		"unknown mode":                func(o *awss3.HeadObjectOutput) { o.ObjectLockMode = "LEGAL" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			out, _, _ := storedHead(t)
			mutate(out)
			info, err := worm.FromHead(out).Info("doc")
			if !errors.Is(err, worm.ErrUnexpectedResponse) || info != (ports.ObjectInfo{}) {
				t.Fatalf("Info() = %+v, %v; want the zero value and ErrUnexpectedResponse", info, err)
			}
		})
	}
}

func TestStoredInstantIsReadCaseInsensitively(t *testing.T) {
	out, _, at := storedHead(t)
	out.Metadata = map[string]string{"Dmpf-Stored-At": out.Metadata["dmpf-stored-at"]}
	info, err := worm.FromHead(out).Info("doc")
	if err != nil || info.StoredAt != at {
		t.Fatalf("Info().StoredAt = %v, %v; want %v", info.StoredAt, err, at)
	}
}

func TestHexChecksum(t *testing.T) {
	sum := sha256.Sum256([]byte("x"))
	got, err := worm.HexChecksum(worm.Checksum(sum))
	if err != nil || got != hex.EncodeToString(sum[:]) {
		t.Fatalf("HexChecksum() = %q, %v", got, err)
	}
	if _, err := worm.HexChecksum("AAAA"); !errors.Is(err, worm.ErrUnexpectedResponse) {
		t.Fatalf("HexChecksum(short) = %v, want ErrUnexpectedResponse", err)
	}
}

func TestServerSideEncryption(t *testing.T) {
	sse, key := worm.ServerSideEncryption(ports.Encryption{Algorithm: "aws:kms", KeyRef: keyARN})
	if sse != types.ServerSideEncryptionAwsKms || aws.ToString(key) != keyARN {
		t.Fatalf("ServerSideEncryption(kms) = %q, %v", sse, key)
	}
	sse, key = worm.ServerSideEncryption(ports.Encryption{Algorithm: "AES256"})
	if sse != types.ServerSideEncryptionAes256 || key != nil {
		t.Fatalf("ServerSideEncryption(AES256) = %q, %v", sse, key)
	}
}

func TestRetainUntilIsUTCWholeSecond(t *testing.T) {
	got := worm.RetainUntil(ports.Instant(1_760_000_100_000_000_000))
	if got.Location() != time.UTC || got.Unix() != 1_760_000_100 || got.Nanosecond() != 0 {
		t.Fatalf("RetainUntil() = %v", got)
	}
}
