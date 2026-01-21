.PHONY:  build run run-simulator test clean lint fmt help db-up db-down db-logs db-shell

# Binary names
AUTOSCALER_BINARY=bin/autoscaler
SIMULATOR_BINARY=bin/simulator

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GORUN=$(GOCMD) run
GOTEST=$(GOCMD) test
GOCLEAN=$(GOCMD) clean
GOGET=$(GOCMD) get
GOFMT=$(GOCMD) fmt
GOVET=$(GOCMD) vet

# Build flags
LDFLAGS=-ldflags "-s -w"

## help:  Show this help message
help:
	@echo "Usage:"
	@sed -n 's/^##//p' $(MAKEFILE_LIST) | column -t -s ': ' | sed -e 's/^/ /'

## build: Build all binaries
build:  build-autoscaler build-simulator

## build-autoscaler: Build the autoscaler binary
build-autoscaler:
	$(GOBUILD) $(LDFLAGS) -o $(AUTOSCALER_BINARY) ./cmd/autoscaler

## build-simulator: Build the simulator binary
build-simulator: 
	$(GOBUILD) $(LDFLAGS) -o $(SIMULATOR_BINARY) ./cmd/simulator

## run: Run the autoscaler
run: 
	$(GORUN) ./cmd/autoscaler/main.go

## run-config: Run with dev config
run-config:
	$(GORUN) ./cmd/autoscaler/main.go --config configs/config.dev.yaml

## run-simulator: Run the metrics simulator
run-simulator: 
	$(GORUN) ./cmd/simulator/main.go

## test: Run all tests
test: 
	$(GOTEST) -v ./...

## test-coverage: Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html

## clean: Clean build artifacts
clean: 
	$(GOCLEAN)
	rm -rf bin/
	rm -f coverage.out coverage.html

## lint: Run linter
lint:
	golangci-lint run ./...

## fmt: Format code
fmt:
	$(GOFMT) ./...
	$(GOVET) ./...

## deps: Download dependencies
deps:
	$(GOGET) -v ./...

## db-up: Start database container
db-up:
	docker compose -f deployments/docker-compose.yml up -d timescaledb

## db-down: Stop database container
db-down: 
	docker compose -f deployments/docker-compose.yml down

## db-down-v: Stop database container and remove volumes
db-down-v:
	docker compose -f deployments/docker-compose.yml down -v

## db-logs: View database logs
db-logs:
	docker compose -f deployments/docker-compose.yml logs -f timescaledb

## db-shell: Open psql shell
db-shell: 
	docker exec -it autoscaler-db psql -U admin -d autoscaler

## docker-up: Start all services with Docker Compose
docker-up: 
	docker compose -f deployments/docker-compose.yml up --build

## docker-down: Stop all services
docker-down:
	docker compose -f deployments/docker-compose.yml down -v

## migrate:  Run database migrations
migrate:
	$(GORUN) ./cmd/autoscaler/main.go --migrate

## migrate-script: Run migration script
migrate-script:
	./scripts/migrate.sh