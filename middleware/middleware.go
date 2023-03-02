package middleware

import (
	"context"
)

type Handler func(ctx context.Context, req any) (any, error)

type Middleware func(Handler) Handler

func Chain(ms ...Middleware) Middleware {
	return func(next Handler) Handler {
		for i := 0; i < len(ms); i++ {
			next = ms[i](next)
		}
		return next
	}
}
