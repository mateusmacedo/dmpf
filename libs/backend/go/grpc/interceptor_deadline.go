// comment-discipline-ok-file: arquivo de contrato público; cada godoc cita a regra de FND-06 (GRP-04..07, GRP-16, GRP-17) que o símbolo realiza, dentro do limite de 3 linhas.

package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/mateusmacedo/dmpf/libs/backend/go/transport/deadline"
)

// DeadlineUnaryInterceptor derives the deadline of every unary call from the
// caller's and the method's budget (GRP-04, GRP-05, GRP-16, GRP-17); no
// deadline, an exhausted one or an undeclared method is refused before the wire.
func DeadlineUnaryInterceptor(cfg Config) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		bounded, cancel, err := outgoing(ctx, cfg, method)
		if err != nil {
			return err
		}
		defer cancel()
		return invoker(bounded, method, req, reply, cc, opts...)
	}
}

// DeadlineStreamInterceptor is the same derivation for a client stream; the
// derived context is cancelled on the first terminal error, when the stream's
// own context ends, or by its deadline — an abandoned stream lives until then.
func DeadlineStreamInterceptor(cfg Config) grpc.StreamClientInterceptor {
	return func(ctx context.Context, desc *grpc.StreamDesc, cc *grpc.ClientConn, method string, streamer grpc.Streamer, opts ...grpc.CallOption) (grpc.ClientStream, error) {
		bounded, cancel, err := outgoing(ctx, cfg, method)
		if err != nil {
			return nil, err
		}
		stream, err := streamer(bounded, desc, cc, method, opts...)
		if err != nil {
			cancel()
			return nil, err
		}
		context.AfterFunc(stream.Context(), cancel)
		return &boundedStream{ClientStream: stream, cancel: cancel}, nil
	}
}

// outgoing is the shared derivation: policy, deadline.Outgoing, then a child
// context bounded by that instant. The wire carries the remaining duration and
// the receiver rebuilds it, so the deadline is never restarted (GRP-06, GRP-07).
func outgoing(ctx context.Context, cfg Config, method string) (context.Context, context.CancelFunc, error) {
	policy, err := cfg.Policy(method)
	if err != nil {
		return nil, nil, err
	}
	until, err := deadline.Outgoing(ctx, cfg.Clock, policy.Budget)
	if err != nil {
		return nil, nil, err
	}
	bounded, cancel := cfg.Clock.WithTimeout(ctx, until.Sub(cfg.Clock.Now()))
	return bounded, cancel, nil
}

type boundedStream struct {
	grpc.ClientStream
	cancel context.CancelFunc
}

func (s *boundedStream) RecvMsg(m any) error {
	err := s.ClientStream.RecvMsg(m)
	if err != nil {
		s.cancel()
	}
	return err
}

func (s *boundedStream) SendMsg(m any) error {
	err := s.ClientStream.SendMsg(m)
	if err != nil {
		s.cancel()
	}
	return err
}

func (s *boundedStream) Header() (metadata.MD, error) {
	md, err := s.ClientStream.Header()
	if err != nil {
		s.cancel()
	}
	return md, err
}

// CloseSend that fails is terminal; one that succeeds is only the half-close,
// and the derived context stays alive for what is still to be received.
func (s *boundedStream) CloseSend() error {
	err := s.ClientStream.CloseSend()
	if err != nil {
		s.cancel()
	}
	return err
}
