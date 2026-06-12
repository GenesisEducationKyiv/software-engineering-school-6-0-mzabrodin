package metrics

import (
	"context"
	"log/slog"
	"time"

	"connectrpc.com/connect"
)

func NewConnectObservabilityInterceptor(
	log *slog.Logger,
	protocol func(context.Context) string,
) connect.UnaryInterceptorFunc {
	return func(next connect.UnaryFunc) connect.UnaryFunc {
		return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
			start := time.Now()

			resp, err := next(ctx, req)

			proto := protocol(ctx)
			if proto == "" {
				proto = "unknown"
			}

			procedure := req.Spec().Procedure

			code := "ok"
			if err != nil {
				code = connect.CodeOf(err).String()
			}

			duration := time.Since(start)

			log.InfoContext(ctx, "request",
				"protocol", proto,
				"procedure", procedure,
				"code", code,
				"duration_ms", duration.Milliseconds(),
			)

			RequestsTotal.WithLabelValues(proto, procedure, code).Inc()
			RequestDuration.WithLabelValues(proto, procedure).Observe(duration.Seconds())

			return resp, err
		}
	}
}
