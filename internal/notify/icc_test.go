package notify_test

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/OpenSlides/openslides-icc-service/internal/iccerror"
	"github.com/OpenSlides/openslides-icc-service/internal/notify"
)

func TestSend(t *testing.T) {
	backend := newBackendStrub()
	icc := notify.New(context.Background(), backend)

	t.Run("invalid json", func(t *testing.T) {
		defer backend.reset()

		err := icc.Send(strings.NewReader(`{123`), 1)

		if !errors.Is(err, iccerror.ErrInvalid) {
			t.Errorf("send() returned err `%s`, expected `%s`", err, iccerror.ErrInvalid.Error())
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		defer backend.reset()

		err := icc.Send(strings.NewReader(`{"to_users":1,"message":"hans"}`), 1)

		if !errors.Is(err, iccerror.ErrInvalid) {
			t.Errorf("send() returned err `%s`, expected `%s`", err, iccerror.ErrInvalid.Error())
		}
	})

	t.Run("no channel_id", func(t *testing.T) {
		defer backend.reset()

		err := icc.Send(strings.NewReader(`
		{
			"to_users": [2], 
			"message": "hans"
		}`), 1)

		if !errors.Is(err, iccerror.ErrInvalid) {
			t.Fatalf("send returned unexpected error: %v", err)
		}

	})

	t.Run("invalid channel_id", func(t *testing.T) {
		defer backend.reset()

		err := icc.Send(strings.NewReader(`
		{
			"channel_id": "abc",
			"to_users": [2], 
			"message": "hans"
		}`), 1)

		if !errors.Is(err, iccerror.ErrInvalid) {
			t.Fatalf("send returned unexpected error: %v", err)
		}
	})

	t.Run("no Name", func(t *testing.T) {
		defer backend.reset()

		err := icc.Send(strings.NewReader(`
		{
			"channel_id": "server:1:2",
			"to_users": [2], 
			"message": "hans"
		}`), 1)

		if !errors.Is(err, iccerror.ErrInvalid) {
			t.Fatalf("send returned unexpected error: %v", err)
		}
	})

	t.Run("valid", func(t *testing.T) {
		defer backend.reset()

		err := icc.Send(strings.NewReader(`
		{
			"channel_id": "server:1:2",
			"name": "message-name",
			"to_users": [2], 
			"message": "hans"
		}`), 1)

		if err != nil {
			t.Fatalf("send returned unexpected error: %v", err)
		}

		if len(backend.receivedMessages) != 1 {
			t.Fatalf("backend received %d messages, expected 1", len(backend.receivedMessages))
		}

		expected := `{"channel_id":"server:1:2","to_users":[2],"name":"message-name","message":"hans"}`
		if string(backend.receivedMessages[0]) != expected {
			t.Errorf("received message:\n%s\n\nexpected:\n%s", backend.receivedMessages[0], expected)
		}
	})
}

func TestReceive(t *testing.T) {
	backend := newBackendStrub()
	icc := notify.New(context.Background(), backend)

	r, w := io.Pipe()
	decoder := json.NewDecoder(r)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	receiveDone := make(chan error, 1)
	go func() {
		receiveDone <- icc.Receive(ctx, w, 1, 2)
	}()

	t.Run("Get channel id", func(t *testing.T) {
		var firstMessage struct {
			ChannelID string `json:"channel_id"`
		}
		if err := decoder.Decode(&firstMessage); err != nil {
			t.Errorf("decoding first message: %v", err)
		}

		if firstMessage.ChannelID == "" {
			t.Errorf("first message did not have a channelID")
		}
	})

	t.Run("Get first message", func(t *testing.T) {
		if err := icc.Send(strings.NewReader(`{"channel_id":"server:1:2","name":"message-name","to_users":[2],"message":"hans"}`), 1); err != nil {
			t.Fatalf("sending message: %v", err)
		}

		var iccMessage struct {
			SenderUserID    int             `json:"sender_user_id"`
			SenderChannelID string          `json:"sender_channel_id"`
			Name            string          `json:"name"`
			Message         json.RawMessage `json:"message"`
		}
		if err := decoder.Decode(&iccMessage); err != nil {
			t.Errorf("decoding first message: %v", err)
		}

		if iccMessage.SenderUserID != 1 {
			t.Errorf("message.sender_user_id == %d, expected 1", iccMessage.SenderUserID)
		}

		if iccMessage.SenderChannelID != "server:1:2" {
			t.Errorf("message.sender_channel_id == %s, expected server:1:2", iccMessage.SenderChannelID)
		}

		if iccMessage.Name != "message-name" {
			t.Errorf("message.name == %s, expected message-name", iccMessage.Name)
		}

		if string(iccMessage.Message) != `"hans"` {
			t.Errorf("message.message == %s, expected hans", iccMessage.Message)
		}
	})

	t.Run("Message for meeting", func(t *testing.T) {
		if err := icc.Send(strings.NewReader(`{"channel_id":"server:1:2","name":"to-meeting-name","to_meeting":1,"message":"klaus"}`), 1); err != nil {
			t.Fatalf("sending message: %v", err)
		}

		var iccMessage struct {
			ToMeeting int             `json:"to_meeting"`
			Name      string          `json:"name"`
			Message   json.RawMessage `json:"message"`
		}
		if err := decoder.Decode(&iccMessage); err != nil {
			t.Errorf("decoding first message: %v", err)
		}

		if iccMessage.Name != "to-meeting-name" {
			t.Errorf("message.name == %s, expected to-meeting-name", iccMessage.Name)
		}

		if string(iccMessage.Message) != `"klaus"` {
			t.Errorf("message.message == %s, expected klaus", iccMessage.Message)
		}
	})

	t.Run("Message not for me", func(t *testing.T) {
		if err := icc.Send(strings.NewReader(`{"channel_id":"server:1:2","name":"message-name","to_users":[3],"message":"hans"}`), 1); err != nil {
			t.Fatalf("sending message: %v", err)
		}

		done := make(chan struct{})
		var v interface{}
		go func() {
			if err := decoder.Decode(&v); err != nil {
				t.Errorf("decoding message for other user: %v", err)
			}
			close(done)
		}()

		timer := time.NewTimer(10 * time.Millisecond)
		defer timer.Stop()
		select {
		case <-done:
			t.Errorf("decoded an unexpected message: %v", v)
		case <-timer.C:
		}
	})

	cancel()
	err := <-receiveDone

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("receive returned an unexpected error: %v", err)
	}
}

// func TestApplause(t *testing.T) {
// 	backend := newBackendStrub()
// 	icc := New(context.Background(), backend)

// 	r, w := io.Pipe()
// 	decoder := json.NewDecoder(r)

// 	ctx, cancel := context.WithCancel(context.Background())
// 	defer cancel()

// 	receiveDone := make(chan error, 1)
// 	go func() {
// 		receiveDone <- icc.Receive(ctx, w, 2)
// 	}()

// 	t.Run("Receive no applause at start", func(t *testing.T) {
// 		var receive struct {
// 			Applause int `json:"applause"`
// 			Base     int `json:"base"`
// 		}
// 		if err := decoder.Decode(&receive); err != nil {
// 			t.Fatalf("decoding first message: %v", err)
// 		}

// 		if receive

// 	})
// }
