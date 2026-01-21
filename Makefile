.PHONY:  build run run-simulator test clean lint fmt help

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

## run-simulator: Run the metrics simulator
run-simulator:
	$(GORUN) ./cmd/simulator/main.go

## test: Run all tests
test: 
	$(GOTEST) -v ./... 

## test-coverage: Run tests with coverage
test-coverage:
	$(GOTEST) -v -coverprofile=coverage.out ./... 
	$(GOCMD) tool cover -html=coverage.out -o coverage. html

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

## docker-up: Start all services with Docker Compose
docker-up: 
	docker-compose -f deployments/docker-compose.yml up --build

## docker-down: Stop all services
docker-down:
	docker-compose -f deployments/docker-compose.yml down -v