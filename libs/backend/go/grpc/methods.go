package grpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// Method is the descriptor of a method the service declares. An undeclared
// name panics: a ServiceDesc is built at package initialization, so a method
// missing from the .proto fails at startup instead of at the first call.
func Method(desc protoreflect.ServiceDescriptor, name protoreflect.Name) protoreflect.MethodDescriptor {
	md := desc.Methods().ByName(name)
	if md == nil {
		panic("grpc: " + string(desc.FullName()) + " declares no method " + string(name))
	}
	return md
}

func MethodNames(desc protoreflect.ServiceDescriptor) []string {
	methods := desc.Methods()
	names := make([]string, 0, methods.Len())
	for i := range methods.Len() {
		names = append(names, string(methods.Get(i).Name()))
	}
	return names
}

func FullMethod(service, method string) string { return "/" + service + "/" + method }

// Unary builds the MethodDesc that serves md with call. On error it returns an
// untyped nil response, so no interface ever holds a nil message pointer.
func Unary[S, Req any, PReq interface {
	*Req
	protoreflect.ProtoMessage
}, Resp protoreflect.ProtoMessage](service string, md protoreflect.MethodDescriptor, call func(S, context.Context, PReq) (Resp, error)) grpc.MethodDesc {
	name := string(md.Name())
	fullMethod := FullMethod(service, name)
	invoke := func(server S, ctx context.Context, req PReq) (any, error) {
		resp, err := call(server, ctx, req)
		if err != nil {
			return nil, err
		}
		return resp, nil
	}
	return grpc.MethodDesc{
		MethodName: name,
		Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
			in := PReq(new(Req))
			if err := dec(in); err != nil {
				return nil, err
			}
			server := srv.(S)
			if interceptor == nil {
				return invoke(server, ctx, in)
			}
			info := &grpc.UnaryServerInfo{Server: srv, FullMethod: fullMethod}
			return interceptor(ctx, in, info, func(ctx context.Context, req any) (any, error) {
				return invoke(server, ctx, req.(PReq))
			})
		},
	}
}

// Uncovered lists, in descriptor order, the methods the service declares that sd
// does not register exactly once as unary: gRPC keeps the last of two MethodDesc
// with one name, and a StreamDesc covers no unary method.
func Uncovered(sd *grpc.ServiceDesc, desc protoreflect.ServiceDescriptor) []string {
	registered := make(map[string]int, len(sd.Methods))
	for _, m := range sd.Methods {
		registered[m.MethodName]++
	}
	var uncovered []string
	for _, name := range MethodNames(desc) {
		if registered[name] != 1 {
			uncovered = append(uncovered, name)
		}
	}
	return uncovered
}
