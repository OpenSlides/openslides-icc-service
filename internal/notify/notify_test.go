package notify_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/OpenSlides/openslides-icc-service/internal/iccerror"
	"github.com/OpenSlides/openslides-icc-service/internal/notify"
)

func TestSend(t *testing.T) {
	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend := newBackendStrub()
	n, bg := notify.New(backend)
	go bg(shutdownCtx, nil)

	t.Run("invalid json", func(t *testing.T) {
		defer backend.reset()

		err := n.Publish(strings.NewReader(`{123`), 1)

		if !errors.Is(err, iccerror.ErrInvalid) {
			t.Errorf("send() returned err `%s`, expected `%s`", err, iccerror.ErrInvalid.Error())
		}
	})

	t.Run("invalid format", func(t *testing.T) {
		defer backend.reset()

		err := n.Publish(strings.NewReader(`{"to_users":1,"message":"hans"}`), 1)

		if !errors.Is(err, iccerror.ErrInvalid) {
			t.Errorf("send() returned err `%s`, expected `%s`", err, iccerror.ErrInvalid.Error())
		}
	})

	t.Run("no channel_id", func(t *testing.T) {
		defer backend.reset()

		err := n.Publish(strings.NewReader(`
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

		err := n.Publish(strings.NewReader(`
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

		err := n.Publish(strings.NewReader(`
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

		err := n.Publish(strings.NewReader(`
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
	shutdownCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	backend := newBackendStrub()
	n, bg := notify.New(backend)
	go bg(shutdownCtx, nil)

	_, next := n.Receive(1, 2)

	t.Run("Get first message", func(t *testing.T) {
		if err := n.Publish(strings.NewReader(`{"channel_id":"server:1:2","name":"message-name","to_users":[2],"message":"hans"}`), 1); err != nil {
			t.Fatalf("sending message: %v", err)
		}

		notifyMessage, err := next(context.Background())
		if err != nil {
			t.Fatalf("Next() returned: %v", err)
		}

		if notifyMessage.SenderUserID != 1 {
			t.Errorf("message.sender_user_id == %d, expected 1", notifyMessage.SenderUserID)
		}

		if notifyMessage.SenderChannelID != "server:1:2" {
			t.Errorf("message.sender_channel_id == %s, expected server:1:2", notifyMessage.SenderChannelID)
		}

		if notifyMessage.Name != "message-name" {
			t.Errorf("message.name == %s, expected message-name", notifyMessage.Name)
		}

		if string(notifyMessage.Message) != `"hans"` {
			t.Errorf("message.message == %s, expected hans", notifyMessage.Message)
		}
	})

	t.Run("Message for meeting", func(t *testing.T) {
		if err := n.Publish(strings.NewReader(`{"channel_id":"server:1:2","name":"to-meeting-name","to_meeting":1,"message":"klaus"}`), 1); err != nil {
			t.Fatalf("sending message: %v", err)
		}

		notifyMessage, err := next(context.Background())
		if err != nil {
			t.Fatalf("Next() returned: %v", err)
		}

		if notifyMessage.Name != "to-meeting-name" {
			t.Errorf("message.name == %s, expected to-meeting-name", notifyMessage.Name)
		}

		if string(notifyMessage.Message) != `"klaus"` {
			t.Errorf("message.message == %s, expected klaus", notifyMessage.Message)
		}
	})

	t.Run("Message not for me", func(t *testing.T) {
		if err := n.Publish(strings.NewReader(`{"channel_id":"server:1:2","name":"message-name","to_users":[3],"message":"hans"}`), 1); err != nil {
			t.Fatalf("sending message: %v", err)
		}

		done := make(chan error)
		go func() {
			_, err := next(context.Background())
			done <- err
		}()

		timer := time.NewTimer(10 * time.Millisecond)
		defer timer.Stop()
		select {
		case err := <-done:
			if err != nil {
				t.Fatalf("Next returned unexpected error: %v", err)
			}
			t.Errorf("decoded an unexpected message")
		case <-timer.C:
		}
	})
}
