// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (RST-02, RST-03) ou FND-08 (RES-21, RES-22, RES-31) que o símbolo realiza, dentro do limite de 3 linhas.

package dmpfhttp

import (
	"bytes"
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"slices"

	"go.opentelemetry.io/otel/trace"

	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/clock"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/metrics"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-observability/resilience"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/compose"
	"gitea.lidercap.com.br/lidercap-apps/lidercap-platform/libs/backend/go/dmpf-transport/deadline"
)

const spanPrefix = "dmpf.http.client "

// maxBufferedBody bounds what is kept of a transient response so the
// connection can be reused and the last response still reaches the caller.
const maxBufferedBody = 1 << 20

var (
	// ErrRouteNotDeclared is a call to a route with no declared policy: there
	// is no default deadline, contract or retry (RST-03, RST-04).
	ErrRouteNotDeclared = errors.New("dmpfhttp: route is not declared")

	// ErrMethodMismatch is a request whose method differs from the route's:
	// the retry decision was made for the declared method (RST-02).
	ErrMethodMismatch = errors.New("dmpfhttp: request method differs from the route's")

	// ErrBodyNotReplayable is a request on a retryable route whose body cannot
	// be sent again: without GetBody a second attempt would send nothing.
	ErrBodyNotReplayable = errors.New("dmpfhttp: request body has no GetBody and the route may retry")

	// ErrIncompleteConfig is a configuration without routes or clock.
	ErrIncompleteConfig = errors.New("dmpfhttp: configuration is incomplete")
)

// Config is the client of one external dependency: its routes, the resilience
// sheet of RES-21, the transport, and what the decorators record through.
type Config struct {
	Routes      map[string]Route
	Sheet       resilience.Sheet
	Transport   http.RoundTripper
	Service     string
	Clock       clock.Clock
	Tracer      trace.Tracer
	Instruments *metrics.Instruments
	Logger      *slog.Logger
	Rand        func() float64
	NewKey      func() string
}

