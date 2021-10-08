package iccerror

import "fmt"

const (
	// ErrInternal should not happen.
	ErrInternal TypeError = iota

	// ErrInvalid happens, when an invalid icc-message is send.
	ErrInvalid

	// ErrNotAllowed happens on a vote request, when the request user is
	// anonymous or is not allowed for the request.
	ErrNotAllowed
)

// TypeError is an error that can happend in this API.
type TypeError int

// Type returns a name for the error.
func (err TypeError) Type() string {
	switch err {
	case ErrInvalid:
		return "invalid"

	case ErrNotAllowed:
		return "not-allowed"

	default:
		return "internal"
	}
}

func (err TypeError) Error() string {
	var msg string
	switch err {
	case ErrInvalid:
		msg = "The input data is invalid."

	case ErrNotAllowed:
		msg = "You are not allowed to do this."

	default:
		msg = "Ups, something went wrong!"

	}
	return fmt.Sprintf(`{"error":"%s","msg":"%s"}`, err.Type(), msg)
}

// MessageError is a TypeError with an individuel error message.
type MessageError struct {
	t   TypeError
	msg string
}

// NewMessageError creates an error of a specific type with a different message.
//
// This are messages, that should be send to the client.
func NewMessageError(t TypeError, format string, a ...interface{}) error {
	return MessageError{
		t,
		fmt.Sprintf(format, a...),
	}
}

func (err MessageError) Error() string {
	return fmt.Sprintf(`{"error":"%s","msg":"%s"}`, err.t.Type(), err.msg)
}

func (err MessageError) Unwrap() error {
	return err.t
}
