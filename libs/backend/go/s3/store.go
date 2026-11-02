// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra (DAT-08, DAT-11, DAT-14, DAT-18, IDN-14, RES-27) ou a exigência da SPEC-CWBA4BPV que o símbolo realiza, dentro do limite de 3 linhas.

package s3

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/sdk"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/worm"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/compose"
)

// Store realizes ports.ObjectStore over one bucket with Object Lock. No input
// it builds sets BypassGovernanceRetention: the bypass is not a capability of
// the port (DAT-11), only of the package admin.
type Store struct {
	cfg   Config
	api   s3API
	call  resilience.Call
	ops   operations
	ready atomic.Bool
}

type operations struct {
	put, get, head, retain, hold, purge, versioning, lock resilience.Operation
}

var _ ports.ObjectStore = (*Store)(nil)

// New builds the store and its client without touching the bucket; it refuses
// every operation until Ready confirms the bucket keeps versions under lock.
func New(cfg Config) (*Store, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return build(cfg, NewClient(cfg))
}

func build(cfg Config, api s3API) (*Store, error) {
	cc := composition(cfg)
	call, err := compose.Build(cc)
	if err != nil {
		return nil, err
	}
	if cfg.Declaration.Ceiling != nil {
		ceiling := *cfg.Declaration.Ceiling
		cfg.Declaration.Ceiling = &ceiling
	}
	put := compose.Operation(cc, "s3 PutObject", false)
	put.EffectAbsent = effectAbsent
	return &Store{cfg: cfg, api: api, call: call, ops: operations{
		put:        put,
		get:        compose.Operation(cc, "s3 GetObject", true),
		head:       compose.Operation(cc, "s3 HeadObject", true),
		retain:     compose.Operation(cc, "s3 PutObjectRetention", true),
		hold:       compose.Operation(cc, "s3 PutObjectLegalHold", true),
		purge:      compose.Operation(cc, "s3 DeleteObject", true),
		versioning: compose.Operation(cc, "s3 GetBucketVersioning", true),
		lock:       compose.Operation(cc, "s3 GetObjectLockConfiguration", true),
	}}, nil
}

func composition(cfg Config) compose.Config {
	return compose.Config{
		Sheet:          cfg.Sheet,
		Clock:          cfg.Clock,
		Tracer:         cfg.Tracer,
		Instruments:    cfg.Instruments,
		LoggerProvider: cfg.LoggerProvider,
		Rand:           cfg.Rand,
		Category:       categoryOf,
		Classifier:     Classifier,
	}
}

// Ready confirms the bucket keeps versions, has Object Lock enabled and no
// default retention; until it does, every operation is refused. A failed check may be repeated, and S3
// never lets the lock be disabled, so a confirmation holds.
func (s *Store) Ready(ctx context.Context) error {
	var versioning *awss3.GetBucketVersioningOutput
	err := s.call(ctx, s.ops.versioning, func(ctx context.Context) error {
		out, err := s.api.GetBucketVersioning(ctx, &awss3.GetBucketVersioningInput{Bucket: &s.cfg.Bucket})
		versioning = out
		return err
	})
	if err != nil {
		return sdk.Fail("GetBucketVersioning", err)
	}
	if versioning == nil || versioning.Status != types.BucketVersioningStatusEnabled {
		return fmt.Errorf("%w: versioning is not enabled", ErrBucketNotImmutable)
	}
	var lock *awss3.GetObjectLockConfigurationOutput
	err = s.call(ctx, s.ops.lock, func(ctx context.Context) error {
		out, err := s.api.GetObjectLockConfiguration(ctx, &awss3.GetObjectLockConfigurationInput{Bucket: &s.cfg.Bucket})
		lock = out
		return err
	})
	if sdk.Code(err) == "ObjectLockConfigurationNotFoundError" {
		return fmt.Errorf("%w: object lock is not configured", ErrBucketNotImmutable)
	}
	if err != nil {
		return sdk.Fail("GetObjectLockConfiguration", err)
	}
	if lock == nil || lock.ObjectLockConfiguration == nil || lock.ObjectLockConfiguration.ObjectLockEnabled != types.ObjectLockEnabledEnabled {
		return fmt.Errorf("%w: object lock is not enabled", ErrBucketNotImmutable)
	}
	if rule := lock.ObjectLockConfiguration.Rule; rule != nil && rule.DefaultRetention != nil {
		return fmt.Errorf("%w: a default retention would retain the writes the port declares free (DAT-17)", ErrBucketNotImmutable)
	}
	s.ready.Store(true)
	return nil
}

