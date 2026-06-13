FROM golang:1.26.3-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o push-service ./cmd/main.go

FROM alpine:3.23
WORKDIR /app
COPY --from=builder /app/push-service .
COPY --from=builder /app/config ./config
RUN adduser -D -u 10001 app
RUN mkdir -p /data && chown app:app /data
USER app
ENTRYPOINT ["./push-service"]
CMD ["serve", "--config=config/config.example.yaml"]
