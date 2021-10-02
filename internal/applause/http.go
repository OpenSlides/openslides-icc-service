package applause

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/OpenSlides/openslides-icc-service/internal/iccerror"
	"github.com/OpenSlides/openslides-icc-service/internal/icchttp"
)

// Sender saves the applause.
type Sender interface {
	Send(meetingID, uid int) error
}

// HandleSend registers the icc/applause route.
func HandleSend(mux *http.ServeMux, applause Sender, auth icchttp.Authenticater) {
	mux.HandleFunc(
		icchttp.Path+"/applause/send",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			uid := auth.FromContext(r.Context())
			if uid == 0 {
				w.WriteHeader(401)
				icchttp.ErrorNoStatus(w, iccerror.NewMessageError(iccerror.ErrNotAllowed, "Anonymous user can not send applause."))
				return
			}

			// TODO: What permission is needed to send applause?

			meetingStr := r.URL.Query().Get("meeting_id")
			meetingID, err := strconv.Atoi(meetingStr)
			if err != nil {
				icchttp.Error(w, iccerror.NewMessageError(iccerror.ErrInvalid, "Query meeting has to be an int."))
				return
			}

			if err := applause.Send(meetingID, uid); err != nil {
				icchttp.Error(w, fmt.Errorf("saving applause: %w", err))
				return
			}
		},
	)
}

// Receive gets applause messages.
type Receive interface {
	Receive(ctx context.Context, tid uint64, meetingID int) (newTID uint64, msg MSG, err error)
	LastID() uint64
}

// HandleReceive registers the icc/applause route.
func HandleReceive(mux *http.ServeMux, applause Receive, auth icchttp.Authenticater) {
	mux.HandleFunc(
		icchttp.Path+"/applause",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")

			// TODO: Can anonymous receive applause?

			meetingStr := r.URL.Query().Get("meeting_id")
			meetingID, err := strconv.Atoi(meetingStr)
			if err != nil {
				icchttp.Error(w, iccerror.NewMessageError(iccerror.ErrInvalid, "Query meeting has to be an int."))
				return
			}

			var tid uint64
			encoder := json.NewEncoder(w)
			if err := encoder.Encode(MSG{}); err != nil {
				icchttp.Error(w, fmt.Errorf("writing firstmessage: %w", err))
				return
			}
			w.(http.Flusher).Flush()

			for {
				var message MSG
				tid, message, err = applause.Receive(r.Context(), tid, meetingID)
				if err != nil {
					icchttp.ErrorNoStatus(w, fmt.Errorf("receive applause data: %w", err))
					return
				}

				if err := encoder.Encode(message); err != nil {
					icchttp.ErrorNoStatus(w, fmt.Errorf("writing message: %w", err))
					return
				}
				w.(http.Flusher).Flush()
			}
		},
	)
}
