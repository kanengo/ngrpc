package ratelimit

type Limiter interface {
	Allow() error
}
