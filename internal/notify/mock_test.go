package notify_test

import (
	"context"
	"io"

	"github.com/OpenSlides/openslides-icc-service/internal/notify"
)

type messageProviderStub struct {
	nm  chan notify.OutMessage
	err chan error
}

func newMessageProviderStub() *messageProviderStub {
	return &messageProviderStub{
		nm:  make(chan notify.OutMessage, 1),
		err: make(chan error, 1),
	}
}

func (mp *messageProviderStub) Next(ctx context.Context) (notify.OutMessage, error) {
	select {
	case m := <-mp.nm:
		return m, nil
	case err := <-mp.err:
		return notify.OutMessage{}, err
	case <-ctx.Done():
		return notify.OutMessage{}, ctx.Err()
	}
}

func (mp *messageProviderStub) Send(m notify.OutMessage) {
	mp.nm <- m
}

func (mp *messageProviderStub) SendError(err error) {
	mp.err <- err
}

type receiverStub struct {
	cid string
	nm  notify.NextMessage

	called           bool
	callledMeetingID int
}

func (r *receiverStub) Receive(meetingID, uid int) (cid string, nm notify.NextMessage) {
	r.called = true
	r.callledMeetingID = meetingID

	return r.cid, r.nm
}

type publisherStub struct {
	expectedErr  error
	called       bool
	calledUserID int
}

func (s *publisherStub) Publish(r io.Reader, uid int) error {
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

func (b *backendStub) NotifyPublish(bs []byte) error {
	b.messages <- bs
	b.receivedMessages = append(b.receivedMessages, bs)
	return nil
}

func (b *backendStub) NotifyReceive(ctx context.Context) (message []byte, err error) {
	select {
	case m := <-b.messages:
		return m, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
