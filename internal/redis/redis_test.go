package redis_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/OpenSlides/openslides-icc-service/internal/redis"
	"github.com/ory/dockertest/v3"
)

func startRedis(t *testing.T) (string, func()) {
	t.Helper()

	pool, err := dockertest.NewPool("")
	if err != nil {
		t.Fatalf("Could not connect to docker: %s", err)
	}

	resource, err := pool.Run("redis", "6.2", nil)
	if err != nil {
		t.Fatalf("Could not start redis container: %s", err)
	}

	return resource.GetPort("6379/tcp"), func() {
		if err = pool.Purge(resource); err != nil {
			t.Fatalf("Could not purge redis container: %s", err)
		}
	}
}

func TestICC(t *testing.T) {
	port, stopRedis := startRedis(t)
	defer stopRedis()

	r := redis.New("localhost:" + port)
	r.Wait(context.Background())

	t.Run("Receive blocks", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan error)
		go func() {
			_, err := r.NotifyReceive(ctx)
			done <- err
		}()

		timer := time.NewTimer(10 * time.Millisecond)
		defer timer.Stop()
		select {
		case err := <-done:
			t.Errorf("ReceiveICC returned with error: %v. Expected it to block.", err)
		case <-timer.C:
		}
	})

	t.Run("Receive unblocks on cancel", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		done := make(chan error)
		go func() {
			_, err := r.NotifyReceive(ctx)
			done <- err
		}()

		cancel()

		timer := time.NewTimer(10 * time.Millisecond)
		defer timer.Stop()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("ReceiveICC returned with error: %v, expected context.Canceled", err)
			}
		case <-timer.C:
			t.Errorf("ReceiveICC did not unblock after context was canceled.")
		}
	})

	t.Run("Receive gets a send message", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		type receiveReturn struct {
			message []byte
			err     error
		}

		done := make(chan receiveReturn)
		go func() {
			message, err := r.NotifyReceive(ctx)
			done <- receiveReturn{message, err}
		}()

		// Wait for ReceiveICC to be called.
		time.Sleep(10 * time.Millisecond)

		r.NotifyPublish([]byte("my message"))

		timer := time.NewTimer(50 * time.Millisecond)
		defer timer.Stop()

		select {
		case data := <-done:
			if err := data.err; err != nil {
				t.Errorf("ReceiveICC returned unexpected error: %v", err)
			}

			if string(data.message) != "my message" {
				t.Errorf("RecieveICC returned message `%s`, expected `my message`", data.message)
			}

		case <-timer.C:
			t.Errorf("ReceiveICC did not unblock after message was send.")
		}
	})

	t.Run("Receive empty applause", func(t *testing.T) {
		applause, err := r.ApplauseReceive(1000)

		if err != nil {
			t.Fatalf("receiveApplause returned unexpected error: %v", err)
		}

		if applause != 0 {
			t.Errorf("receiveApplause returned %d, expected 0", applause)
		}
	})

	t.Run("Delete applause", func(t *testing.T) {
		if err := r.ApplausePublish(1, 10); err != nil {
			t.Fatalf("sending applause: %v", err)
		}

		if err := r.ApplauseCleanOld(100); err != nil {
			t.Fatalf("deleting old applause: %v", err)
		}

		applause, err := r.ApplauseReceive(10)

		if err != nil {
			t.Fatalf("receiveApplause returned unexpected error: %v", err)
		}

		if applause != 0 {
			t.Errorf("receiveApplause returned %d, expected 0", applause)
		}
	})

	t.Run("Delete not new applause", func(t *testing.T) {
		defer r.ApplauseCleanOld(1000)

		if err := r.ApplausePublish(1, 10); err != nil {
			t.Fatalf("sending applause: %v", err)
		}

		if err := r.ApplauseCleanOld(10); err != nil {
			t.Fatalf("deleting old applause: %v", err)
		}

		applause, err := r.ApplauseReceive(10)

		if err != nil {
			t.Fatalf("receiveApplause returned unexpected error: %v", err)
		}

		if applause != 1 {
			t.Errorf("receiveApplause returned %d, expected 1", applause)
		}
	})

	t.Run("Receive applause for one user", func(t *testing.T) {
		defer r.ApplauseCleanOld(1000)

		if err := r.ApplausePublish(1, 10); err != nil {
			t.Fatalf("sending applause: %v", err)
		}

		applause, err := r.ApplauseReceive(10)

		if err != nil {
			t.Fatalf("receiveApplause returned unexpected error: %v", err)
		}

		if applause != 1 {
			t.Errorf("receiveApplause returned %d, expected 1", applause)
		}
	})

	t.Run("Receive applause for one user twice", func(t *testing.T) {
		defer r.ApplauseCleanOld(1000)

		if err := r.ApplausePublish(1, 10); err != nil {
			t.Fatalf("sending applause: %v", err)
		}

		if err := r.ApplausePublish(1, 11); err != nil {
			t.Fatalf("sending applause: %v", err)
		}

		applause, err := r.ApplauseReceive(10)

		if err != nil {
			t.Fatalf("receiveApplause returned unexpected error: %v", err)
		}

		if applause != 1 {
			t.Errorf("receiveApplause returned %d, expected 1", applause)
		}
	})

	t.Run("Receive applause for one user to old", func(t *testing.T) {
		defer r.ApplauseCleanOld(1000)

		if err := r.ApplausePublish(1, 9); err != nil {
			t.Fatalf("sending applause: %v", err)
		}

		applause, err := r.ApplauseReceive(10)

		if err != nil {
			t.Fatalf("receiveApplause returned unexpected error: %v", err)
		}

		if applause != 0 {
			t.Errorf("receiveApplause returned %d, expected 0", applause)
		}
	})

	t.Run("Receive applause for two users", func(t *testing.T) {
		defer r.ApplauseCleanOld(1000)

		if err := r.ApplausePublish(1, 10); err != nil {
			t.Fatalf("sending applause: %v", err)
		}

		if err := r.ApplausePublish(2, 10); err != nil {
			t.Fatalf("sending applause: %v", err)
		}

		applause, err := r.ApplauseReceive(10)

		if err != nil {
			t.Fatalf("receiveApplause returned unexpected error: %v", err)
		}

		if applause != 2 {
			t.Errorf("receiveApplause returned %d, expected 2", applause)
		}
	})
}
