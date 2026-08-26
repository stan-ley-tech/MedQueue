.PHONY: build run run-worker test test-unit test-integration test-race test-load lint fmt vet migrate-up migrate-down migrate-version docker-up docker-down docker-logs tidy

build:
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker
	go build -o bin/migrate ./cmd/migrate

run:
	go run ./cmd/api

run-worker:
	go run ./cmd/worker

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-version:
	go run ./cmd/migrate version

test:
	go test ./... -count=1

test-unit:
	go test ./internal/... -count=1

test-race:
	go test ./... -race -count=1

test-integration:
	go test ./tests/integration/... -count=1 -tags=integration

test-e2e:
	go test ./tests/e2e/... -count=1 -tags=e2e

test-load:
	go test ./tests/load/... -count=1 -run TestQueueLoad -v

lint:
	go vet ./...
	gofmt -l .

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down -v

docker-logs:
	docker compose logs -f api worker
