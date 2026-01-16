// Package icchttp contains helper function to handle http requests.
//
// The handers are defined by its own package (icc, applause...).
package icchttp

import (
	"context"
	"crypto/tls"
	"encoding/json"
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
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
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
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintln(w, `{"healthy":true, "service":"icc"}`)
		},
	)
}

// HealthClient sends a http request to a server to fetch the health status.
func HealthClient(ctx context.Context, useHTTPS bool, host, port string, insecure bool) error {
	proto := "http"
	if useHTTPS {
		proto = "https"
	}

	req, err := http.NewRequestWithContext(
		ctx,
		"GET",
		fmt.Sprintf("%s://%s:%s/system/icc/health", proto, host, port),
		nil,
	)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	if insecure {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("health returned status %s", resp.Status)
	}

	var body struct {
		Healthy bool   `json:"healthy"`
		Service string `json:"service"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return fmt.Errorf("reading and parsing response body: %w", err)
	}

	if !body.Healthy || body.Service != "icc" {
		return fmt.Errorf("Server returned unhealthy response")
	}

	return nil
}
