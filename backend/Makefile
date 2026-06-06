.PHONY: dev migrate-up migrate-down sqlc test lint build run-api run-worker tidy

# Run full stack via docker compose
dev:
	docker compose up

# Apply migrations
migrate-up:
	goose -dir internal/db/migrations postgres "$$DATABASE_URL" up

# Roll back one migration
migrate-down:
	goose -dir internal/db/migrations postgres "$$DATABASE_URL" down

# Generate sqlc query code
sqlc:
	sqlc generate

# Run all tests
test:
	go test ./...

# Lint
lint:
	go vet ./...

# Build both binaries to bin/
build:
	mkdir -p bin
	go build -o bin/api ./cmd/api
	go build -o bin/worker ./cmd/worker

# Resolve and pin dependencies
tidy:
	go mod tidy

# Run API server locally (without docker)
run-api:
	go run ./cmd/api

# Run worker locally (without docker)
run-worker:
	go run ./cmd/worker
