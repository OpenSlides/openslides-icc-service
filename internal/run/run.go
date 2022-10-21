package run

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/OpenSlides/openslides-autoupdate-service/pkg/auth"
	"github.com/OpenSlides/openslides-autoupdate-service/pkg/datastore"
	messageBusRedis "github.com/OpenSlides/openslides-autoupdate-service/pkg/redis"
	"github.com/OpenSlides/openslides-icc-service/internal/applause"
	"github.com/OpenSlides/openslides-icc-service/internal/icchttp"
	"github.com/OpenSlides/openslides-icc-service/internal/icclog"
	"github.com/OpenSlides/openslides-icc-service/internal/notify"
	"github.com/OpenSlides/openslides-icc-service/internal/redis"
)

// Run starts the http server.
//
// The server is automaticly closed when ctx is done.
//
// The service is configured by the argument `environment`. It expect strings in
// the format `KEY=VALUE`, like the output from `os.Environmen()`.
func Run(ctx context.Context, environment []string) error {
	env := defaultEnv(environment)

	errHandler := func(err error) {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return
		}

		icclog.Info("Error: %v", err)
	}

	messageBus, err := buildMessageBus(env)
	if err != nil {
		return fmt.Errorf("building message bus: %w", err)
	}

	auth, authBackground, err := initAuth(
		env,
		messageBus,
		errHandler,
	)
	if err != nil {
		return fmt.Errorf("building auth: %w", err)
	}
	defer authBackground(ctx)

	ds, dsBackground, err := initDatastore(ctx, env, messageBus, errHandler)
	if err != nil {
		return fmt.Errorf("build datastore service: %w", err)
	}
	defer dsBackground(ctx)

	backend := redis.New(env["ICC_REDIS_HOST"] + ":" + env["ICC_REDIS_PORT"])

	notifyService := notify.New(backend)
	go notifyService.Listen(ctx)

	applauseService := applause.New(backend, ds)
	go applauseService.Loop(ctx, errHandler)
	go applauseService.PruneOldData(ctx)

	mux := http.NewServeMux()
	icchttp.HandleHealth(mux)
	notify.HandleReceive(mux, notifyService, auth)
	notify.HandlePublish(mux, notifyService, auth)
	applause.HandleReceive(mux, applauseService, auth)
	applause.HandleSend(mux, applauseService, auth)

	listenAddr := ":" + env["ICC_PORT"]
	srv := &http.Server{
		Addr:        listenAddr,
		Handler:     mux,
		BaseContext: func(net.Listener) context.Context { return ctx },
	}

	// Shutdown logic in separate goroutine.
	wait := make(chan error)
	go func() {
		// Wait for the context to be closed.
		<-ctx.Done()

		if err := srv.Shutdown(context.Background()); err != nil {
			wait <- fmt.Errorf("HTTP server shutdown: %w", err)
			return
		}
		wait <- nil
	}()

	icclog.Info("Listen on %s", listenAddr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		return fmt.Errorf("HTTP Server failed: %v", err)
	}

	return <-wait
}

// defaultEnv parses the environment (output from os.Environ()) and sets specific
// defaut values.
func defaultEnv(environment []string) map[string]string {
	env := map[string]string{
		"ICC_PORT": "9007",

		"ICC_REDIS_HOST": "localhost",
		"ICC_REDIS_PORT": "6379",

		"DATASTORE_DATABASE_HOST": "localhost",
		"DATASTORE_DATABASE_PORT": "5432",
		"DATASTORE_DATABASE_USER": "openslides",
		"DATASTORE_DATABASE_NAME": "openslides",

		"DATASTORE_READER_HOST":     "localhost",
		"DATASTORE_READER_PORT":     "9010",
		"DATASTORE_READER_PROTOCOL": "http",

		"MESSAGE_BUS_HOST": "localhost",
		"MESSAGE_BUS_PORT": "6379",
		"REDIS_TEST_CONN":  "true",

		"AUTH":          "fake",
		"AUTH_PROTOCOL": "http",
		"AUTH_HOST":     "localhost",
		"AUTH_PORT":     "9004",

		"OPENSLIDES_DEVELOPMENT": "false",
		"MAX_PARALLEL_KEYS":      "1000",
		"DATASTORE_TIMEOUT":      "3s",
	}

	for _, value := range environment {
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 {
			panic(fmt.Sprintf("Invalid value from environment(): %s", value))
		}

		env[parts[0]] = parts[1]
	}
	return env
}

func secret(env map[string]string, name string) ([]byte, error) {
	useDev, _ := strconv.ParseBool(env["OPENSLIDES_DEVELOPMENT"])

	if useDev {
		debugSecred := "openslides"
		switch name {
		case "auth_token_key":
			debugSecred = auth.DebugTokenKey
		case "auth_cookie_key":
			debugSecred = auth.DebugCookieKey
		}

		return []byte(debugSecred), nil
	}

	path := path.Join(env["SECRETS_PATH"], name)
	secret, err := os.ReadFile(path)
	if err != nil {
		// TODO EXTERMAL ERROR
		return nil, fmt.Errorf("reading `%s`: %w", path, err)
	}

	return secret, nil
}

