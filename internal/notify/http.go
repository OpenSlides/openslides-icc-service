package notify

import (
	"encoding/json"
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
	Receive(meetingID, uid int) (cid string, mp NextMessage)
}

// HandleReceive registers the notify route.
func HandleReceive(mux *http.ServeMux, notify Receiver, auth icchttp.Authenticater) {
	url := icchttp.Path + "/notify"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Cache-Control", "no-store, max-age=0")

		uid := auth.FromContext(r.Context())
		if uid == 0 {
			w.WriteHeader(401)
			icchttp.ErrorNoStatus(w, iccerror.NewMessageError(iccerror.ErrNotAllowed, "Anonymous user can not receive notify messages."))
			return
		}

		meetingIDs := r.URL.Query()["meeting_id"]
		meetingID := 0
		if len(meetingIDs) != 0 {
			var err error
			meetingID, err = strconv.Atoi(meetingIDs[0])
			if err != nil {
				icchttp.Error(w, iccerror.NewMessageError(iccerror.ErrInvalid, "url query meeting_id has to be an int"))
				return
			}
		}

		cid, next := notify.Receive(meetingID, uid)

		// Send channel id.
		if _, err := fmt.Fprintf(w, `{"channel_id": "%s"}`+"\n", cid); err != nil {
			icchttp.Error(w, fmt.Errorf("sending channel id: %w", err))
			return
		}
		w.(http.Flusher).Flush()

		encoder := json.NewEncoder(w)

		for {
			message, err := next(r.Context())
			if err != nil {
				icchttp.ErrorNoStatus(w, fmt.Errorf("receiving message: %w", err))
				return
			}

			if err := encoder.Encode(message); err != nil {
				icchttp.ErrorNoStatus(w, fmt.Errorf("sending message: %w", err))
				return
			}

			w.(http.Flusher).Flush()
		}
	})

	mux.Handle(
		url,
		icchttp.AuthMiddleware(handler, auth),
	)
}

// Publisher saves a notify message.
type Publisher interface {
	Publish(io.Reader, int) error
}

// HandlePublish registers the notify/publish route.
func HandlePublish(mux *http.ServeMux, notify Publisher, auth icchttp.Authenticater) {
	url := icchttp.Path + "/notify/publish"
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		uid := auth.FromContext(r.Context())
		if uid == 0 {
			w.WriteHeader(401)
			icchttp.ErrorNoStatus(w, iccerror.NewMessageError(iccerror.ErrNotAllowed, "Anonymous user can not publish notify messages."))
			return
		}

		if err := notify.Publish(r.Body, uid); err != nil {
			icchttp.Error(w, fmt.Errorf("publish notify message: %w", err))
			return
		}
	})

	mux.Handle(
		url,
		icchttp.AuthMiddleware(handler, auth),
	)
}
