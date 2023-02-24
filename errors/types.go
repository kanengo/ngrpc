package errors

func ServiceUnavailable(message string) *Error {
	return New(503, message)
}
