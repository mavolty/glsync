BINARY     := glsync
BUILD_DIR  := bin
CONFIG     ?= config/glsync.yaml
DB_URL     ?= postgres://glsync:glsync@localhost:5432/glsync?sslmode=disable

# Load .env file if it exists
-include .env
export

.PHONY: build test lint run docker-build migrate-up migrate-down

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY) ./cmd/glsync

test:
	go test ./... -race -count=1

lint:
	golangci-lint run ./...

run: build
	./$(BUILD_DIR)/$(BINARY) -config $(CONFIG)

migrate-up:
	migrate -path migrations -database "$(DB_URL)" up

migrate-down:
	migrate -path migrations -database "$(DB_URL)" down 1

migrate-status:
	migrate -path migrations -database "$(DB_URL)" version

docker-build:
	docker build -t $(BINARY):latest .

# Create local dev database (requires Docker)
dev-db:
	docker run -d --name glsync-pg \
		-e POSTGRES_USER=glsync \
		-e POSTGRES_PASSWORD=glsync \
		-e POSTGRES_DB=glsync \
		-p 5432:5432 \
		postgres:16-alpine

tidy:
	go mod tidy
