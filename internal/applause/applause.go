package applause

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/OpenSlides/openslides-autoupdate-service/pkg/datastore"
	"github.com/OpenSlides/openslides-icc-service/internal/iccerror"
	"github.com/ostcar/topic"
)

const (
	applauseInterval = time.Second
	countTime        = 5 * time.Second
	pruneTime        = 10 * time.Minute
)

// Backend stores the applause messages.
type Backend interface {
	// ApplausePublish adds the applause from a user to a meeting.
	//
	// The function can be called many times. The implementation of the
	// interface has to make sure, that the applause is only counted once.
	ApplausePublish(meetingID, userID int, time int64) error

	// ApplauseSince returns the number of applause for each meeting since
	// `time`
	ApplauseSince(time int64) (map[int]int, error)
}

// Applause holds the state of the service.
type Applause struct {
	backend   Backend
	topic     *topic.Topic[string]
	datastore datastore.Getter
}

// New returns an initialized state of the notify service.
//
// The New function is not blocking. The context is used to stop a goroutine
// that is started by this function.
func New(b Backend, db datastore.Getter, closed <-chan struct{}) *Applause {
	notify := Applause{
		backend:   b,
		topic:     topic.New(topic.WithClosed[string](closed)),
		datastore: db,
	}

	// Make sure the topic is not empty.
	notify.topic.Publish("")

	return &notify
}

// MSG contians the current applause level and number of present users.
type MSG struct {
	Level        int `json:"level"`
	PresentUsers int `json:"present_users"`
}

// Send registers, that a user applaused in a meeting.
func (a *Applause) Send(ctx context.Context, meetingID, userID int) error {
	if userID == 0 {
		return iccerror.NewMessageError(iccerror.ErrNotAllowed, "Anonymous is not allowed to applause. Please be quiet.")
	}

	fetcher := datastore.NewRequest(a.datastore)

	applauseEnabled, err := fetcher.Meeting_ApplauseEnable(meetingID).Value(ctx)
	if err != nil {
		return fmt.Errorf("fetching applause enabled: %w", err)
	}

	if !applauseEnabled {
		return iccerror.NewMessageError(iccerror.ErrNotAllowed, "applause is not enabled in meeting %d. Please be quiet.", meetingID)
	}

	meetingUserIDs, err := fetcher.Meeting_UserIDs(meetingID).Value(ctx)
	if err != nil {
		return fmt.Errorf("fetching meeting users: %w", err)
	}

	var inMeeting bool
	for _, u := range meetingUserIDs {
		if u == userID {
			inMeeting = true
			break
		}
	}

	if !inMeeting {
		return iccerror.NewMessageError(iccerror.ErrNotAllowed, "You are not part of meeting %d. Please be quiet.", meetingID)
	}

	if err := a.backend.ApplausePublish(meetingID, userID, time.Now().Unix()); err != nil {
		return fmt.Errorf("publish applause in backend: %w", err)
	}
	return nil
}

// CanReceive returns an error, if the user can not receive applause.
func (a *Applause) CanReceive(ctx context.Context, meetingID, userID int) error {
	fetcher := datastore.NewRequest(a.datastore)
	if userID == 0 {
		anonymousEnabled, err := fetcher.Meeting_EnableAnonymous(meetingID).Value(ctx)
		if err != nil {
			return fmt.Errorf("fetching anonymous enabled: %w", err)

		}
		if !anonymousEnabled {
			return iccerror.NewMessageError(iccerror.ErrNotAllowed, "Anonymous is not enabled")
		}
		return nil
	}

	meetingUserIDs, err := fetcher.Meeting_UserIDs(meetingID).Value(ctx)
	if err != nil {
		return fmt.Errorf("fetching meeting users: %w", err)
	}

	var inMeeting bool
	for _, u := range meetingUserIDs {
		if u == userID {
			inMeeting = true
			break
		}
	}

	if !inMeeting {
		return iccerror.NewMessageError(iccerror.ErrNotAllowed, "You are not part of meeting %d.", meetingID)
	}
	return nil
}

