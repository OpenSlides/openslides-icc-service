ARG CONTEXT=prod

FROM golang:1.24.2-alpine as base

## Setup
ARG CONTEXT
WORKDIR /root/openslides-icc-service
ENV ${CONTEXT}=1

## Install
RUN apk add git --no-cache

COPY go.mod go.sum ./
RUN go mod download

COPY main.go main.go
COPY internal internal

## External Information
LABEL org.opencontainers.image.title="OpenSlides ICC Service"
LABEL org.opencontainers.image.description="With the OpenSlides ICC Service clients can communicate with each other."
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.source="https://github.com/OpenSlides/openslides-icc-service"

EXPOSE 9007


# Development Image

FROM base as dev

RUN ["go", "install", "github.com/githubnemo/CompileDaemon@latest"]

## Command
CMD CompileDaemon -log-prefix=false -build="go build" -command="./openslides-icc-service"



# Testing Image

FROM dev as tests



# Production Image

FROM base as builder

RUN go build


FROM scratch as prod

WORKDIR /

LABEL org.opencontainers.image.title="OpenSlides ICC Service"
LABEL org.opencontainers.image.description="With the OpenSlides ICC Service clients can communicate with each other."
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.source="https://github.com/OpenSlides/openslides-icc-service"

EXPOSE 9007
COPY --from=builder /root/openslides-icc-service/openslides-icc-service .
ENTRYPOINT ["/openslides-icc-service"]
HEALTHCHECK CMD ["/openslides-icc-service", "health"]