func (s *Store) confirmed() error {
	if !s.ready.Load() {
		return ErrNotReady
	}
	return nil
}

// Put writes a new version with its checksum, the declared encryption (DAT-08)
// and, when asked, a dated retention under the ceiling (DAT-14). It is never
// repeated once sent: an unknown outcome is ErrPutOutcomeUnknown (RES-27).
func (s *Store) Put(ctx context.Context, req ports.PutRequest) (ports.ObjectVersion, error) {
	if err := req.Key.Validate(); err != nil {
		return ports.ObjectVersion{}, err
	}
	physical, err := worm.Physical(ctx, s.cfg.Declaration.Scope, req.Key)
	if err != nil {
		return ports.ObjectVersion{}, err
	}
	sum := sha256.Sum256(req.Body)
	digest := hex.EncodeToString(sum[:])
	if req.ContentSHA256 != digest {
		return ports.ObjectVersion{}, ports.ErrIntegrity
	}
	now := s.now()
	var retention *ports.Retention
	if req.Retention != nil {
		dated := worm.WholeSeconds(*req.Retention)
		if err := worm.CheckRetention(now, s.cfg.Declaration.Ceiling, dated); err != nil {
			return ports.ObjectVersion{}, err
		}
		retention = &dated
	}
	if err := s.confirmed(); err != nil {
		return ports.ObjectVersion{}, err
	}

	checksum := worm.Checksum(sum)
	sse, keyID := worm.ServerSideEncryption(s.cfg.Declaration.Encryption)
	in := awss3.PutObjectInput{
		Bucket:               &s.cfg.Bucket,
		Key:                  &physical,
		ContentLength:        aws.Int64(int64(len(req.Body))),
		ChecksumSHA256:       &checksum,
		ServerSideEncryption: sse,
		SSEKMSKeyId:          keyID,
		Metadata:             worm.StoredAt(now),
	}
	if req.ContentType != "" {
		in.ContentType = &req.ContentType
	}
	if retention != nil {
		in.ObjectLockMode = types.ObjectLockMode(retention.Mode)
		in.ObjectLockRetainUntilDate = worm.RetainUntil(retention.Until)
	}
	var out *awss3.PutObjectOutput
	err = s.call(ctx, s.ops.put, func(ctx context.Context) error {
		attempt := in
		attempt.Body = bytes.NewReader(req.Body)
		var err error
		out, err = s.api.PutObject(ctx, &attempt)
		return err
	})
	if err != nil {
		return ports.ObjectVersion{}, putOutcome(err)
	}
	if out == nil || aws.ToString(out.VersionId) == "" {
		return ports.ObjectVersion{}, fmt.Errorf("%w: %w: the write returned no version id", ErrPutOutcomeUnknown, ErrBucketNotImmutable)
	}
	if aws.ToString(out.ChecksumSHA256) != checksum {
		return ports.ObjectVersion{}, fmt.Errorf("%w: version %s was written with another checksum", ErrPutOutcomeUnknown, aws.ToString(out.VersionId))
	}
	return ports.ObjectVersion{
		Key: req.Key, Version: ports.VersionID(*out.VersionId), ContentSHA256: digest,
		Size: int64(len(req.Body)), StoredAt: now,
	}, nil
}

