package notify_test

import (
	"context"
	"fmt"
	"io"
)

type receiverStub struct {
	expectedMessage  string
	expectedErr      error
	called           bool
	callledMeetingID int
}

func (r *receiverStub) Receive(ctx context.Context, w io.Writer, meetingID, uid int) error {
	r.called = true
	r.callledMeetingID = meetingID

	if _, err := w.Write([]byte(r.expectedMessage)); err != nil {
		return fmt.Errorf("writing first message: %w", err)
	}

	return r.expectedErr
}

type senderStub struct {
	expectedErr  error
	called       bool
	calledUserID int
}

func (s *senderStub) Send(r io.Reader, uid int) error {
	s.called = true
	s.calledUserID = uid
	return s.expectedErr
}

type backendStub struct {
	messages         chan []byte
	receivedMessages [][]byte
}

func newBackendStrub() *backendStub {
	b := backendStub{}
	b.messages = make(chan []byte, 10)
	return &b
}

func (b *backendStub) reset() {
	b.receivedMessages = b.receivedMessages[0:0]
	for {
		select {
		case <-b.messages:
		default:
			return
		}
	}
}

func (b *backendStub) SendNotify(bs []byte) error {
	b.messages <- bs
	b.receivedMessages = append(b.receivedMessages, bs)
	return nil
}

func (b *backendStub) ReceiveNotify(ctx context.Context) (message []byte, err error) {
	select {
	case m := <-b.messages:
		return m, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
