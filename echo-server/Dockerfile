FROM golang:1.21-alpine AS builder

WORKDIR /app
COPY go.mod* ./
RUN go mod init echoserver || true
COPY . .

RUN CGO_ENABLED=0 go build -o /echo-server

FROM alpine:latest

COPY --from=builder /echo-server /echo-server

EXPOSE 80
CMD ["/echo-server"]