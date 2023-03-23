package circuitbreaker

const (
	StateOpened = 1
	StateClosed = 0
)

type Breaker interface {
	Allow() error
	MarkSuccess()
	MarkFailed()
}
