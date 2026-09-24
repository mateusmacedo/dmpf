// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (SQS-01, SQS-02, TRP-19) que o símbolo realiza, dentro do limite de 3 linhas.

package sqs

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// EncodeBody is the one textual encoding of TRP-19: the serialized envelope in
// standard Base64, applied by the publisher and by nobody in between.
func EncodeBody(envelope []byte) string {
	return base64.StdEncoding.EncodeToString(envelope)
}

// DecodeBody is the matching single decode (SQS-01). The SNS notification
// wrapper is refused as ErrSNSEnvelopeNotRaw: the subscription did not deliver
// raw, and reading the wrapper as the envelope is the non-conforming hop of §5.2.
func DecodeBody(body string) ([]byte, error) {
	if isSNSNotification(body) {
		return nil, ErrSNSEnvelopeNotRaw
	}
	raw, err := base64.StdEncoding.DecodeString(body)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidBody, err)
	}
	return raw, nil
}

// snsNotification are the fields that identify the wrapper SNS puts around a
// message when raw delivery is off.
type snsNotification struct {
	Type     string  `json:"Type"`
	TopicArn string  `json:"TopicArn"`
	Message  *string `json:"Message"`
}

func isSNSNotification(body string) bool {
	if len(body) == 0 || body[0] != '{' || !json.Valid([]byte(body)) {
		return false
	}
	var n snsNotification
	if err := json.Unmarshal([]byte(body), &n); err != nil {
		return false
	}
	return n.Type == "Notification" && n.TopicArn != "" && n.Message != nil
}
