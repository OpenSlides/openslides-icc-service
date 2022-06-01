# OpenSlides-ICC

The ICC Service is part of the OpenSlides environment. Clients can connect to it
and communicate with eachother.

IMPORTANT: The data are sent via an open http-connection. All browsers limit the
amount of open http1.1 connections to a domain. For this service to work, the
browser has to connect to the service with http2 and therefore needs https.


## Start

### With Golang

```
go build ./cmd/icc
./icc
```

### With Docker

The docker build uses the auth token. Either configure it to use the fake
services (see environment variables below) or make sure the service inside the
docker container can connect to redis and the datastore-reader. For example with
the docker argument --network host. The auth-secrets have to given as a file.

```
docker build . --tag openslides-icc
printf "my_token_key" > auth_token_key 
printf "my_cookie_key" > auth_cookie_key
docker run --network host -v $PWD/auth_token_key:/run/secrets/auth_token_key -v $PWD/auth_cookie_key:/run/secrets/auth_cookie_key openslides-icc
```

It uses the host network to connect to redis.


### With Auto Restart

To restart the service when ever a source file has shanged, the tool
[CompileDaemon](https://github.com/githubnemo/CompileDaemon) can help.

```
go install github.com/githubnemo/CompileDaemon@latest
CompileDaemon -log-prefix=false -build "go build ./cmd/icc" -command "./icc"
```

The make target `build-dev` creates a docker image that uses this tool. The
environment varialbe `OPENSLIDES_DEVELOPMENT` is used to use default auth keys.

```
make build-dev
docker run --network host --env OPENSLIDES_DEVELOPMENT=true openslides-icc-dev
```


## Test

### With Golang

```
go test ./...
```


## Examples

Curl needs the flag `-N / --no-buffer` or it can happen, that the output is not
printed immediately.


### Notify

To listen to messages, you can use this command:

```
curl -N localhot:9007/system/icc/notify?meeting_id=5
```

The meeting_id query argument is optional.

The output has the [json lines](https://jsonlines.org/) format.

The first line returns an individual channel-id. It has to be used later so
publish messages:

```
{"channel_id": "QRboMVjb:1:0"}
```

Each other other line is one notify message. It has the following format:

```
{"sender_user_id":1,"sender_channel_id":"8NWRQy18:1:0","name":"my message title","message":"my message"}
```

To publish a message, you can use the following request:

```
curl localhost:9007/system/icc/notify/publish -d '{
  "channel_id": "STRING_SEE_ABOVE",
  "to_meeting": 5,
  "to_users": [3,4],
  "to_channels": "some:valid:channel_id",
  "name": "my message title",
  "message": {"any":"valid","json":"data"}
}'
```

The example message would be received by all users that are in meeting 5, and to
all connections of the user 3 and 4 and the connection with the channel id
"some:valid:channel_id".

Only one of the to_* fields is required. All other fields are required.


### Applause

The applause service needs a running datastore-reader. For testing, you can use
the [fake
datastore](https://github.com/OpenSlides/openslides-autoupdate-service/tree/main/cmd/datastore)
from the autoupdate-service repo.

To listen to messages, you can use this command:

```
curl -N localhot:9007/system/icc/notify?meeting_id=5
```

The meeting_id argument is required.

The returned messages have the format:

```
{"level":5,"present_users":25}
```

To send applause, use:

```
curl localhost:9007/system/icc/applause/send?meeting_id=1
```

The argument meeting_id is required.


## Configuration

### Environment variables

The Service uses the following environment variables:

* `ICC_PORT`: Lets the service listen on port 9007. The default is
  `9007`.
* `ICC_HOST`: The device where the service starts. The default is am
  empty string which starts the service on any device.
* `ICC_REDIS_HOST`: The host of the redis instance to save icc messages. The
  default is `localhost`.
* `ICC_REDIS_PORT`: The port of the redis instance to save icc messages. The
  default is `6379`.
* `DATASTORE_READER_HOST`: Host of the datastore reader. The default is
  `localhost`.
* `DATASTORE_READER_PORT`: Port of the datastore reader. The default is `9010`.
* `DATASTORE_READER_PROTOCOL`: Protocol of the datastore reader. The default is
  `http`.
* `MESSAGE_BUS_HOST`: Host of the redis server. The default is `localhost`.
* `MESSAGE_BUS_PORT`: Port of the redis server. The default is `6379`.
* `REDIS_TEST_CONN`: Test the redis connection on startup. Disable on the cloud
  if redis needs more time to start then this service. The default is `true`.
* `AUTH`: Sets the type of the auth service. `fake` (default) or `ticket`.
* `AUTH_HOST`: Host of the auth service. The default is `localhost`.
* `AUTH_PORT`: Port of the auth service. The default is `9004`.
* `AUTH_PROTOCOL`: Protocol of the auth servicer. The default is `http`.
* `OPENSLIDES_DEVELOPMENT`: If set, the service starts, even when secrets (see
  below) are not given. The default is `false`.
* `MAX_PARALLEL_KEYS`: Max keys that are send in one request to the datastore.
  The default is `1000`.


### Secrets

Secrets are filenames in `/run/secrets/`. The service only starts if it can find
each secret file and read its content. The default values are only used, if the
environment variable `OPENSLIDES_DEVELOPMENT` is set.

* `auth_token_key`: Key to sign the JWT auth tocken. Default `auth-dev-key`.
* `auth_cookie_key`: Key to sign the JWT auth cookie. Default `auth-dev-key`.
