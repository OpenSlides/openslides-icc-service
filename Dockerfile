ARG CONTEXT=prod
ARG GO_IMAGE_VERSION=1.24.2

FROM golang:${GO_IMAGE_VERSION}-alpine as base

ARG CONTEXT
ARG GO_IMAGE_VERSION

WORKDIR /root/openslides-icc-service

RUN apk add git

COPY go.mod go.sum ./
RUN go mod download

COPY main.go main.go
COPY internal internal

LABEL org.opencontainers.image.title="OpenSlides ICC Service"
LABEL org.opencontainers.image.description="With the OpenSlides ICC Service clients can communicate with each other."
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.source="https://github.com/OpenSlides/openslides-icc-service"

EXPOSE 9007


# Development Image

FROM base as dev

RUN ["go", "install", "github.com/githubnemo/CompileDaemon@latest"]

WORKDIR /root
CMD CompileDaemon -log-prefix=false -build="go build -o icc-service ./openslides-icc-service" -command="./icc-service"
HEALTHCHECK CMD ["/openslides-icc-service", "health"]


# Testing Image

FROM dev as tests



# Production Image

FROM base as builder

RUN go build


FROM scratch as prod

LABEL org.opencontainers.image.title="OpenSlides ICC Service"
LABEL org.opencontainers.image.description="With the OpenSlides ICC Service clients can communicate with each other."
LABEL org.opencontainers.image.licenses="MIT"
LABEL org.opencontainers.image.source="https://github.com/OpenSlides/openslides-icc-service"

EXPOSE 9007
COPY --from=builder /root/openslides-icc-service/openslides-icc-service .
ENTRYPOINT ["/openslides-icc-service"]
HEALTHCHECK CMD ["/openslides-icc-service", "health"]


