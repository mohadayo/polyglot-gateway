FROM golang:1.22-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY *.go .

RUN go build -o rate-limiter .

FROM alpine:3.20

WORKDIR /app
COPY --from=builder /app/rate-limiter .

EXPOSE 8002

CMD ["./rate-limiter"]
