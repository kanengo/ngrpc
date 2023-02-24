package errors

import (
	"errors"
	"fmt"

	"google.golang.org/grpc/status"
)

const (
	InternalErrMaxCode = 9999

	UnknownCode = 500
)

type Error struct {
	Status
	cause error
}

func New(code int32, message string) *Error {
	return &Error{
		Status: Status{
			Code:    code,
			Message: message,
		},
	}
}

func (e *Error) Error() string {
	return fmt.Sprintf("error: code = %d message = %s metadata = %v", e.Code, e.Message, e.Metadata)
}

func (e *Error) Internal() bool {
	return e.Code <= InternalErrMaxCode
}

func (e *Error) Unwrap() error {
	return e.cause
}

func FromError(err error) *Error {
	if err == nil {
		return nil
	}
	if se := new(Error); errors.As(err, &se) {
		return se
	}
	gs, ok := status.FromError(err)
	if !ok {
		return New(UnknownCode, err.Error())
	}

	ret := New(int32(gs.Code()), gs.Message())

	//for _, detail := range gs.Details() {
	//	switch d := detail.(type) {
	//
	//	}
	//}

	return ret
}
