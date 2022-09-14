package applause_test

import "context"

type applauserStub struct {
	expectedErr     error
	called          bool
	calledUserID    int
	calledMeetingID int
}

func (s *applauserStub) Send(ctx context.Context, meetingID, uid int) error {
	s.called = true
	s.calledUserID = uid
	s.calledMeetingID = meetingID
	return s.expectedErr
}

type backendStub struct {
	PublishCalled int
	ExpectSince   map[int]int
}

func (b *backendStub) ApplausePublish(meetingID, userID int, time int64) error {
	b.PublishCalled++
	return nil
}

func (b backendStub) ApplauseSince(time int64) (map[int]int, error) {
	return b.ExpectSince, nil
}
