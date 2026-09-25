package api

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"

	"go.opentelemetry.io/otel/trace"

	"github.com/mateusmacedo/dmpf/libs/backend/go/observability/tracing"

	"github.com/mateusmacedo/dmpf/apps/backend/bff/app/rpc"
)

const (
	maxBodyBytes = 1 << 16
	maxSKULength = 128
)

var (
	idFormat = regexp.MustCompile(`^[A-Za-z0-9._:-]{1,128}$`)

	errMissingResult = errors.New("api: the context answered without a result")
	errUnknownStatus = errors.New("api: the context answered an unknown status")
)

type handlers struct {
	orders       OrdersClient
	reservations ReservationsClient
}

type rejection struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type refusal interface {
	GetCode() string
	GetMessage() string
}

func pathID(w http.ResponseWriter, r *http.Request, name string) (string, bool) {
	id := r.PathValue(name)
	if !idFormat.MatchString(id) {
		writeRejection(r, w, http.StatusBadRequest, "invalid-request", name+" must have 1 to 128 characters of [A-Za-z0-9._:-]")
		return "", false
	}
	return id, true
}

// decodeBody reads exactly one JSON object of the contract: unknown fields and
// trailing data are refused, so nothing half-parsed reaches a context.
func decodeBody(w http.ResponseWriter, r *http.Request, into any) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		return err
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("api: trailing data after the request body")
	}
	return nil
}

func writeFailure(w http.ResponseWriter, r *http.Request, err error) {
	failure := rpc.Classify(err)
	if failure.Status >= http.StatusInternalServerError {
		tracing.RecordError(trace.SpanFromContext(r.Context()), failure.Code)
	}
	writeRejection(r, w, failure.Status, failure.Code, failure.Message)
}

func writeRefusal(r *http.Request, w http.ResponseWriter, refused refusal) {
	writeRejection(r, w, http.StatusUnprocessableEntity, refused.GetCode(), refused.GetMessage())
}

func writeRejection(r *http.Request, w http.ResponseWriter, status int, code, message string) {
	writeJSON(r, w, status, rejection{Code: code, Message: message})
}

// WHY: the status line is already on the wire when the body fails, so the only
// thing left is to say so on the span; silence makes a truncated answer look
// like a success in every dashboard.
func writeJSON(r *http.Request, w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		tracing.RecordError(trace.SpanFromContext(r.Context()), "response")
	}
}

// WHY: the published contract fixes Rejection as the body of every 4xx, and
// the kernel's default refusal is plain text, which no generated client parses.
func refuseAsRejection(w http.ResponseWriter, r *http.Request, status int, reason string) {
	code := "admission-refused"
	if status == http.StatusNotFound {
		code = "not-found"
	}
	writeRejection(r, w, status, code, reason)
}
