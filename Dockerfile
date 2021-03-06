FROM golang:1.16.5-alpine as basis
LABEL maintainer="OpenSlides Team <info@openslides.com>"
WORKDIR /root/

RUN apk add git

COPY go.mod go.sum ./
RUN go mod download

COPY cmd cmd
COPY internal internal

# Build service in seperate stage.
FROM basis as builder
RUN CGO_ENABLED=0 go build ./cmd/icc


# Development build.
FROM basis as development

RUN ["go", "install", "github.com/githubnemo/CompileDaemon@latest"]
EXPOSE 9012
ENV MESSAGING redis
ENV AUTH ticket

CMD CompileDaemon -log-prefix=false -build="go build ./cmd/icc" -command="./icc"


# Productive build
FROM scratch

COPY --from=builder /root/icc .
EXPOSE 9007
ENV MESSAGING redis
ENV AUTH ticket
ENTRYPOINT ["/icc"]
