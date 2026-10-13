.PHONY: all build test test-race test-integration run docker-up docker-down docker-build trivy-scan tf-validate clean help

# Configuration defaults
APP_NAME := ledger-engine
DOCKER_IMAGE := $(APP_NAME):latest
DATABASE_URL ?= postgres://ledger_user:ledger_secret@127.0.0.1:5432/ledger_db?sslmode=disable
REDIS_ADDR ?= 127.0.0.1:6379

all: build

## help: Display available targets
help:
	@echo "Available make targets:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-20s\033[0m %s\n", $$1, $$2}'

## build: Build the statically linked Go server binary
build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -o bin/$(APP_NAME) ./cmd/server

## test: Run unit tests
test:
	go test -v ./internal/...

## test-race: Run all tests with Go race detector
test-race:
	DATABASE_URL="$(DATABASE_URL)" REDIS_ADDR="$(REDIS_ADDR)" go test -v -race ./...

## test-integration: Run high-concurrency integration tests against PostgreSQL and Redis
test-integration:
	DATABASE_URL="$(DATABASE_URL)" REDIS_ADDR="$(REDIS_ADDR)" go test -v -race ./test/integration/...

## run: Run the ledger engine locally
run:
	DATABASE_URL="$(DATABASE_URL)" REDIS_ADDR="$(REDIS_ADDR)" go run ./cmd/server

## docker-up: Start PostgreSQL and Redis infrastructure containers
docker-up:
	docker compose up -d postgres redis 2>/dev/null || podman run -d --rm --name ledger-postgres -p 5432:5432 -e POSTGRES_USER=ledger_user -e POSTGRES_PASSWORD=ledger_secret -e POSTGRES_DB=ledger_db postgres:16-alpine && podman run -d --rm --name ledger-redis -p 6379:6379 redis:7-alpine

## docker-down: Stop local infrastructure containers
docker-down:
	docker compose down 2>/dev/null || podman stop ledger-postgres ledger-redis 2>/dev/null || true

## docker-build: Build minimal, non-root Alpine container image
docker-build:
	docker build -t $(DOCKER_IMAGE) . 2>/dev/null || podman build -t $(DOCKER_IMAGE) .

## trivy-scan: Scan container image with Trivy for vulnerabilities
trivy-scan: docker-build
	trivy image --severity CRITICAL,HIGH $(DOCKER_IMAGE) 2>/dev/null || \
	(rm -f /tmp/$(APP_NAME).tar && podman save $(DOCKER_IMAGE) -o /tmp/$(APP_NAME).tar && chmod 666 /tmp/$(APP_NAME).tar && \
	podman run --rm -v /tmp/$(APP_NAME).tar:/img.tar:ro,Z docker.io/aquasec/trivy:latest image --input /img.tar --severity CRITICAL,HIGH)

## tf-validate: Validate and lint Terraform modules
tf-validate:
	terraform fmt -check -recursive
	terraform -chdir=terraform/environments/prod init -backend=false
	terraform -chdir=terraform/environments/prod validate

## clean: Remove compiled binaries and test coverage files
clean:
	rm -rf bin/ coverage.out coverage.txt /tmp/$(APP_NAME).tar
