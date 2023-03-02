package errors

func ServiceUnavailable(message string) *Error {
	return New(503, message)
}

func BadRequest(message string) *Error {
	return New(400, message)
}