// Get reads one version and refuses a body that does not hash to the checksum
// the server keeps for it.
func (s *Store) Get(ctx context.Context, ref ports.ObjectRef) (ports.Object, error) {
	physical, err := s.locate(ctx, ref)
	if err != nil {
		return ports.Object{}, err
	}
	var object ports.Object
	err = s.call(ctx, s.ops.get, func(ctx context.Context) error {
		out, err := s.api.GetObject(ctx, &awss3.GetObjectInput{
			Bucket: &s.cfg.Bucket, Key: &physical, VersionId: aws.String(string(ref.Version)),
			ChecksumMode: types.ChecksumModeEnabled,
		})
		if err != nil {
			return err
		}
		defer func() { _ = out.Body.Close() }()
		body, err := io.ReadAll(out.Body)
		if err != nil {
			return err
		}
		info, err := worm.FromGet(out).Info(ref.Key)
		if err != nil {
			return err
		}
		object = ports.Object{ObjectInfo: info, Body: body}
		return nil
	})
	if err != nil {
		return ports.Object{}, translate("GetObject", err)
	}
	sum := sha256.Sum256(object.Body)
	if hex.EncodeToString(sum[:]) != object.ContentSHA256 {
		return ports.Object{}, fmt.Errorf("%w: the body does not hash to the stored checksum", ports.ErrIntegrity)
	}
	return object, nil
}

// Head reports the retention, the legal hold and the encryption of the stored
// version, so DAT-08 is checked on the surface, not in the code that writes.
func (s *Store) Head(ctx context.Context, ref ports.ObjectRef) (ports.ObjectInfo, error) {
	physical, err := s.locate(ctx, ref)
	if err != nil {
		return ports.ObjectInfo{}, err
	}
	return s.head(ctx, physical, ref)
}

// Retain extends, or promotes GOVERNANCE to COMPLIANCE on a date no earlier,
// within the ceiling (DAT-14); S3 refuses the rest again, without a bypass.
func (s *Store) Retain(ctx context.Context, ref ports.ObjectRef, to ports.Retention) (ports.ObjectInfo, error) {
	physical, err := s.locate(ctx, ref)
	if err != nil {
		return ports.ObjectInfo{}, err
	}
	now := s.now()
	to = worm.WholeSeconds(to)
	if err := worm.CheckRetention(now, s.cfg.Declaration.Ceiling, to); err != nil {
		return ports.ObjectInfo{}, err
	}
	info, err := s.head(ctx, physical, ref)
	if err != nil {
		return ports.ObjectInfo{}, err
	}
	if err := worm.CheckChange(now, info.Retention, to); err != nil {
		return ports.ObjectInfo{}, err
	}
	err = s.call(ctx, s.ops.retain, func(ctx context.Context) error {
		_, err := s.api.PutObjectRetention(ctx, &awss3.PutObjectRetentionInput{
			Bucket: &s.cfg.Bucket, Key: &physical, VersionId: aws.String(string(ref.Version)),
			Retention: &types.ObjectLockRetention{
				Mode:            types.ObjectLockRetentionMode(to.Mode),
				RetainUntilDate: worm.RetainUntil(to.Until),
			},
		})
		return err
	})
	if err != nil {
		return ports.ObjectInfo{}, translate("PutObjectRetention", err)
	}
	info.Retention = &to
	return info, nil
}

// SetLegalHold puts or lifts the hold without touching the retention; putting
// it requires the declared origin of the obligation (DAT-15).
func (s *Store) SetLegalHold(ctx context.Context, ref ports.ObjectRef, hold ports.LegalHold) (ports.HoldReceipt, error) {
	physical, err := s.locate(ctx, ref)
	if err != nil {
		return ports.HoldReceipt{}, err
	}
	if hold.On && hold.Reason == "" {
		return ports.HoldReceipt{}, ports.ErrLegalHoldReasonRequired
	}
	now := s.now()
	status := types.ObjectLockLegalHoldStatusOff
	if hold.On {
		status = types.ObjectLockLegalHoldStatusOn
	}
	err = s.call(ctx, s.ops.hold, func(ctx context.Context) error {
		_, err := s.api.PutObjectLegalHold(ctx, &awss3.PutObjectLegalHoldInput{
			Bucket: &s.cfg.Bucket, Key: &physical, VersionId: aws.String(string(ref.Version)),
			LegalHold: &types.ObjectLockLegalHold{Status: status},
		})
		return err
	})
	if err != nil {
		return ports.HoldReceipt{}, translate("PutObjectLegalHold", err)
	}
	return ports.HoldReceipt{Ref: ref, On: hold.On, Reason: hold.Reason, At: now}, nil
}

