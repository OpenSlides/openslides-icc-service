ARG CONTEXT=prod

FROM golang:1.24.5-alpine AS base

## Setup
ARG CONTEXT
WORKDIR /app/openslides-icc-service
ENV APP_CONTEXT=${CONTEXT}

## Install
RUN apk add git --no-cache

COPY go.mod go.sum ./
RUN go mod download

COPY main.go main.go
COPY internal internal

## External Information
EXPOSE 9007

# Development Image

FROM base AS dev

RUN ["go", "install", "github.com/githubnemo/CompileDaemon@latest"]

## Command
CMD CompileDaemon -log-prefix=false -build="go build" -command="./openslides-icc-service"

# Testing Image

FROM dev AS tests

COPY dev/container-tests.sh ./dev/container-tests.sh

RUN apk add --no-cache \
    build-base \
    docker && \
    go get -u github.com/ory/dockertest/v3 && \
    go install golang.org/x/lint/golint@latest && \
    chmod +x dev/container-tests.sh

## Command
STOPSIGNAL SIGKILL
CMD ["sleep", "inf"]

# Production Image

FROM base AS builder

RUN go build

FROM scratch AS prod

## Setup
ARG CONTEXT
ENV APP_CONTEXT=prod

LABEL org.opencontainers.image.title="OpenSlides ICC Service"
LABEL org.opencontainers.image.description="With the OpenSlides ICC Service clients can communicate with each other."
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.source="https://github.com/OpenSlides/openslides-icc-service"

EXPOSE 9007
COPY --from=builder /app/openslides-icc-service/openslides-icc-service /
ENTRYPOINT ["/openslides-icc-service"]
HEALTHCHECK CMD ["/openslides-icc-service", "health"]
