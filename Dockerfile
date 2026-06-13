FROM golang:1.26.3-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o push-service ./cmd/main.go

FROM alpine:3.23
WORKDIR /app
COPY --from=builder /app/push-service .
RUN adduser -D -u 10001 app
RUN mkdir -p /data && chown app:app /data
USER app
CMD ["./push-service"]
