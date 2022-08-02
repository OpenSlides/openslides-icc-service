package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"strconv"

	"os"
	"os/signal"

	"github.com/OpenSlides/openslides-icc-service/internal/icclog"
	"github.com/OpenSlides/openslides-icc-service/internal/run"
	"golang.org/x/sys/unix"
)

func main() {
	ctx, cancel := interruptContext()
	defer cancel()

	icclog.SetInfoLogger(log.Default())
	if dev, _ := strconv.ParseBool(os.Getenv("OPENSLIDES_DEVELOPMENT")); dev {
		icclog.SetDebugLogger(log.New(os.Stderr, "DEBUG ", log.LstdFlags))
	}

	if err := run.Run(ctx, os.Environ(), secret); err != nil {
		icclog.Info("Error: %v", err)
	}
}

// interruptContext works like signal.NotifyContext. It returns a context that
// is canceled, when a signal is received.
//
// It listens on os.Interrupt and unix.SIGTERM. If the signal is received two
// times, os.Exit(2) is called.
func interruptContext() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt, unix.SIGTERM)
		<-sig
		cancel()
		<-sig
		os.Exit(2)
	}()
	return ctx, cancel
}

func secret(name string) (string, error) {
	f, err := os.Open("/run/secrets/" + name)
	if err != nil {
		return "", err
	}

	secret, err := io.ReadAll(f)
	if err != nil {
		return "", fmt.Errorf("reading `/run/secrets/%s`: %w", name, err)
	}

	return string(secret), nil
}
