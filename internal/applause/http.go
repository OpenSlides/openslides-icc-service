package applause

import (
	"context"
	"fmt"
	"io"
	"net/http"

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

			// TODO: Get meetingID from query
			if err := applause.Send(1, uid); err != nil {
				icchttp.Error(w, fmt.Errorf("saving applause: %w", err))
				return
			}
		},
	)
}

// Receive gets applause messages.
type Receive interface {
	Receive(ctx context.Context, w io.Writer, meetingID int) error
}

// HandleReceive registers the icc/applause route.
func HandleReceive(mux *http.ServeMux, applause Receive, auth icchttp.Authenticater) {
	mux.HandleFunc(
		icchttp.Path+"/applause",
		func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Cache-Control", "no-store, max-age=0")

			// TODO: Can anonymous receive applause?
			// TODO: Get meetingID from query
			if err := applause.Receive(r.Context(), w, 1); err != nil {
				icchttp.Error(w, fmt.Errorf("receiving applause messages: %w", err))
				return
			}
		},
	)
}
