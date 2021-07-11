package notify

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/OpenSlides/openslides-icc-service/internal/iccerror"
	"github.com/OpenSlides/openslides-icc-service/internal/icchttp"
)

// Receiver is a type with the function Receive(). It is a blocking function
// that writes the notify-messages to the writer as soon as they occur.
type Receiver interface {
	Receive(ctx context.Context, w io.Writer, meetingID, uid int) error
}

// HandleReceive registers the notify route.
func HandleReceive(mux *http.ServeMux, notify Receiver, auth icchttp.Authenticater) {
	mux.Handle(
		icchttp.Path+"/notify",
		icchttp.AuthMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/octet-stream")

				uid := auth.FromContext(r.Context())
				if uid == 0 {
					w.WriteHeader(401)
					icchttp.ErrorNoStatus(w, iccerror.NewMessageError(iccerror.ErrNotAllowed, "Anonymous user can not receive notify messages."))
					return
				}

				vars := r.URL.Query()["meeting_id"]
				meetingID := 0
				if len(vars) != 0 {
					var err error
					meetingID, err = strconv.Atoi(vars[0])
					if err != nil {
						icchttp.Error(w, iccerror.NewMessageError(iccerror.ErrInvalid, "url query meeting_id has to be an int"))
					}
				}

				if err := notify.Receive(r.Context(), w, meetingID, uid); err != nil {
					icchttp.ErrorNoStatus(w, fmt.Errorf("receiving notify messages: %w", err))
					return
				}
			}),
			auth,
		),
	)
}

// Sender saves a notify message.
type Sender interface {
	Send(io.Reader, int) error
}

// HandleSend registers the notify/send route.
func HandleSend(mux *http.ServeMux, notify Sender, auth icchttp.Authenticater) {
	mux.Handle(
		icchttp.Path+"/notify/send",
		icchttp.AuthMiddleware(
			http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")

				uid := auth.FromContext(r.Context())
				if uid == 0 {
					w.WriteHeader(401)
					icchttp.ErrorNoStatus(w, iccerror.NewMessageError(iccerror.ErrNotAllowed, "Anonymous user can not send notify messages."))
					return
				}

				if err := notify.Send(r.Body, uid); err != nil {
					icchttp.Error(w, fmt.Errorf("saving notify message: %w", err))
					return
				}
			}),
			auth,
		),
	)
}