func initAuth(env map[string]string, messageBus auth.LogoutEventer, errHandler func(error)) (icchttp.Authenticater, func(context.Context), error) {
	method := env["AUTH"]

	switch method {
	case "ticket":
		tokenKey, err := secret(env, "auth_token_key")
		if err != nil {
			return nil, nil, fmt.Errorf("getting token secret: %w", err)
		}

		cookieKey, err := secret(env, "auth_cookie_key")
		if err != nil {
			return nil, nil, fmt.Errorf("getting cookie secret: %w", err)
		}

		url := fmt.Sprintf("%s://%s:%s", env["AUTH_PROTOCOL"], env["AUTH_HOST"], env["AUTH_PORT"])
		a, err := auth.New(url, tokenKey, cookieKey)
		if err != nil {
			return nil, nil, fmt.Errorf("creating auth service: %w", err)
		}

		backgroundtask := func(ctx context.Context) {
			go a.ListenOnLogouts(ctx, messageBus, errHandler)
			go a.PruneOldData(ctx)
		}

		return a, backgroundtask, nil

	case "fake":
		fmt.Println("Auth Method: FakeAuth (User ID 1 for all requests)")
		return auth.Fake(1), func(context.Context) {}, nil

	default:
		// TODO LAST ERROR
		return nil, nil, fmt.Errorf("unknown auth method: %s", method)
	}
}

func initDatastore(ctx context.Context, env map[string]string, mb messageBus, handleError func(error)) (*datastore.Datastore, func(context.Context), error) {
	maxParallel, err := strconv.Atoi(env["MAX_PARALLEL_KEYS"])
	if err != nil {
		return nil, nil, fmt.Errorf("environment variable MAX_PARALLEL_KEYS has to be a number, not %s", env["MAX_PARALLEL_KEYS"])
	}

	timeout, err := parseDuration(env["DATASTORE_TIMEOUT"])
	if err != nil {
		return nil, nil, fmt.Errorf("environment variable DATASTORE_TIMEOUT has to be a duration like 3s, not %s: %w", env["DATASTORE_TIMEOUT"], err)
	}

	datastoreSource := datastore.NewSourceDatastore(
		env["DATASTORE_READER_PROTOCOL"]+"://"+env["DATASTORE_READER_HOST"]+":"+env["DATASTORE_READER_PORT"],
		mb,
		maxParallel,
		timeout,
	)

	password, err := secret(env, "postgres_password")
	if err != nil {
		return nil, nil, fmt.Errorf("getting postgres password: %w", err)
	}

	addr := fmt.Sprintf(
		"postgres://%s@%s:%s/%s",
		env["DATASTORE_DATABASE_USER"],
		env["DATASTORE_DATABASE_HOST"],
		env["DATASTORE_DATABASE_PORT"],
		env["DATASTORE_DATABASE_NAME"],
	)

	postgresSource, err := datastore.NewSourcePostgres(ctx, addr, string(password), datastoreSource)
	if err != nil {
		return nil, nil, fmt.Errorf("creating connection to postgres: %w", err)
	}

	ds := datastore.New(
		postgresSource,
		nil,
		datastoreSource,
	)

	background := func(ctx context.Context) {
		go ds.ListenOnUpdates(ctx, handleError)
	}

	return ds, background, nil
}

// authStub implements the authenticater interface. It allways returs the given
// user id.
type authStub int

// Authenticate does nothing.
func (a authStub) Authenticate(w http.ResponseWriter, r *http.Request) (context.Context, error) {
	return r.Context(), nil
}

// FromContext returns the uid the object was initialiced with.
func (a authStub) FromContext(ctx context.Context) int {
	return int(a)
}

type messageBus interface {
	auth.LogoutEventer
	datastore.Updater
}

func buildMessageBus(env map[string]string) (messageBus, error) {
	redisAddress := env["MESSAGE_BUS_HOST"] + ":" + env["MESSAGE_BUS_PORT"]
	conn := messageBusRedis.NewConnection(redisAddress)
	if env["REDIS_TEST_CONN"] == "true" {
		if err := conn.TestConn(); err != nil {
			return nil, fmt.Errorf("connect to redis: %w", err)
		}
	}

	return &messageBusRedis.Redis{Conn: conn}, nil
}

// buildDatastore configures the datastore service.
func buildDatastore(env map[string]string, updater datastore.Updater) (*datastore.Datastore, error) {
	protocol := env["DATASTORE_READER_PROTOCOL"]
	host := env["DATASTORE_READER_HOST"]
	port := env["DATASTORE_READER_PORT"]
	url := protocol + "://" + host + ":" + port

	maxParallel, err := strconv.Atoi(env["MAX_PARALLEL_KEYS"])
	if err != nil {
		return nil, fmt.Errorf("environmentvariable MAX_PARALLEL_KEYS has to be a number, not %s", env["MAX_PARALLEL_KEYS"])
	}

	timeout, err := parseDuration(env["DATASTORE_TIMEOUT"])
	if err != nil {
		return nil, fmt.Errorf("environment variable DATASTORE_TIMEOUT has to be a duration like 3s, not %s: %w", env["DATASTORE_TIMEOUT"], err)
	}

	source := datastore.NewSourceDatastore(url, updater, maxParallel, timeout)
	return datastore.New(source, nil, nil), nil
}

func parseDuration(s string) (time.Duration, error) {
	sec, err := strconv.Atoi(s)
	if err == nil {
		// TODO External error
		return time.Duration(sec) * time.Second, nil
	}

	return time.ParseDuration(s)
}
