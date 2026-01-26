# Cloud Autoscaler

Production-grade cloud resource autoscaling system with real-time metrics collection, intelligent analysis, and automated scaling decisions.

## Overview

A comprehensive autoscaling solution that monitors cluster metrics, analyzes resource utilization patterns, and automatically scales infrastructure up or down based on configurable thresholds and sustained usage patterns. Built with resilience, observability, and extensibility in mind.

## Features

- **Real-time Metrics Collection**: HTTP-based metrics collection with circuit breaker and retry logic
- **Intelligent Analysis**: CPU and memory utilization analysis with trend detection and spike identification
- **Smart Scaling Decisions**: Configurable thresholds, cooldown periods, and sustained pattern detection
- **Multiple Cluster Support**: Concurrent pipeline orchestration for managing multiple clusters
- **Resilience**: Circuit breaker pattern, graceful degradation, and error recovery
- **WebSocket Support**: Real-time cluster state updates and event streaming
- **TimescaleDB Integration**: Time-series data storage with automatic retention policies
- **RESTful API**: Comprehensive HTTP API with JWT authentication
- **Built-in Simulator**: Metrics simulator for testing scaling scenarios

## Architecture

The system follows a pipeline architecture where each cluster has its own processing pipeline:

```
Collector → Analyzer → Decision Engine → Scaler
     ↓           ↓            ↓            ↓
  Metrics    Analysis     Decision      Scaling
Collection   & Trends    Making       Execution
```

### Components

- **Orchestrator**: Central coordinator managing multiple cluster pipelines
- **Collector**: Fetches metrics from cluster endpoints with resilience patterns
- **Analyzer**: Evaluates CPU/memory utilization and detects trends
- **Decision Engine**: Makes scaling decisions based on configurable policies
- **Scaler**: Executes scaling actions (simulator or cloud provider integration)
- **Event Bus**: Publishes system events for logging and WebSocket broadcast

## Tech Stack

- **Language**: Go 1.25+
- **Web Framework**: Gin
- **Database**: PostgreSQL with TimescaleDB extension
- **Authentication**: JWT (golang-jwt/jwt)
- **WebSockets**: Gorilla WebSocket
- **Configuration**: Viper (YAML)
- **Logging**: Logrus

## Project Structure

```
.
├── cmd/                    # Application entry points
│   ├── autoscaler/        # Main autoscaler service
│   └── simulator/         # Metrics simulator
├── internal/              # Private application logic
│   ├── orchestrator/      # Pipeline orchestration
│   ├── collector/         # Metrics collection
│   ├── analyzer/          # Metrics analysis
│   ├── decision/          # Scaling decision engine
│   ├── scaler/            # Scaling execution
│   ├── resilience/        # Circuit breaker implementation
│   └── events/            # Event bus and logging
├── pkg/                   # Public packages
│   ├── config/            # Configuration management
│   ├── database/          # Database client and migrations
│   └── models/            # Data models
├── api/                   # HTTP API layer
│   ├── handlers/          # Route handlers
│   └── middleware/        # Auth, CORS, rate limiting
└── configs/               # Configuration files
```

## Getting Started

### Prerequisites

- Go 1.25 or higher
- PostgreSQL 14+ with TimescaleDB extension
- Docker and Docker Compose (optional)

### Installation

1. Clone the repository:

```bash
git clone https://github.com/OldStager01/cloud-autoscaler.git
cd cloud-autoscaler
```

2. Install dependencies:

```bash
go mod download
```

3. Set up the database:

```bash
docker-compose -f deployments/docker-compose.yml up -d
```

4. Run database migrations:

```bash
./scripts/migrate.sh
# or
go run cmd/autoscaler/main.go --migrate
```

### Configuration

Edit `configs/config.dev.yaml` to configure:

- Database connection settings
- Collector endpoints and intervals
- Scaling thresholds and policies
- API server settings
- WebSocket configuration

Key configuration sections:

- `analyzer.thresholds`: CPU/memory utilization thresholds
- `decision`: Cooldown periods, min/max servers, scale step size
- `collector`: Metrics endpoint, retry logic, circuit breaker settings
- `scaler`: Scaling backend type (simulator or cloud provider)

### Running

Quick start:

```bash
# 1. Build the project
make build

# 2. Start the database (ensure port 5432 is not occupied)
# If port 5432 is occupied by PostgreSQL, stop it first:
# sudo systemctl stop postgresql
make db-up

# 3. Run the autoscaler
make run

# 4. Run the simulator (in a separate terminal)
go run cmd/simulator/main.go
```

Alternative methods:

Using Go directly:

```bash
# Run autoscaler with default config
go run cmd/autoscaler/main.go

# Run with specific config file
go run cmd/autoscaler/main.go --config configs/config.dev.yaml
```

Using compiled binaries:

```bash
./bin/autoscaler --config configs/config.dev.yaml
./bin/simulator
```

## API Endpoints

### Authentication

- `POST /api/v1/auth/register` - Register new user
- `POST /api/v1/auth/login` - Login and get JWT token

### Clusters

- `GET /api/v1/clusters` - List all clusters
- `POST /api/v1/clusters` - Create cluster
- `GET /api/v1/clusters/:id` - Get cluster details
- `PUT /api/v1/clusters/:id` - Update cluster
- `DELETE /api/v1/clusters/:id` - Delete cluster
- `GET /api/v1/clusters/:id/state` - Get current cluster state

### Metrics

- `GET /api/v1/metrics/clusters/:id` - Get cluster metrics history
- `GET /api/v1/metrics/clusters/:id/latest` - Get latest metrics

### Health

- `GET /api/health` - Health check

### WebSocket

- `WS /ws` - Real-time cluster updates (requires authentication)

## Testing

```bash
# Run all tests
make test

# Run specific tests
go test ./internal/analyzer/...

# Run with coverage
go test -cover ./...
```

## Development

### Building

```bash
make build              # Build all binaries
make build-autoscaler   # Build autoscaler only
make build-simulator    # Build simulator only
```

### Code Quality

```bash
make fmt    # Format code
make lint   # Run linter
make vet    # Run go vet
```

### Database

```bash
make db-up      # Start database
make db-down    # Stop database
make db-logs    # View database logs
make db-shell   # Connect to database shell
```

## Scaling Policies

The system supports multiple scaling strategies:

- **Threshold-based**: Scale when CPU/memory exceeds configured thresholds
- **Sustained Pattern**: Require sustained high/low usage before scaling
- **Emergency Scaling**: Rapid scale-up when emergency thresholds are breached
- **Cooldown Periods**: Prevent flapping with configurable cooldown between actions
- **Bounded Scaling**: Min/max server limits and maximum scale step size

## License

This project is licensed under the MIT License.

## Contributing

Contributions are welcome. Please ensure tests pass and follow the existing code style.

## Author

OldStager01
