// comment-discipline-ok-file: arquivo de contrato interno; o godoc cita a regra (RES-27, SPEC-CWBA4BPV) que o símbolo realiza, dentro do limite de 3 linhas.

package s3

import (
	"context"
	"strings"
	"sync"

	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/sdk"
)

// s3API is the slice of *s3.Client the store uses, declared here so a test
// substitutes it. It has no method that could carry the governance bypass
// other than the two whose input the store builds without it.
type s3API interface {
	PutObject(ctx context.Context, in *awss3.PutObjectInput, opts ...func(*awss3.Options)) (*awss3.PutObjectOutput, error)
	GetObject(ctx context.Context, in *awss3.GetObjectInput, opts ...func(*awss3.Options)) (*awss3.GetObjectOutput, error)
	HeadObject(ctx context.Context, in *awss3.HeadObjectInput, opts ...func(*awss3.Options)) (*awss3.HeadObjectOutput, error)
	PutObjectRetention(ctx context.Context, in *awss3.PutObjectRetentionInput, opts ...func(*awss3.Options)) (*awss3.PutObjectRetentionOutput, error)
	PutObjectLegalHold(ctx context.Context, in *awss3.PutObjectLegalHoldInput, opts ...func(*awss3.Options)) (*awss3.PutObjectLegalHoldOutput, error)
	DeleteObject(ctx context.Context, in *awss3.DeleteObjectInput, opts ...func(*awss3.Options)) (*awss3.DeleteObjectOutput, error)
	GetBucketVersioning(ctx context.Context, in *awss3.GetBucketVersioningInput, opts ...func(*awss3.Options)) (*awss3.GetBucketVersioningOutput, error)
	GetObjectLockConfiguration(ctx context.Context, in *awss3.GetObjectLockConfigurationInput, opts ...func(*awss3.Options)) (*awss3.GetObjectLockConfigurationOutput, error)
}

var _ s3API = (*awss3.Client)(nil)

// NewClient is the real client over the resolved AWS configuration, pointed at
// the endpoint when one is configured. The SDK never repeats a call: only the
// sheet authorizes another attempt (RES-27).
func NewClient(cfg Config) *awss3.Client {
	warnPlaintext(cfg)
	return sdk.Client(cfg.AWS, cfg.Endpoint, cfg.UsePathStyle, cfg.LoggerProvider)
}

var plaintextWarning = new(sync.Once)

func warnPlaintext(cfg Config) {
	if cfg.Endpoint != "" && !strings.HasPrefix(cfg.Endpoint, "https://") {
		plaintextWarning.Do(func() {
			cfg.logger().Warn("s3: endpoint without TLS by explicit development-only opt-out")
		})
	}
}
