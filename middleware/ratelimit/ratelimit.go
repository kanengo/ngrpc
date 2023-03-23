package ratelimit

import (
	"context"

	"github.com/kanengo/ngrpc/errors"
	"github.com/kanengo/ngrpc/middleware"
)

type Limiter interface {
	Allow() error
}

var ErrTriggerLimit = errors.ServiceUnavailable("Trigger server limit, please try again later")

func RateLimit(limiter Limiter) middleware.Middleware {
	return func(handler middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			if err := limiter.Allow(); err != nil {
				return nil, ErrTriggerLimit
			}
			return handler(ctx, req)
		}
	}
}
