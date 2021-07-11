package notify

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/OpenSlides/openslides-icc-service/internal/iccerror"
	"github.com/OpenSlides/openslides-icc-service/internal/icclog"
	"github.com/ostcar/topic"
)

// Backend stores the icc messages.
type Backend interface {
	// SendICC saves a valid icc message.
	SendICC([]byte) error

	// ReceiveICC is a blocking function that receives the messages.
	//
	// The first call returnes the first icc message, the next call the second
	// an so on. If there are no more messages to read, the function blocks
	// until there is or the context ist canceled.
	//
	// It is expected, that only one goroutine is calling this function. The
	// Backend keeps track what the last send message was.
	ReceiveICC(ctx context.Context) (message []byte, err error)
}

// ICC holds the state of the service.
type ICC struct {
	backend Backend
	cIDGen  cIDGen
	topic   *topic.Topic
}

// New returns an initialized state of the service.
//
// The New function is not blocking. The context is used to stop a goroutine
// that is started by this function.
func New(ctx context.Context, b Backend) *ICC {
	icc := ICC{
		backend: b,
		topic:   topic.New(topic.WithClosed(ctx.Done())),
	}

	go icc.listen(ctx)
	return &icc
}

// listen waits for ICCges from the backend and saves them into the
// topic.
func (icc *ICC) listen(ctx context.Context) {
	for {
		m, err := icc.backend.ReceiveICC(ctx)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}

			icclog.Info("Error: can not receive data from backend: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		icclog.Debug("Found icc message: `%s`", m)
		icc.topic.Publish(string(m))
	}
}

type flusher interface {
	Flush()
}

// Receive is a blocking function that sends data to w as soon as an icc event
// happens.
func (icc *ICC) Receive(ctx context.Context, w io.Writer, meetingID, uid int) error {
	cid := icc.cIDGen.generate(uid)

	if _, err := fmt.Fprintf(w, `{"channel_id": "%s"}`+"\n", cid); err != nil {
		return fmt.Errorf("sending channel id: %w", err)
	}

	if f, ok := w.(flusher); ok {
		f.Flush()
	}

	encoder := json.NewEncoder(w)
	tid := icc.topic.LastID()
	var messages []string
	var err error
	for {
		tid, messages, err = icc.topic.Receive(ctx, tid)
		if err != nil {
			return fmt.Errorf("fetching message from topic: %w", err)
		}

		for _, message := range messages {
			var m iccMessage
			if err := json.Unmarshal([]byte(message), &m); err != nil {
				return fmt.Errorf("decoding message: %w", err)
			}

			if !m.forMe(meetingID, uid, cid) {
				continue
			}

			out := struct {
				SenderUserID    int             `json:"sender_user_id"`
				SenderChannelID string          `json:"sender_channel_id"`
				Name            string          `json:"name"`
				Message         json.RawMessage `json:"message"`
			}{
				m.ChannelID.uid(),
				m.ChannelID.String(),
				m.Name,
				m.Message,
			}

			if err := encoder.Encode(out); err != nil {
				return fmt.Errorf("sending message: %w", err)
			}

		}

		if f, ok := w.(flusher); ok {
			f.Flush()
		}
	}
}

// Send reads and saves the icc event from the given reader.
func (icc *ICC) Send(r io.Reader, uid int) error {
	var message iccMessage
	if err := json.NewDecoder(r).Decode(&message); err != nil {
		return iccerror.NewMessageError(iccerror.ErrInvalid, "invalid json: %v", err)
	}

	if err := validateMessage(message, uid); err != nil {
		return fmt.Errorf("validate message: %w", err)
	}

	bs, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("can not marshal icc message: %v", err)
	}

	icclog.Debug("Saving icc message: `%s`", bs)
	if err := icc.backend.SendICC(bs); err != nil {
		return fmt.Errorf("sending message to backend: %w", err)
	}

	return nil
}

func validateMessage(message iccMessage, userID int) error {
	if message.ChannelID.uid() != userID {
		return iccerror.NewMessageError(iccerror.ErrInvalid, "invalid channel id `%s`", message.ChannelID)
	}

	if message.Name == "" {
		return iccerror.NewMessageError(iccerror.ErrInvalid, "icc message does not have required field `name`")
	}

	return nil
}

type iccMessage struct {
	ChannelID  channelID       `json:"channel_id"`
	ToMeeting  int             `json:"to_meeting,omitempty"`
	ToUsers    []int           `json:"to_users,omitempty"`
	ToChannels []string        `json:"to_channels,omitempty"`
	Name       string          `json:"name"`
	Message    json.RawMessage `json:"message"`
}

func (m iccMessage) forMe(meetingID, uid int, cID channelID) bool {
	if m.ToMeeting != 0 && m.ToMeeting == meetingID {
		return true
	}

	for _, toUID := range m.ToUsers {
		if toUID == uid {
			return true
		}
	}

	for _, toCID := range m.ToChannels {
		if toCID == cID.String() {
			return true
		}
	}
	return false
}
