package applause_test

import (
	"context"
	"errors"
	"testing"

	"github.com/OpenSlides/openslides-go/datastore/dsmock"
	"github.com/OpenSlides/openslides-icc-service/internal/applause"
	"github.com/OpenSlides/openslides-icc-service/internal/iccerror"
)

func TestApplauseCanReceiveInMeeting(t *testing.T) {
	ctx := context.Background()

	t.Run("Meeting does not exist", func(t *testing.T) {
		backend := new(backendStub)
		ds := dsmock.Stub(dsmock.YAMLData(`---
		user/5/id: 5
		`))
		app, _ := applause.New(backend, ds)

		err := app.CanReceive(ctx, 1, 5)

		if !errors.Is(err, iccerror.ErrNotAllowed) {
			t.Errorf("Got error `%v`, expected `%v`", err, iccerror.ErrNotAllowed)
		}
	})

	t.Run("Can see livestream", func(t *testing.T) {
		backend := new(backendStub)
		ds := dsmock.Stub(dsmock.YAMLData(`---
		user/5/meeting_user_ids: [50]
		meeting_user/50:
			meeting_id: 1
			user_id: 5
			group_ids: [13]
		group/13/permissions: [meeting.can_see_livestream]
		meeting/1/admin_group_id: 1
		`))
		app, _ := applause.New(backend, ds)

		err := app.CanReceive(ctx, 1, 5)

		if err != nil {
			t.Errorf("Got error `%v`, expected `nil`", err)
		}
	})

	t.Run("Can not see livestream", func(t *testing.T) {
		backend := new(backendStub)
		ds := dsmock.Stub(dsmock.YAMLData(`---
		user/5/meeting_user_ids: [50]
		meeting_user/50:
			meeting_id: 1
			user_id: 5
			group_ids: [13]
		group/13/permissions: []
		meeting/1/admin_group_id: 1
		`))
		app, _ := applause.New(backend, ds)

		err := app.CanReceive(ctx, 1, 5)

		if !errors.Is(err, iccerror.ErrNotAllowed) {
			t.Errorf("Got error `%v`, expected `%v`", err, iccerror.ErrNotAllowed)
		}
	})
}
