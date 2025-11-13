FROM golang:1.25-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "-s -w" -o ./bin/vortex ./cmd/vortex/

FROM alpine:latest
WORKDIR /app

COPY --from=builder /app/bin/vortex /app/vortex
COPY ./web /app/web
COPY ./samples /app/samples

ENV VORTEX_STATIC_ROOT=/app/web/static
ENV VORTEX_CONFIG_FILE_PATH=/app/samples
ENV VORTEX_CONFIG_FILE_NAME=docker.vortex

EXPOSE 8080 9090

CMD ["/app/vortex"]