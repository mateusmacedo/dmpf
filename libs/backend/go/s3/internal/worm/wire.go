// comment-discipline-ok-file: tradução entre os cabeçalhos do S3 e os tipos da porta; os godocs dizem o que a tradução preserva das garantias de DAT-08, DAT-22 e da falha fechada da SPEC-CWBA4BPV, dentro do limite de 3 linhas.

package worm

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
)

// ErrUnexpectedResponse is a response without what the port promises (version,
// checksum, the store's instant): the store fails closed rather than invent it.
var ErrUnexpectedResponse = errors.New("s3: response lacks what the port promises")

// StoredAtKey is the user metadata that keeps the store's instant of the write,
// which the port returns from every read (the PUT response carries no date).
const StoredAtKey = "dmpf-stored-at"

// Headers is what HeadObject and GetObject both report about a version.
type Headers struct {
	VersionID      *string
	ChecksumSHA256 *string
	ContentLength  *int64
	ContentType    *string
	Metadata       map[string]string
	LockMode       types.ObjectLockMode
	RetainUntil    *time.Time
	LegalHold      types.ObjectLockLegalHoldStatus
	SSE            types.ServerSideEncryption
	KMSKeyID       *string
}

// FromHead reads the headers of a HeadObject response.
func FromHead(out *awss3.HeadObjectOutput) Headers {
	return Headers{
		VersionID: out.VersionId, ChecksumSHA256: out.ChecksumSHA256, ContentLength: out.ContentLength,
		ContentType: out.ContentType, Metadata: out.Metadata, LockMode: out.ObjectLockMode,
		RetainUntil: out.ObjectLockRetainUntilDate, LegalHold: out.ObjectLockLegalHoldStatus,
		SSE: out.ServerSideEncryption, KMSKeyID: out.SSEKMSKeyId,
	}
}

// FromGet reads the headers of a GetObject response.
func FromGet(out *awss3.GetObjectOutput) Headers {
	return Headers{
		VersionID: out.VersionId, ChecksumSHA256: out.ChecksumSHA256, ContentLength: out.ContentLength,
		ContentType: out.ContentType, Metadata: out.Metadata, LockMode: out.ObjectLockMode,
		RetainUntil: out.ObjectLockRetainUntilDate, LegalHold: out.ObjectLockLegalHoldStatus,
		SSE: out.ServerSideEncryption, KMSKeyID: out.SSEKMSKeyId,
	}
}

// Info is the version as the port describes it, or the zero value with
// ErrUnexpectedResponse when a header the port promises is missing or malformed.
func (h Headers) Info(key ports.ObjectKey) (ports.ObjectInfo, error) {
	if h.VersionID == nil || *h.VersionID == "" {
		return ports.ObjectInfo{}, fmt.Errorf("%w: version id", ErrUnexpectedResponse)
	}
	if h.ChecksumSHA256 == nil {
		return ports.ObjectInfo{}, fmt.Errorf("%w: sha-256 checksum", ErrUnexpectedResponse)
	}
	digest, err := HexChecksum(*h.ChecksumSHA256)
	if err != nil {
		return ports.ObjectInfo{}, err
	}
	if h.ContentLength == nil {
		return ports.ObjectInfo{}, fmt.Errorf("%w: content length", ErrUnexpectedResponse)
	}
	storedAt, err := storedAt(h.Metadata)
	if err != nil {
		return ports.ObjectInfo{}, err
	}
	retention, err := h.retention()
	if err != nil {
		return ports.ObjectInfo{}, err
	}
	info := ports.ObjectInfo{
		ObjectVersion: ports.ObjectVersion{
			Key: key, Version: ports.VersionID(*h.VersionID), ContentSHA256: digest,
			Size: *h.ContentLength, StoredAt: storedAt,
		},
		Retention:  retention,
		LegalHold:  h.LegalHold == types.ObjectLockLegalHoldStatusOn,
		Encryption: ports.Encryption{Algorithm: string(h.SSE)},
	}
	if h.ContentType != nil {
		info.ContentType = *h.ContentType
	}
	if h.KMSKeyID != nil {
		info.Encryption.KeyRef = *h.KMSKeyID
	}
	return info, nil
}

func (h Headers) retention() (*ports.Retention, error) {
	if h.LockMode == "" {
		return nil, nil
	}
	if h.RetainUntil == nil {
		return nil, fmt.Errorf("%w: retention mode without date", ErrUnexpectedResponse)
	}
	mode := ports.RetentionMode(h.LockMode)
	if !mode.Known() {
		return nil, fmt.Errorf("%w: retention mode", ErrUnexpectedResponse)
	}
	dated := WholeSeconds(ports.Retention{Mode: mode, Until: InstantOf(*h.RetainUntil)})
	return &dated, nil
}

func storedAt(metadata map[string]string) (ports.Instant, error) {
	for name, value := range metadata {
		if strings.EqualFold(name, StoredAtKey) {
			at, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return 0, fmt.Errorf("%w: stored instant", ErrUnexpectedResponse)
			}
			return ports.Instant(at), nil
		}
	}
	return 0, fmt.Errorf("%w: stored instant", ErrUnexpectedResponse)
}

// Checksum is the SHA-256 as S3 carries it, in Base64.
func Checksum(sum [sha256.Size]byte) string {
	return base64.StdEncoding.EncodeToString(sum[:])
}

// HexChecksum is the Base64 SHA-256 of S3 as the port carries it, in lower-case
// hexadecimal; a composite (multipart) or malformed checksum is refused.
func HexChecksum(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(raw) != sha256.Size {
		return "", fmt.Errorf("%w: sha-256 checksum", ErrUnexpectedResponse)
	}
	return hex.EncodeToString(raw), nil
}

// StoredAt is the metadata that keeps the store's instant of a write.
func StoredAt(at ports.Instant) map[string]string {
	return map[string]string{StoredAtKey: strconv.FormatInt(int64(at), 10)}
}

// ServerSideEncryption is the declared encryption as the PUT carries it, so the
// version is encrypted as declared whatever the bucket default is (DAT-08).
func ServerSideEncryption(e ports.Encryption) (types.ServerSideEncryption, *string) {
	if e.KeyRef == "" || types.ServerSideEncryption(e.Algorithm) == types.ServerSideEncryptionAes256 {
		return types.ServerSideEncryption(e.Algorithm), nil
	}
	key := e.KeyRef
	return types.ServerSideEncryption(e.Algorithm), &key
}

// RetainUntil is a retention date as S3 takes it, in UTC.
func RetainUntil(at ports.Instant) *time.Time {
	t := time.Unix(0, int64(at)).UTC()
	return &t
}

// InstantOf is a date of S3 as the port carries it.
func InstantOf(t time.Time) ports.Instant {
	return ports.Instant(t.UnixNano())
}