// Validate refuses a configuration without clock or routes, a sheet with a
// blank, or a route the Route type refuses.
func (c Config) Validate() error {
	if c.Clock == nil {
		return fmt.Errorf("%w: clock", ErrIncompleteConfig)
	}
	if len(c.Routes) == 0 {
		return fmt.Errorf("%w: no route is declared", ErrIncompleteConfig)
	}
	if err := c.Sheet.Validate(); err != nil {
		return err
	}
	for name, route := range c.Routes {
		if name != route.Name {
			return fmt.Errorf("%w: key %q names route %q", ErrIncompleteConfig, name, route.Name)
		}
		if err := route.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Client sends the declared routes through the composition of RES-22, one
// resilience.Call shared by every route: idempotency travels in the operation
// and the transient statuses are decided inside the attempt.
type Client struct {
	cfg  Config
	call resilience.Call
}

// NewClient validates the configuration and composes the call once.
func NewClient(cfg Config) (*Client, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if cfg.Transport == nil {
		cfg.Transport = http.DefaultTransport
	}
	if cfg.NewKey == nil {
		cfg.NewKey = randomKey
	}
	call, err := compose.Build(compose.Config{
		Sheet:       cfg.Sheet,
		Service:     cfg.Service,
		SpanPrefix:  spanPrefix,
		Clock:       cfg.Clock,
		Tracer:      cfg.Tracer,
		Instruments: cfg.Instruments,
		Logger:      cfg.Logger,
		Rand:        cfg.Rand,
		Category:    categoryOf,
		Classifier:  Classifier,
	})
	if err != nil {
		return nil, err
	}
	return &Client{cfg: cfg, call: call}, nil
}

// Do sends the request on the named route under the deadline its budget
// derives (RST-03); a transient status is retried only on an idempotent route
// (RST-02) and the last response comes back. Closing its Body releases the call.
func (c *Client) Do(ctx context.Context, routeName string, req *http.Request) (*http.Response, error) {
	route, declared := c.cfg.Routes[routeName]
	if !declared {
		return nil, fmt.Errorf("%w: %q", ErrRouteNotDeclared, routeName)
	}
	if req.Method != route.Method {
		return nil, fmt.Errorf("%w: %s is %s, request is %s", ErrMethodMismatch, route.Name, route.Method, req.Method)
	}
	hasBody := req.Body != nil && req.Body != http.NoBody
	if hasBody && req.GetBody == nil && route.Idempotent() {
		return nil, fmt.Errorf("%w: %s", ErrBodyNotReplayable, route.Name)
	}

	until, err := deadline.Outgoing(ctx, c.cfg.Clock, route.Budget)
	if err != nil {
		return nil, err
	}
	// Each attempt gets its own request context, tied to the decorators' attempt
	// context only while the round trip runs: a body read after Do returns must
	// not find the context the Timeout decorator already cancelled.
	outer, cancelOuter := context.WithDeadline(ctx, until)

	if route.Method == http.MethodPost && route.IdempotencyKey != "" && req.Header.Get(route.IdempotencyKey) == "" {
		req = req.Clone(outer)
		req.Header.Set(route.IdempotencyKey, c.cfg.NewKey())
	}

	op := route.Budget.Operation()
	op.Idempotent = route.Idempotent()

	var final *http.Response
	var releaseFinal context.CancelFunc
	err = c.call(outer, op, func(attemptCtx context.Context) error {
		reqCtx, cancelReq := context.WithCancel(outer)
		stop := context.AfterFunc(attemptCtx, cancelReq)
		resp, err := c.attempt(reqCtx, req, hasBody)
		stop()
		if err != nil {
			cancelReq()
			return err
		}
		if slices.Contains(route.RetryableStatus, resp.StatusCode) {
			buffer(resp)
			cancelReq()
			return &RetryableStatusError{Status: resp.StatusCode, Response: resp}
		}
		final, releaseFinal = resp, cancelReq
		return nil
	})

	var transient *RetryableStatusError
	switch {
	case err == nil:
	case errors.As(err, &transient):
		final, releaseFinal = transient.Response, func() {}
	default:
		cancelOuter()
		return nil, err
	}
	final.Body = &releasing{ReadCloser: final.Body, release: func() {
		releaseFinal()
		cancelOuter()
	}}
	return final, nil
}

// attempt sends one try. The body comes from GetBody on every try, so each
// attempt sends the whole body and the original reader is left untouched.
func (c *Client) attempt(ctx context.Context, req *http.Request, hasBody bool) (*http.Response, error) {
	try := req.Clone(ctx)
	if hasBody && req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, fmt.Errorf("dmpfhttp: GetBody: %w", err)
		}
		try.Body = body
	}
	return c.cfg.Transport.RoundTrip(try)
}

// buffer keeps up to maxBufferedBody of a transient response and drains the
// rest, so the connection is reusable and the response still readable.
func buffer(resp *http.Response) {
	if resp.Body == nil {
		return
	}
	// A read error here is the transient response's own failure: what was read
	// is kept for the caller, and the retry that follows is the real answer.
	kept, _ := io.ReadAll(io.LimitReader(resp.Body, maxBufferedBody))
	_, _ = io.Copy(io.Discard, resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(kept))
}

type releasing struct {
	io.ReadCloser
	release context.CancelFunc
}

func (r *releasing) Close() error {
	defer r.release()
	return r.ReadCloser.Close()
}

// randomKey is the idempotency key sent when the caller set none: 128 random
// bits, stable for the whole call and every attempt of it.
func randomKey() string {
	var b [16]byte
	if _, err := cryptorand.Read(b[:]); err != nil {
		panic(fmt.Sprintf("dmpfhttp: crypto/rand: %v", err))
	}
	return hex.EncodeToString(b[:])
}
