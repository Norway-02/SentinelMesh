package observability

import (
	"context"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// UnaryServerInterceptor extracts W3C trace context, correlation ID, and attaches server spans.
func UnaryServerInterceptor() grpc.UnaryServerInterceptor {
	return func(
		ctx context.Context,
		req interface{},
		info *grpc.UnaryServerInfo,
		handler grpc.UnaryHandler,
	) (interface{}, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			ctx = ExtractGRPCMetadata(ctx, md)
		}

		corrID := ""
		if ok {
			if vals := md.Get("x-correlation-id"); len(vals) > 0 {
				corrID = vals[0]
			}
			if vals := md.Get("x-tenant-id"); len(vals) > 0 {
				ctx = WithTenantID(ctx, vals[0])
			}
			if vals := md.Get("x-run-id"); len(vals) > 0 {
				ctx = WithRunID(ctx, vals[0])
			}
		}
		if corrID == "" {
			corrID = uuid.NewString()
		}
		ctx = WithCorrelationID(ctx, corrID)

		ctx, span := StartSpan(ctx, "grpc."+info.FullMethod, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", info.FullMethod),
			attribute.String("sentinel.correlation_id", corrID),
		)

		resp, err := handler(ctx, req)
		if err != nil {
			s, _ := status.FromError(err)
			span.RecordError(err)
			span.SetStatus(codes.Error, s.Message())
			span.SetAttributes(attribute.Int("rpc.grpc.status_code", int(s.Code())))
		} else {
			span.SetStatus(codes.Ok, "")
			span.SetAttributes(attribute.Int("rpc.grpc.status_code", 0))
		}

		return resp, err
	}
}

// StreamServerInterceptor extracts W3C trace context and attaches server spans for gRPC streams.
func StreamServerInterceptor() grpc.StreamServerInterceptor {
	return func(
		srv interface{},
		ss grpc.ServerStream,
		info *grpc.StreamServerInfo,
		handler grpc.StreamHandler,
	) error {
		ctx := ss.Context()
		md, ok := metadata.FromIncomingContext(ctx)
		if ok {
			ctx = ExtractGRPCMetadata(ctx, md)
		}

		corrID := ""
		if ok {
			if vals := md.Get("x-correlation-id"); len(vals) > 0 {
				corrID = vals[0]
			}
			if vals := md.Get("x-tenant-id"); len(vals) > 0 {
				ctx = WithTenantID(ctx, vals[0])
			}
			if vals := md.Get("x-run-id"); len(vals) > 0 {
				ctx = WithRunID(ctx, vals[0])
			}
		}
		if corrID == "" {
			corrID = uuid.NewString()
		}
		ctx = WithCorrelationID(ctx, corrID)

		ctx, span := StartSpan(ctx, "grpc."+info.FullMethod, trace.WithSpanKind(trace.SpanKindServer))
		defer span.End()

		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", info.FullMethod),
			attribute.String("sentinel.correlation_id", corrID),
		)

		wrapped := &wrappedServerStream{
			ServerStream: ss,
			ctx:          ctx,
		}

		err := handler(srv, wrapped)
		if err != nil {
			s, _ := status.FromError(err)
			span.RecordError(err)
			span.SetStatus(codes.Error, s.Message())
			span.SetAttributes(attribute.Int("rpc.grpc.status_code", int(s.Code())))
		} else {
			span.SetStatus(codes.Ok, "")
			span.SetAttributes(attribute.Int("rpc.grpc.status_code", 0))
		}

		return err
	}
}

// UnaryClientInterceptor injects W3C trace context and correlation ID into outgoing gRPC calls.
func UnaryClientInterceptor() grpc.UnaryClientInterceptor {
	return func(
		ctx context.Context,
		method string,
		req, reply interface{},
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		ctx, span := StartSpan(ctx, "grpc.client."+method, trace.WithSpanKind(trace.SpanKindClient))
		defer span.End()

		md, ok := metadata.FromOutgoingContext(ctx)
		if !ok {
			md = metadata.New(nil)
		} else {
			md = md.Copy()
		}

		InjectGRPCMetadata(ctx, md)
		if corrID := GetCorrelationID(ctx); corrID != "" {
			md.Set("x-correlation-id", corrID)
		}
		if runID := GetRunID(ctx); runID != "" {
			md.Set("x-run-id", runID)
		}
		if tenantID := GetTenantID(ctx); tenantID != "" {
			md.Set("x-tenant-id", tenantID)
		}

		ctx = metadata.NewOutgoingContext(ctx, md)

		span.SetAttributes(
			attribute.String("rpc.system", "grpc"),
			attribute.String("rpc.method", method),
		)

		err := invoker(ctx, method, req, reply, cc, opts...)
		if err != nil {
			s, _ := status.FromError(err)
			span.RecordError(err)
			span.SetStatus(codes.Error, s.Message())
			span.SetAttributes(attribute.Int("rpc.grpc.status_code", int(s.Code())))
		} else {
			span.SetStatus(codes.Ok, "")
			span.SetAttributes(attribute.Int("rpc.grpc.status_code", 0))
		}

		return err
	}
}

type wrappedServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (w *wrappedServerStream) Context() context.Context {
	return w.ctx
}
