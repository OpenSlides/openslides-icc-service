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

	t.Run("Anonyomus disabled", func(t *testing.T) {
		backend := new(backendStub)
		ds := dsmock.Stub(dsmock.YAMLData(`---
		meeting/1/enable_anonymous: false
		`))
		app, _ := applause.New(backend, ds)

		err := app.CanReceive(ctx, 1, 0)

		if !errors.Is(err, iccerror.ErrNotAllowed) {
			t.Errorf("Got error `%v`, expected `%v`", err, iccerror.ErrNotAllowed)
		}
	})

	t.Run("Anonyomus enabled", func(t *testing.T) {
		backend := new(backendStub)
		ds := dsmock.Stub(dsmock.YAMLData(`---
		meeting/1/enable_anonymous: true
		`))
		app, _ := applause.New(backend, ds)

		err := app.CanReceive(ctx, 1, 0)

		if err != nil {
			t.Errorf("Got error `%v`, expected `nil`", err)
		}
	})

	t.Run("Not in meeting", func(t *testing.T) {
		backend := new(backendStub)
		ds := dsmock.Stub(dsmock.YAMLData(`---
		meeting/1/enable_anonymous: false

		user/5/id: 5
		`))
		app, _ := applause.New(backend, ds)

		err := app.CanReceive(ctx, 1, 5)

		if !errors.Is(err, iccerror.ErrNotAllowed) {
			t.Errorf("Got error `%v`, expected `%v`", err, iccerror.ErrNotAllowed)
		}
	})

	t.Run("User in Meeting", func(t *testing.T) {
		backend := new(backendStub)
		ds := dsmock.Stub(dsmock.YAMLData(`---
		meeting/1/enable_anonymous: false

		user/5/meeting_user_ids: [50]
		meeting_user/50:
			meeting_id: 1
			user_id: 5
		`))
		app, _ := applause.New(backend, ds)

		err := app.CanReceive(ctx, 1, 5)

		if err != nil {
			t.Errorf("Got error `%v`, expected `nil`", err)
		}
	})

	t.Run("Superadmin", func(t *testing.T) {
		backend := new(backendStub)
		ds := dsmock.Stub(dsmock.YAMLData(`---
		meeting/1/enable_anonymous: false

		user/5/organization_management_level: superadmin
		`))
		app, _ := applause.New(backend, ds)

		err := app.CanReceive(ctx, 1, 5)

		if err != nil {
			t.Errorf("Got error `%v`, expected `nil`", err)
		}
	})
}
