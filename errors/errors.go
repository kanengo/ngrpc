package errors

import (
	"google.golang.org/grpc/status"
)

const (
	InternalErrMaxCode = 9999
)

type Error struct {
	*status.Status
}

func (e *Error) Code() int32 {
	return int32(e.Status.Code())
}

func (e *Error) Message() string {
	return e.Status.Message()
}

func (e *Error) Internal() bool {
	return e.Code() <= InternalErrMaxCode
}

func FromError(err error) (Error, bool) {
	s, ok := status.FromError(err)
	return Error{s}, ok
}
