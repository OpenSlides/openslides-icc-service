package notify_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/OpenSlides/openslides-icc-service/internal/iccerror"
	"github.com/OpenSlides/openslides-icc-service/internal/icctest"
	"github.com/OpenSlides/openslides-icc-service/internal/notify"
)

func TestHandleReceive(t *testing.T) {
	url := "/system/icc/notify"

	t.Run("Anonymous", func(t *testing.T) {
		auther := icctest.AutherStub{}
		receiver := receiverStub{
			expectedMessage: "my answer",
		}
		mux := http.NewServeMux()
		notify.HandleReceive(mux, &receiver, &auther)
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, httptest.NewRequest("GET", url, nil))

		if resp.Result().StatusCode != 401 {
			t.Fatalf("handler returned status %s: %s", resp.Result().Status, resp.Body.String())
		}

		if !strings.Contains(resp.Body.String(), iccerror.ErrNotAllowed.Type()) {
			t.Errorf("handler returned message `%s`, expected to contain `%s`", resp.Body.String(), iccerror.ErrNotAllowed.Type())
		}

		if receiver.called {
			t.Errorf("handler did call the reciver")
		}
	})

	t.Run("Receiver is called", func(t *testing.T) {
		receiver := receiverStub{
			expectedMessage: "my answer",
		}
		auther := icctest.AutherStub{
			UserID: 1,
		}
		mux := http.NewServeMux()
		notify.HandleReceive(mux, &receiver, &auther)
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, httptest.NewRequest("GET", url, nil))

		if resp.Result().StatusCode != 200 {
			t.Fatalf("handler returned status %s: %s", resp.Result().Status, resp.Body.String())
		}

		if !receiver.called {
			t.Errorf("receiver was not called")
		}

		if resp.Body.String() != "my answer" {
			t.Errorf("resp body is `%s`, expected `my answer`", resp.Body.String())
		}
	})

	t.Run("Receiver is called with meetingID", func(t *testing.T) {
		receiver := receiverStub{
			expectedMessage: "my answer",
		}
		auther := icctest.AutherStub{
			UserID: 1,
		}
		mux := http.NewServeMux()
		notify.HandleReceive(mux, &receiver, &auther)
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, httptest.NewRequest("GET", url+"?meeting_id=5", nil))

		if resp.Result().StatusCode != 200 {
			t.Fatalf("handler returned status %s: %s", resp.Result().Status, resp.Body.String())
		}

		if !receiver.called {
			t.Errorf("receiver was not called")
		}

		if receiver.callledMeetingID != 5 {
			t.Errorf("receiver was called witht meetingID %d, expected 5", receiver.callledMeetingID)
		}

		if resp.Body.String() != "my answer" {
			t.Errorf("resp body is `%s`, expected `my answer`", resp.Body.String())
		}
	})

	t.Run("Receiver has an internal error", func(t *testing.T) {
		myError := errors.New("Test error")
		receiver := receiverStub{
			expectedErr: myError,
		}
		auther := icctest.AutherStub{
			UserID: 1,
		}
		mux := http.NewServeMux()
		notify.HandleReceive(mux, &receiver, &auther)
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, httptest.NewRequest("GET", url, nil))

		if resp.Result().StatusCode != 200 {
			t.Fatalf("handler returned status %s: %s", resp.Result().Status, resp.Body.String())
		}

		if strings.Contains(resp.Body.String(), myError.Error()) {
			t.Errorf("handler returned the error message: %s", resp.Body.String())
		}
	})

	t.Run("Receiver has an error for the client", func(t *testing.T) {
		myError := iccerror.ErrInvalid
		receiver := receiverStub{
			expectedErr: myError,
		}
		auther := icctest.AutherStub{
			UserID: 1,
		}
		mux := http.NewServeMux()
		notify.HandleReceive(mux, &receiver, &auther)
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, httptest.NewRequest("GET", url, nil))

		if resp.Result().StatusCode != 200 {
			t.Fatalf("handler returned status %s: %s", resp.Result().Status, resp.Body.String())
		}

		if !strings.Contains(resp.Body.String(), myError.Error()) {
			t.Errorf("handler did not return the error message: %s", resp.Body.String())
		}
	})
}

func TestHandleSend(t *testing.T) {
	url := "/system/icc/notify/publish"

	t.Run("Anonymous", func(t *testing.T) {
		auther := icctest.AutherStub{}
		sender := publisherStub{}
		mux := http.NewServeMux()
		notify.HandlePublish(mux, &sender, &auther)
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, httptest.NewRequest("GET", url, nil))

		if resp.Result().StatusCode != 401 {
			t.Fatalf("handler returned status %s: %s", resp.Result().Status, resp.Body.String())
		}

		if !strings.Contains(resp.Body.String(), iccerror.ErrNotAllowed.Type()) {
			t.Errorf("handler returned message `%s`, expected to contain `%s`", resp.Body.String(), iccerror.ErrNotAllowed.Type())
		}

		if sender.called {
			t.Errorf("handler did call the sender")
		}
	})

	t.Run("User", func(t *testing.T) {
		auther := icctest.AutherStub{
			UserID: 1,
		}
		sender := publisherStub{}
		mux := http.NewServeMux()
		notify.HandlePublish(mux, &sender, &auther)
		resp := httptest.NewRecorder()

		mux.ServeHTTP(resp, httptest.NewRequest("GET", url, nil))

		if resp.Result().StatusCode != 200 {
			t.Fatalf("handler returned status %s: %s", resp.Result().Status, resp.Body.String())
		}

		if !sender.called {
			t.Errorf("handler did not call the sender")
		}

		if sender.calledUserID != 1 {
			t.Errorf("sender was called with userID %d, expected 1", sender.calledUserID)
		}
	})

	t.Run("Internal error", func(t *testing.T) {
		myError := errors.New("Test error")
		sender := publisherStub{
			expectedErr: myError,
		}
		auther := icctest.AutherStub{
			UserID: 1,
		}
		mux := http.NewServeMux()
		notify.HandlePublish(mux, &sender, &auther)
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
