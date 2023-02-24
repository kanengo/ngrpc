package circuitbreaker

type CircuitBreaker interface {
	Allow()
	MarkSuccess()
	MarkFailed()
}
