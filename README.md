# Cloud Resource Auto-Scaler

Production-grade multi-cluster auto-scaling system with real-time monitoring, ML-based predictions, and live dashboard.

## Tech Stack

- **Backend:** Go 1.21+ (Gin, WebSockets, goroutines)
- **Database:** TimescaleDB (PostgreSQL with time-series extension)
- **Frontend:** React + Vite + Zustand + WebSockets
- **Deployment:** Docker Compose

## Project Structure

``
cloud-autoscaler/
├── cmd/ # Application entry points
│ ├── autoscaler/ # Main auto-scaler service
│ └── simulator/ # Metrics simulator service
├── internal/ # Private application code
│ ├── orchestrator/ # Central coordinator
│ ├── collector/ # Metrics collection
│ ├── analyzer/ # Metrics analysis
│ ├── decision/ # Scaling decisions
│ ├── predictor/ # ML predictions
│ ├── scaler/ # Scaling execution
│ ├── logger/ # Structured logging
│ └── websocket/ # Real-time updates
├── pkg/ # Public reusable packages
│ ├── models/ # Data models
│ ├── config/ # Configuration
│ └── database/ # Database client
├── api/ # HTTP handlers
│ ├── handlers/ # Route handlers
│ └── middleware/ # HTTP middleware
├── simulator/ # Simulator logic
├── web/ # React frontend
├── tests/ # Test suites
├── configs/ # Configuration files
├── deployments/ # Docker files
├── scripts/ # Utility scripts
└── docs/ # Documentation

````

## Getting Started

### Prerequisites

- Go 1.21+
- Docker and Docker Compose
- Node.js 18+ (for frontend)

### Development

```bash
# Run auto-scaler
go run cmd/autoscaler/main.go

# Run simulator
go run cmd/simulator/main. go
````
