// comment-discipline-ok-file: arquivo de contrato público do caminho administrativo de GOVERNANCE; cada godoc cita a regra (DAT-11, DAT-14 a DAT-18, DAT-25) ou a exigência da SPEC-CWBA4BPV que o símbolo realiza, dentro do limite de 3 linhas.

// Package admin is the administrative path of GOVERNANCE retention, outside the
// port and outside s3.Store: when a stricter ceiling supersedes the one a
// version was written under (DAT-16), it shortens or purges that version and
// returns the evidence for the operator's own audit (DAT-18, DAT-25).
//
// It runs under its own credential, the only principal the bucket policy lets
// send x-amz-bypass-governance-retention. A COMPLIANCE version, which S3 lets
// no one shorten nor delete, is refused before any call.
package admin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"go.opentelemetry.io/otel/log"
	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/clock"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/metrics"
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/resilience"
	"github.com/mateusmacedo/dmpf/libs/backend/go/ports"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/sdk"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/worm"
	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/compose"
)

var (
	// ErrIncompleteConfig is a configuration without region, bucket, scope or
	// clock, or path-style addressing without an explicit endpoint.
	ErrIncompleteConfig = errors.New("admin: configuration is incomplete")

	// ErrTLSRequired is an endpoint that is not https without the explicit
	// development-only opt-out: the SigV4 signature travels in the headers.
	ErrTLSRequired = errors.New("admin: endpoint must be https outside development")

	// ErrComplianceImmutable is a COMPLIANCE version in force: S3 lets no one
	// shorten nor delete it, so the path refuses before any call (DAT-14).
	ErrComplianceImmutable = errors.New("admin: a COMPLIANCE version is never shortened nor purged before it expires")

	// ErrWithinCeiling is a version the new ceiling still allows to keep as it
	// is: there is nothing to shorten nor to purge early.
	ErrWithinCeiling = errors.New("admin: the version's retention is within the ceiling")

	// ErrOutlivedCeiling is a version older than the new ceiling allows: it is
	// purged, not shortened.
	ErrOutlivedCeiling = errors.New("admin: the version outlived the ceiling, purge it")
)

// Config is the administrative principal: its own resolved AWS configuration,
// the bucket and the scope of the store whose versions it reaches, and the
// sheet of RES-21 with the decorators' outputs.
type Config struct {
	AWS                        aws.Config
	Bucket                     string
	Endpoint                   string
	InsecureForDevelopmentOnly bool
	UsePathStyle               bool
	Scope                      ports.Scope
	Clock                      clock.Clock
	Sheet                      resilience.Sheet
	Tracer                     trace.Tracer
	Instruments                *metrics.Instruments
	LoggerProvider             log.LoggerProvider
	Rand                       func() float64
}

// Validate refuses a configuration without region, bucket, clock or a known
// scope, path-style addressing without an endpoint, a plaintext endpoint
// without the development-only opt-out, or a sheet with a blank.
func (c Config) Validate() error {
	switch {
	case c.AWS.Region == "":
		return fmt.Errorf("%w: aws region", ErrIncompleteConfig)
	case c.Bucket == "":
		return fmt.Errorf("%w: bucket", ErrIncompleteConfig)
	case c.Clock == nil:
		return fmt.Errorf("%w: clock", ErrIncompleteConfig)
	case c.Scope != ports.ScopeTenant && c.Scope != ports.ScopePlatform:
		return fmt.Errorf("%w: %w", ErrIncompleteConfig, ports.ErrScopeUndeclared)
	case c.UsePathStyle && c.Endpoint == "":
		return fmt.Errorf("%w: path-style addressing needs an explicit endpoint", ErrIncompleteConfig)
	case c.Endpoint != "" && !strings.HasPrefix(c.Endpoint, "https://") && !c.InsecureForDevelopmentOnly:
		return ErrTLSRequired
	}
	return c.Sheet.Validate()
}

