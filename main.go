package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/OpenSlides/openslides-autoupdate-service/pkg/auth"
	"github.com/OpenSlides/openslides-autoupdate-service/pkg/datastore"
	"github.com/OpenSlides/openslides-autoupdate-service/pkg/environment"
	messageBusRedis "github.com/OpenSlides/openslides-autoupdate-service/pkg/redis"
	"github.com/OpenSlides/openslides-icc-service/internal/applause"
	"github.com/OpenSlides/openslides-icc-service/internal/icchttp"
	"github.com/OpenSlides/openslides-icc-service/internal/icclog"
	"github.com/OpenSlides/openslides-icc-service/internal/notify"
	"github.com/OpenSlides/openslides-icc-service/internal/redis"
	"github.com/alecthomas/kong"
)

//go:generate  sh -c "go run main.go build-doc > environment.md"

var (
	envICCServicePort = environment.NewVariable("ICC_PORT", "9007", "Port on which the service listen on.")
	envICCRedisHost   = environment.NewVariable("ICC_REDIS_HOST", "localhost", "The host of the redis instance to save icc messages.")
	envICCRedisPort   = environment.NewVariable("ICC_REDIS_PORT", "6379", "The port of the redis instance to save icc messages.")
)

var cli struct {
	Run      struct{} `cmd:"" help:"Runs the service." default:"withargs"`
	BuildDoc struct{} `cmd:"" help:"Build the environment documentation."`
	Health   struct{} `cmd:"" help:"Runs a health check."`
}

func main() {
	ctx, cancel := environment.InterruptContext()
	defer cancel()

	kongCTX := kong.Parse(&cli, kong.UsageOnError())
	switch kongCTX.Command() {
	case "run":
		if err := contextDone(run(ctx)); err != nil {
			handleError(err)
			os.Exit(1)
		}

	case "build-doc":
		if err := contextDone(buildDocu()); err != nil {
			handleError(err)
			os.Exit(1)
		}

	case "health":
		if err := contextDone(health(ctx)); err != nil {
			handleError(err)
			os.Exit(1)
		}
	}
}

func run(ctx context.Context) error {
	lookup := new(environment.ForProduction)

	service, err := initService(lookup)
	if err != nil {
		return fmt.Errorf("init services: %w", err)
	}

	return service(ctx)
}

func buildDocu() error {
	lookup := new(environment.ForDocu)

	if _, err := initService(lookup); err != nil {
		return fmt.Errorf("init services: %w", err)
	}

	doc, err := lookup.BuildDoc()
	if err != nil {
		return fmt.Errorf("build doc: %w", err)
	}

	fmt.Println(doc)
	return nil
}

func health(ctx context.Context) error {
	port, found := os.LookupEnv("ICC_PORT")
	if !found {
		port = "9007"
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "http://localhost:"+port+"/system/icc/health", nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("sending request: %w", err)
	}

	if resp.StatusCode != 200 {
		return fmt.Errorf("health returned status %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading response body: %w", err)
	}

	expect := `{"healthy": true}`
	got := strings.TrimSpace(string(body))
	if got != expect {
		return fmt.Errorf("got `%s`, expected `%s`", body, expect)
	}

	return nil
}

// initService initializes all packages needed for the icc service.
//
// Returns a the service as callable.
func initService(lookup environment.Environmenter) (func(context.Context) error, error) {
	var backgroundTasks []func(context.Context, func(error))
	listenAddr := ":" + envICCServicePort.Value(lookup)

	// Redis as message bus for datastore and logout events.
	messageBus := messageBusRedis.New(lookup)

	// Datastore Service.
	datastoreService, dsBackground, err := datastore.New(lookup, messageBus)
	if err != nil {
		return nil, fmt.Errorf("init datastore: %w", err)
	}
	backgroundTasks = append(backgroundTasks, dsBackground)

	// Auth Service.
	authService, authBackground := auth.New(lookup, messageBus)
	backgroundTasks = append(backgroundTasks, authBackground)

	backend := redis.New(envICCRedisHost.Value(lookup) + ":" + envICCRedisPort.Value(lookup))

	notifyService, notifyBackground := notify.New(backend)
	backgroundTasks = append(backgroundTasks, notifyBackground)

	applauseService, applauseBackground := applause.New(backend, datastoreService)
	backgroundTasks = append(backgroundTasks, applauseBackground)

	service := func(ctx context.Context) error {
		for _, bg := range backgroundTasks {
			go bg(ctx, handleError)
		}

		// Start http server.
		fmt.Printf("Listen on %s\n", listenAddr)
		return Run(ctx, listenAddr, notifyService, applauseService, authService)
	}

	return service, nil
}

// Run starts a webserver
func Run(ctx context.Context, addr string, notifyService *notify.Notify, applauseService *applause.Applause, auth icchttp.Authenticater) error {
	mux := http.NewServeMux()
	icchttp.HandleHealth(mux)
	notify.HandleReceive(mux, notifyService, auth)
	notify.HandlePublish(mux, notifyService, auth)
	applause.HandleReceive(mux, applauseService, auth)
	applause.HandleSend(mux, applauseService, auth)

	srv := &http.Server{
		Addr:        addr,
		Handler:     mux,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	// Shutdown logic in separate goroutine.
	wait := make(chan error)
	go func() {
		<-ctx.Done()
		if err := srv.Shutdown(context.Background()); err != nil {
			wait <- fmt.Errorf("HTTP server shutdown: %w", err)
			return
		}
		wait <- nil
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("HTTP Server failed: %v", err)
	}

	return <-wait
}

// contextDone returns an empty error if the context is done or exceeded
func contextDone(err error) error {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return nil
	}
	return err
}

// handleError handles an error.
//
// Ignores context closed errors.
func handleError(err error) {
	if contextDone(err) == nil {
		return
	}

	icclog.Info("Error: %v", err)
}