// Receive returns the applause for a given meeting.
func (a *Applause) Receive(ctx context.Context, tid uint64, meetingID int) (newTID uint64, msg MSG, err error) {
	if tid == 0 {
		present, err := a.presentUser(ctx, meetingID)
		if err != nil {
			return 0, MSG{}, fmt.Errorf("fetching present user: %w", err)
		}
		return a.topic.LastID(), MSG{0, present}, nil
	}

	for {
		var messages []string
		tid, messages, err = a.topic.Receive(ctx, tid)
		if err != nil {
			return 0, MSG{}, fmt.Errorf("receiving message from topic: %w", err)
		}

		// We are intressted in the last messaeg that has a entry for out
		// meeting. We go backwards throw the messages and return, if we find
		// something.
		for i := len(messages) - 1; i >= 0; i-- {
			var message map[int]MSG
			if err := json.Unmarshal([]byte(messages[i]), &message); err != nil {
				return 0, MSG{}, fmt.Errorf("decoding message from topic: %w", err)
			}
			if meetingData, ok := message[meetingID]; ok {
				return tid, meetingData, nil
			}
		}
	}
}

// LastID returns the newest id from the topic.
func (a *Applause) LastID() uint64 {
	return a.topic.LastID()
}

// Loop fetches the applause from the backend and saves it for the clients to
// fetch.
func (a *Applause) Loop(ctx context.Context, errHandler func(error)) {
	if errHandler == nil {
		errHandler = func(error) {}
	}

	lastApplause := make(map[int]int)

	for {
		if err := contextSleep(ctx, applauseInterval); err != nil {
			return
		}

		d := time.Now().Add(-countTime)
		applause, err := a.backend.ApplauseSince(d.Unix())
		if err != nil {
			errHandler(fmt.Errorf("fetching applause: %w", err))
			continue
		}

		// Set values that are in lastApplause but not in applause to 0.
		for k := range lastApplause {
			if _, ok := applause[k]; !ok {
				applause[k] = 0
			}
		}

		message := make(map[int]MSG)
		for meetingID, level := range applause {
			if lastApplause[meetingID] == level {
				continue
			}
			lastApplause[meetingID] = level

			msg, err := a.toMSG(ctx, meetingID, level)
			if err != nil {
				errHandler(fmt.Errorf("converting level to MSG: %w", err))
				continue
			}

			message[meetingID] = msg
		}

		if len(message) == 0 {
			continue
		}

		b, err := json.Marshal(message)
		if err != nil {
			errHandler(fmt.Errorf("encoding message: %w", err))
			continue
		}
		a.topic.Publish(string(b))
	}
}

// toMSG converts a int (applause level) to a MSG object.
func (a *Applause) toMSG(ctx context.Context, meetingID, level int) (MSG, error) {
	presentUser, err := a.presentUser(ctx, meetingID)
	if err != nil {
		return MSG{}, fmt.Errorf("getting present Users: %w", err)
	}

	return MSG{
		level,
		presentUser,
	}, nil
}

// PruneOldData removes applause data.
func (a *Applause) PruneOldData(ctx context.Context) {
	tick := time.NewTicker(5 * time.Minute)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-tick.C:
			a.topic.Prune(time.Now().Add(-pruneTime))
		}
	}
}

// presentUser returns the number of users in this meeting.
func (a *Applause) presentUser(ctx context.Context, meetingID int) (int, error) {
	fetch := datastore.NewRequest(a.datastore)
	ids, err := fetch.Meeting_PresentUserIDs(meetingID).Value(ctx)
	if err != nil {
		var errDoesNotExist datastore.DoesNotExistError
		if !errors.As(err, &errDoesNotExist) {
			return 0, fmt.Errorf("get present users for meeting %d: %w", meetingID, err)
		}
	}
	return len(ids), nil
}

// contextSleep is like time.Sleep but also takes a context.
//
// It returns either when the time is up.
//
// Returns ctx.Err() if the context was canceled.
func contextSleep(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
