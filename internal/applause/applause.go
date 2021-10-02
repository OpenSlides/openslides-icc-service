package applause

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/OpenSlides/openslides-autoupdate-service/pkg/datastore"
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
	topic     *topic.Topic
	datastore datastore.Getter
}

// New returns an initialized state of the notify service.
//
// The New function is not blocking. The context is used to stop a goroutine
// that is started by this function.
func New(b Backend, db datastore.Getter, closed <-chan struct{}) *Applause {
	notify := Applause{
		backend:   b,
		topic:     topic.New(topic.WithClosed(closed)),
		datastore: db,
	}

	return &notify
}

// MSG contians the current applause level and number of present users.
type MSG struct {
	Level        int `json:"level"`
	PresentUsers int `json:"present_users"`
}

// Send registers, that a user applaused in a meeting.
func (a *Applause) Send(meetingID, userID int) error {
	if err := a.backend.ApplausePublish(meetingID, userID, time.Now().Unix()); err != nil {
		return fmt.Errorf("publish applause in backend: %w", err)
	}
	return nil
}

// Receive returns the applause for a given meeting.
func (a *Applause) Receive(ctx context.Context, tid uint64, meetingID int) (newTID uint64, msg MSG, err error) {
	// TODO: Test that this does not return, if there is a message in another meeting
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

			presentUser, err := a.presentUser(ctx, meetingID)
			if err != nil {
				errHandler(fmt.Errorf("getting present Users: %w", err))
				continue
			}

			message[meetingID] = MSG{
				level,
				presentUser,
			}
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

func (a *Applause) presentUser(ctx context.Context, meetingID int) (int, error) {
	fetch := datastore.NewFetcher(a.datastore)
	ids := fetch.Field().Meeting_PresentUserIDs(ctx, meetingID)
	if err := fetch.Err(); err != nil {
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