// RetentionReceipt is the evidence of a shortening (DAT-18): the version, the
// retention it had and the one it has now, the origin of the ceiling that
// required it (DAT-15) and when; the operator keeps it in its own audit.
type RetentionReceipt struct {
	Ref    ports.ObjectRef
	From   ports.Retention
	To     ports.Retention
	Reason string
	At     ports.Instant
}

// PurgeReceipt is the evidence of an administrative purge: the port's receipt
// and the origin of the ceiling that required it (DAT-15, DAT-18).
type PurgeReceipt struct {
	ports.PurgeReceipt
	Reason string
}

type api interface {
	HeadObject(ctx context.Context, in *awss3.HeadObjectInput, opts ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error)
	PutObjectRetention(ctx context.Context, in *awss3.PutObjectRetentionInput, opts ...func(*awss3.Options)) (*awss3.PutObjectRetentionOutput, error)
	DeleteObject(ctx context.Context, in *awss3.DeleteObjectInput, opts ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error)
}

// Admin shortens and purges GOVERNANCE versions under a superseding ceiling.
type Admin struct {
	cfg                      Config
	api                      api
	call                     resilience.Call
	head, shorten, purgeCall resilience.Operation
}

// New builds the administrative path and its client without touching the bucket.
func New(cfg Config) (*Admin, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return build(cfg, sdk.Client(cfg.AWS, cfg.Endpoint, cfg.UsePathStyle, cfg.LoggerProvider))
}

func build(cfg Config, client api) (*Admin, error) {
	cc := compose.Config{
		Sheet: cfg.Sheet, Clock: cfg.Clock, Tracer: cfg.Tracer, Instruments: cfg.Instruments,
		LoggerProvider: cfg.LoggerProvider, Rand: cfg.Rand, Category: sdk.Category, Classifier: sdk.Classifier,
	}
	call, err := compose.Build(cc)
	if err != nil {
		return nil, err
	}
	return &Admin{
		cfg: cfg, api: client, call: call,
		head:      compose.Operation(cc, "s3 admin HeadObject", true),
		shorten:   compose.Operation(cc, "s3 admin PutObjectRetention", true),
		purgeCall: compose.Operation(cc, "s3 admin DeleteObject", true),
	}, nil
}

// Shorten brings a GOVERNANCE version in force down to the date the new ceiling
// allows, counted from when the version was written (DAT-16), with the bypass
// only this principal holds, and returns the evidence (DAT-18).
func (a *Admin) Shorten(ctx context.Context, ref ports.ObjectRef, ceiling ports.RetentionCeiling) (RetentionReceipt, error) {
	v, err := a.governed(ctx, ref, ceiling)
	if err != nil {
		return RetentionReceipt{}, err
	}
	current, now := v.info.Retention, v.now
	if current == nil || current.Until <= now {
		return RetentionReceipt{}, ErrWithinCeiling
	}
	limit, ok := ceiling.Span.Add(v.written)
	if !ok {
		return RetentionReceipt{}, ErrWithinCeiling
	}
	to := worm.WholeSeconds(ports.Retention{Mode: ports.RetentionGovernance, Until: limit})
	switch {
	case to.Until >= current.Until:
		return RetentionReceipt{}, ErrWithinCeiling
	case to.Until <= now:
		return RetentionReceipt{}, ErrOutlivedCeiling
	}
	err = a.call(ctx, a.shorten, func(ctx context.Context) error {
		_, err := a.api.PutObjectRetention(ctx, &awss3.PutObjectRetentionInput{
			Bucket: &a.cfg.Bucket, Key: &v.physical, VersionId: aws.String(string(ref.Version)),
			BypassGovernanceRetention: aws.Bool(true),
			Retention: &types.ObjectLockRetention{
				Mode: types.ObjectLockRetentionModeGovernance, RetainUntilDate: worm.RetainUntil(to.Until),
			},
		})
		return err
	})
	if err != nil {
		return RetentionReceipt{}, translate("PutObjectRetention", err)
	}
	return RetentionReceipt{Ref: ref, From: *current, To: to, Reason: ceiling.Origin, At: now}, nil
}