// Purge deletes a free version by its id and returns the evidence the caller
// keeps in its own audit (DAT-18, DAT-25); a held or retained one is locked.
func (s *Store) Purge(ctx context.Context, ref ports.ObjectRef) (ports.PurgeReceipt, error) {
	physical, err := s.locate(ctx, ref)
	if err != nil {
		return ports.PurgeReceipt{}, err
	}
	now := s.now()
	info, err := s.head(ctx, physical, ref)
	if err != nil {
		return ports.PurgeReceipt{}, err
	}
	retainedUntil, err := worm.Purgeable(now, info.Retention, info.LegalHold)
	if err != nil {
		return ports.PurgeReceipt{}, err
	}
	err = s.call(ctx, s.ops.purge, func(ctx context.Context) error {
		_, err := s.api.DeleteObject(ctx, &awss3.DeleteObjectInput{
			Bucket: &s.cfg.Bucket, Key: &physical, VersionId: aws.String(string(ref.Version)),
		})
		return err
	})
	if err != nil {
		return ports.PurgeReceipt{}, translate("DeleteObject", err)
	}
	return ports.PurgeReceipt{Ref: ref, PurgedAt: now, RetainedUntil: retainedUntil}, nil
}

func (s *Store) locate(ctx context.Context, ref ports.ObjectRef) (string, error) {
	physical, err := worm.Locate(ctx, s.cfg.Declaration.Scope, ref)
	if err != nil {
		return "", err
	}
	if err := s.confirmed(); err != nil {
		return "", err
	}
	return physical, nil
}

func (s *Store) head(ctx context.Context, physical string, ref ports.ObjectRef) (ports.ObjectInfo, error) {
	var info ports.ObjectInfo
	err := s.call(ctx, s.ops.head, func(ctx context.Context) error {
		out, err := s.api.HeadObject(ctx, &awss3.HeadObjectInput{
			Bucket: &s.cfg.Bucket, Key: &physical, VersionId: aws.String(string(ref.Version)),
			ChecksumMode: types.ChecksumModeEnabled,
		})
		if err != nil {
			return err
		}
		info, err = worm.FromHead(out).Info(ref.Key)
		return err
	})
	if err != nil {
		return ports.ObjectInfo{}, translate("HeadObject", err)
	}
	return info, nil
}

func (s *Store) now() ports.Instant {
	return worm.InstantOf(s.cfg.Clock.Now())
}

// translate reads the answers of S3 that are the port's own refusals; the
// sentinel goes bare, and every other failure is opaque, so no text of the
// response nor the URL with the physical key reaches the caller (DAT-22).
func translate(operation string, err error) error {
	switch sdk.Code(err) {
	case "NoSuchKey", "NoSuchVersion", "NotFound":
		return ports.ErrObjectNotFound
	case "BadDigest":
		return ports.ErrIntegrity
	}
	if errors.Is(err, worm.ErrUnexpectedResponse) {
		return err
	}
	return sdk.Fail(operation, err)
}

// putOutcome keeps a refusal S3 answered as it is and marks every other failure
// after the request may have left as an unknown outcome: the version may exist.
func putOutcome(err error) error {
	if effectAbsent(err) || errors.Is(err, resilience.ErrBreakerOpen) || errors.Is(err, resilience.ErrBulkheadSaturated) {
		return sdk.Fail("PutObject", err)
	}
	if answered(err) {
		return translate("PutObject", err)
	}
	return fmt.Errorf("%w: %w", ErrPutOutcomeUnknown, sdk.Fail("PutObject", err))
}

// answered is a definite refusal: a 4xx response, or, when no response is at
// hand, an API error the client faulted. A 5xx may follow a stored write.
func answered(err error) bool {
	if status := sdk.Status(err); status != 0 {
		return status < 500
	}
	var api smithy.APIError
	return errors.As(err, &api) && api.ErrorFault() == smithy.FaultClient
}

// effectAbsent is the failure of a write that never reached S3: the connection
// was not established, so another attempt cannot create a second version.
func effectAbsent(err error) bool {
	var op *net.OpError
	if errors.As(err, &op) && op.Op == "dial" {
		return true
	}
	var dns *net.DNSError
	return errors.As(err, &dns)
}
