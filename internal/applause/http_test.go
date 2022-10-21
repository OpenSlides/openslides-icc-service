package applause_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenSlides/openslides-icc-service/internal/applause"
	"github.com/OpenSlides/openslides-icc-service/internal/iccerror"
	"github.com/OpenSlides/openslides-icc-service/internal/icctest"
)

func TestHandleSend(t *testing.T) {
	url := "/system/icc/applause/send?meeting_id=1"

	t.Run("Anonymous", func(t *testing.T) {
		auther := icctest.AutherStub{}
		applauser := applauserStub{}
		mux := http.NewServeMux()
		applause.HandleSend(mux, &applauser, &auther)
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, httptest.NewRequest("GET", url, nil))

		if resp.Result().StatusCode != 401 {
			t.Fatalf("handler returned status %s: %s", resp.Result().Status, resp.Body.String())
		}

		if !strings.Contains(resp.Body.String(), iccerror.ErrNotAllowed.Type()) {
			t.Errorf("handler returned message `%s`, expected to contain `%s`", resp.Body.String(), iccerror.ErrNotAllowed.Type())
		}

		if applauser.called {
			t.Errorf("handler did call the applauser")
		}
	})

	t.Run("User", func(t *testing.T) {
		auther := icctest.AutherStub{
			UserID: 1,
		}
		applauser := applauserStub{}
		mux := http.NewServeMux()
		applause.HandleSend(mux, &applauser, &auther)
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, httptest.NewRequest("GET", url, nil))

		if resp.Result().StatusCode != 200 {
			t.Fatalf("handler returned status %s: %s", resp.Result().Status, resp.Body.String())
		}

		if !applauser.called {
			t.Errorf("handler did not call the applauser")
		}

		if applauser.calledUserID != 1 {
			t.Errorf("applauser was called with userID %d, expected 1", applauser.calledUserID)
		}
	})

	t.Run("Internal error", func(t *testing.T) {
		myError := errors.New("Test error")
		applauser := applauserStub{
			expectedErr: myError,
		}
		auther := icctest.AutherStub{
			UserID: 1,
		}
		mux := http.NewServeMux()
		applause.HandleSend(mux, &applauser, &auther)
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, httptest.NewRequest("GET", url, nil))

		if resp.Result().StatusCode != 500 {
			t.Fatalf("handler returned status %s: %s", resp.Result().Status, resp.Body.String())
		}

		if strings.Contains(resp.Body.String(), myError.Error()) {
			t.Errorf("handler returned the error message: %s", resp.Body.String())
		}
	})
}
