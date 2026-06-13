.PHONY: build run swagger test migrate-up migrate-down

DATABASE_URL ?= postgres://postgres:password@localhost:5432/push?sslmode=disable

build: swagger
	go build -o bin/push-service cmd/main.go

run: build
	./bin/push-service

swagger:
	go run github.com/swaggo/swag/cmd/swag@latest init -g cmd/main.go --parseDependency --parseInternal -o docs

migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down -all

test:
	go test ./...