// Purge deletes a version past the new ceiling's limit, bypassing a GOVERNANCE
// retention still in force but never a legal hold, and returns the evidence
// for the operator's audit (DAT-18, DAT-25).
func (a *Admin) Purge(ctx context.Context, ref ports.ObjectRef, ceiling ports.RetentionCeiling) (PurgeReceipt, error) {
	v, err := a.governed(ctx, ref, ceiling)
	if err != nil {
		return PurgeReceipt{}, err
	}
	if v.info.LegalHold {
		return PurgeReceipt{}, ports.ErrObjectLocked
	}
	if limit, ok := ceiling.Span.Add(v.written); !ok || limit > v.now {
		return PurgeReceipt{}, ErrWithinCeiling
	}
	in := &awss3.DeleteObjectInput{Bucket: &a.cfg.Bucket, Key: &v.physical, VersionId: aws.String(string(ref.Version))}
	var retainedUntil ports.Instant
	if current := v.info.Retention; current != nil {
		retainedUntil = current.Until
		if current.Until > v.now {
			in.BypassGovernanceRetention = aws.Bool(true)
		}
	}
	err = a.call(ctx, a.purgeCall, func(ctx context.Context) error {
		_, err := a.api.DeleteObject(ctx, in)
		return err
	})
	if err != nil {
		return PurgeReceipt{}, translate("DeleteObject", err)
	}
	return PurgeReceipt{PurgeReceipt: ports.PurgeReceipt{Ref: ref, PurgedAt: v.now, RetainedUntil: retainedUntil}, Reason: ceiling.Origin}, nil
}

// version is what the path decides on: the physical key, the stored state,
// when the version was written and the path's now.
type version struct {
	physical string
	info     ports.ObjectInfo
	written  ports.Instant
	now      ports.Instant
}

// governed refuses an undeclared ceiling and COMPLIANCE in force before any
// bypass; the write instant is the later of dmpf-stored-at and LastModified,
// so neither a skewed clock nor a forged metadata brings the limit forward.
func (a *Admin) governed(ctx context.Context, ref ports.ObjectRef, ceiling ports.RetentionCeiling) (version, error) {
	physical, err := worm.Locate(ctx, a.cfg.Scope, ref)
	if err != nil {
		return version{}, err
	}
	if ceiling.Span <= 0 || ceiling.Origin == "" {
		return version{}, ports.ErrRetentionCeilingUndeclared
	}
	v := version{physical: physical, now: worm.InstantOf(a.cfg.Clock.Now())}
	err = a.call(ctx, a.head, func(ctx context.Context) error {
		out, err := a.api.HeadObject(ctx, &awss3.HeadObjectInput{
			Bucket: &a.cfg.Bucket, Key: &physical, VersionId: aws.String(string(ref.Version)),
			ChecksumMode: types.ChecksumModeEnabled,
		})
		if err != nil {
			return err
		}
		if v.info, err = worm.FromHead(out).Info(ref.Key); err != nil {
			return err
		}
		v.written = v.info.StoredAt
		if out.LastModified != nil {
			v.written = max(v.written, worm.InstantOf(*out.LastModified))
		}
		return nil
	})
	if err != nil {
		return version{}, translate("HeadObject", err)
	}
	if current := v.info.Retention; current != nil && current.Until > v.now && current.Mode == ports.RetentionCompliance {
		return version{}, ErrComplianceImmutable
	}
	return v, nil
}

// translate keeps the port's refusals bare and every other failure opaque, so
// no URL with the physical key reaches the operator's log (DAT-22).
func translate(operation string, err error) error {
	switch sdk.Code(err) {
	case "NoSuchKey", "NoSuchVersion", "NotFound":
		return ports.ErrObjectNotFound
	}
	if errors.Is(err, worm.ErrUnexpectedResponse) {
		return err
	}
	return sdk.Fail(operation, err)
}
