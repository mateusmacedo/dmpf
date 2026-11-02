// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-07 e FND-08 (ERR-11, RES-27) que o símbolo realiza, dentro do limite de 3 linhas.

package s3

import (
	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/retry"
	"github.com/mateusmacedo/dmpf/libs/backend/go/s3/internal/sdk"
)

// Classifier is the retry taxonomy of the S3 calls: a throttle, a server fault
// and a network failure are worth another attempt; a cancellation, a deadline
// and any other API error are not. Idempotence is the operation's to declare.
func Classifier(err error) retry.Retryability { return sdk.Classifier(err) }

func categoryOf(err error) string { return sdk.Category(err) }
