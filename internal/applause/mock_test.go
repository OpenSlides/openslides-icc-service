package applause_test

import "context"

type applauserStrub struct {
	expectedErr     error
	called          bool
	calledUserID    int
	calledMeetingID int
}

func (s *applauserStrub) Send(ctx context.Context, meetingID, uid int) error {
	s.called = true
	s.calledUserID = uid
	s.calledMeetingID = meetingID
	return s.expectedErr
}
