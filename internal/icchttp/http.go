// Package icchttp contains helper function to handle http requests.
//
// The handers are defined by its own package (icc, applause...).
package icchttp

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/OpenSlides/openslides-icc-service/internal/iccerror"
	"github.com/OpenSlides/openslides-icc-service/internal/icclog"
)

const (
	// Path is the basic path for all handlers of this service.
	Path = "/system/icc"
)

// Authenticater knowns how to authenticate a request.
type Authenticater interface {
	Authenticate(http.ResponseWriter, *http.Request) (context.Context, error)
	FromContext(context.Context) int
}

// ErrorNoStatus is like Error(), but does not write a status message.
func ErrorNoStatus(w io.Writer, err error) {
	if isConnectionClose(err) {
		return
	}

	msg := err.Error()

	var errTyped interface {
		error
		Type() string
	}
	if !errors.As(err, &errTyped) {
		// Unknown error. Handle as 500er.
		msg = iccerror.ErrInternal.Error()
		icclog.Info("Error: %v", err)
	}

	fmt.Fprint(w, msg)
}

// Error sends an error message to the client as json-message.
//
// If the error does not have a Type() string message, it is handled as 500er.
// In other case, it is handled as 400er.
func Error(w http.ResponseWriter, err error) {
	if isConnectionClose(err) {
		return
	}

	var errTyped interface {
		error
		Type() string
	}
	status := 500
	if errors.As(err, &errTyped) {
		if errTyped.Type() != iccerror.ErrInternal.Error() {
			status = 400
		}
	}

	w.WriteHeader(status)
	icclog.Debug("HTTP: Returning status %d", status)
	ErrorNoStatus(w, err)
}

func isConnectionClose(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		// Client closes connection.
		return true
	}

	var closing interface {
		Closing()
	}
	if errors.As(err, &closing) {
		// Server is shutting down.
		return true
	}
	return false
}

// AuthMiddleware checks the user id of the request.
func AuthMiddleware(next http.Handler, auth Authenticater) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, err := auth.Authenticate(w, r)
		if err != nil {
			w.WriteHeader(401)
			ErrorNoStatus(w, iccerror.NewMessageError(iccerror.ErrNotAllowed, "Anonymous user can not receive icc messages."))
			return
		}
		r = r.WithContext(ctx)
		next.ServeHTTP(w, r)
	})
}

// HandleHealth returns 200 (if the service is running).
func HandleHealth(mux *http.ServeMux) {
	mux.HandleFunc(
		"/system/icc/health",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/octet-stream")
			fmt.Fprintln(w, `{"healthy": true}`)
		},
	)
}
