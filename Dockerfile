ARG CONTEXT=prod
ARG GO_IMAGE_VERSION=1.24.2

FROM golang:${GO_IMAGE_VERSION}-alpine as base

## Setup
ARG CONTEXT
ARG GO_IMAGE_VERSION
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

## Command
COPY ./dev/command.sh ./
RUN chmod +x command.sh
CMD ["./command.sh"]



# Development Image

FROM base as dev

RUN ["go", "install", "github.com/githubnemo/CompileDaemon@latest"]

WORKDIR /root

## Command (workdir reset)
COPY ./dev/command.sh ./
RUN chmod +x command.sh
HEALTHCHECK CMD ["/openslides-icc-service", "health"]


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


