# Cloud Autoscaler API - Postman Collection

This directory contains Postman collection and environment files for testing the Cloud Autoscaler API.

## Files

| File                                            | Description                                |
| ----------------------------------------------- | ------------------------------------------ |
| `Cloud_Autoscaler_API.postman_collection.json`  | Complete API collection with all endpoints |
| `Cloud_Autoscaler_Dev.postman_environment.json` | Development environment variables          |

## Quick Start

### 1. Import into Postman

1. Open Postman
2. Click **Import** (top-left)
3. Drag both JSON files or select them
4. The collection and environment will be imported

### 2. Select Environment

1. Click the environment dropdown (top-right)
2. Select **"Cloud Autoscaler - Development"**

### 3. Authenticate

1. Expand **Authentication** folder
2. Run **Login** request with credentials:
   ```json
   {
     "username": "admin",
     "password": "admin123"
   }
   ```
3. The JWT token is automatically saved to `{{jwt_token}}` variable

### 4. Test Endpoints

All protected endpoints automatically use the saved JWT token.

---

## API Overview

### Base URL

```
http://localhost:8080
```

### Authentication

- **Type:** JWT Bearer Token
- **Header:** `Authorization: Bearer <token>`
- **Expiration:** 24 hours (86400 seconds)

---

## Endpoints Reference

### Health Endpoints (Public)

| Method | Endpoint        | Description                            |
| ------ | --------------- | -------------------------------------- |
| GET    | `/health`       | Full health check with database status |
| GET    | `/health/ready` | Kubernetes readiness probe             |
| GET    | `/health/live`  | Kubernetes liveness probe              |

### Authentication (Public)

| Method | Endpoint      | Description                    |
| ------ | ------------- | ------------------------------ |
| POST   | `/auth/login` | Authenticate and get JWT token |

**Request Body:**

```json
{
  "username": "admin",
  "password": "admin123"
}
```

**Response:**

```json
{
  "token": "eyJhbGciOiJIUzI1NiIs...",
  "expires_in": 86400,
  "username": "admin"
}
```

### Clusters (Protected)

| Method | Endpoint               | Description                           |
| ------ | ---------------------- | ------------------------------------- |
| GET    | `/clusters`            | List all clusters                     |
| POST   | `/clusters`            | Create a new cluster                  |
| GET    | `/clusters/:id`        | Get cluster by ID                     |
| PUT    | `/clusters/:id`        | Update cluster                        |
| DELETE | `/clusters/:id`        | Delete cluster                        |
| GET    | `/clusters/:id/status` | Get cluster status with server counts |

**Create Cluster Request:**

```json
{
  "name": "production-cluster",
  "min_servers": 2,
  "max_servers": 20,
  "config": {
    "collector_endpoint": "http://metrics:9000/metrics",
    "target_cpu": 70.0
  }
}
```

**Update Cluster Request (all fields optional):**

```json
{
  "name": "new-name",
  "min_servers": 3,
  "max_servers": 25,
  "status": "active",
  "config": {
    "target_cpu": 75.0
  }
}
```

### Metrics (Protected)

| Method | Endpoint                       | Description                     |
| ------ | ------------------------------ | ------------------------------- |
| GET    | `/clusters/:id/metrics`        | Get metrics (raw or aggregated) |
| GET    | `/clusters/:id/metrics/latest` | Get latest metrics snapshot     |
| GET    | `/clusters/:id/metrics/hourly` | Get hourly aggregated metrics   |

**Query Parameters:**

| Parameter    | Type   | Description                        | Default    |
| ------------ | ------ | ---------------------------------- | ---------- |
| `from`       | string | Start time (RFC3339)               | 1 hour ago |
| `to`         | string | End time (RFC3339)                 | now        |
| `range`      | string | Relative range (`1h`, `24h`, `7d`) | -          |
| `limit`      | int    | Max results (1-1000)               | 100        |
| `aggregated` | bool   | Enable aggregation                 | false      |
| `bucket`     | int    | Bucket size in minutes             | 5          |

**Examples:**

```
GET /clusters/abc123/metrics?range=1h&limit=50
GET /clusters/abc123/metrics?aggregated=true&bucket=10&range=24h
GET /clusters/abc123/metrics?from=2026-01-23T00:00:00Z&to=2026-01-23T12:00:00Z
```

### Scaling Events (Protected)

| Method | Endpoint                     | Description                      |
| ------ | ---------------------------- | -------------------------------- |
| GET    | `/clusters/:id/events`       | Get cluster scaling events       |
| GET    | `/clusters/:id/events/stats` | Get scaling statistics           |
| GET    | `/events/recent`             | Get recent events (all clusters) |

**Query Parameters:**

| Parameter | Type   | Description          | Default    |
| --------- | ------ | -------------------- | ---------- |
| `from`    | string | Start time (RFC3339) | 1 hour ago |
| `to`      | string | End time (RFC3339)   | now        |
| `range`   | string | Relative range       | -          |
| `limit`   | int    | Max results          | 50/20      |

### WebSocket (Real-time)

| Protocol | Endpoint | Description       |
| -------- | -------- | ----------------- |
| WS       | `/ws`    | Real-time updates |

**Connection:**

```
ws://localhost:8080/ws
```

**Subscribe to Cluster:**

```json
{
  "type": "subscribe",
  "cluster_id": "550e8400-e29b-41d4-a716-446655440000"
}
```

**Server Events:**

- `metrics_update` - New metrics received
- `scaling_event` - Scaling action occurred
- `cluster_status` - Status changed

---

## Error Responses

All errors follow this format:

```json
{
  "error": "error message here"
}
```

### Common HTTP Status Codes

| Code | Meaning                                   |
| ---- | ----------------------------------------- |
| 200  | Success                                   |
| 201  | Created                                   |
| 400  | Bad Request - Invalid input               |
| 401  | Unauthorized - Missing/invalid token      |
| 404  | Not Found - Resource doesn't exist        |
| 409  | Conflict - Duplicate resource             |
| 500  | Internal Server Error                     |
| 503  | Service Unavailable - Health check failed |

---

## Rate Limiting

- **Limit:** 100 requests per minute (configurable)
- **Scope:** Per IP address

When rate limited, you'll receive:

```json
{
  "error": "rate limit exceeded"
}
```

---

## Environment Variables

| Variable     | Description              | Default                 |
| ------------ | ------------------------ | ----------------------- |
| `base_url`   | API base URL             | `http://localhost:8080` |
| `jwt_token`  | JWT authentication token | (auto-populated)        |
| `cluster_id` | Current cluster ID       | (auto-populated)        |
| `ws_url`     | WebSocket URL            | `ws://localhost:8080`   |

---

## Workflow Example

1. **Login** → Get JWT token (auto-saved)
2. **Create Cluster** → Cluster ID auto-saved
3. **Get Cluster Status** → View server counts
4. **Get Metrics** → View CPU/memory/load data
5. **Get Scaling Events** → View scale up/down history
6. **Connect WebSocket** → Real-time updates

---

## Tips

- Use the **Test** scripts in Login and Create Cluster to auto-save variables
- All protected requests inherit Bearer token from collection auth
- Use `range` parameter for relative time queries instead of `from`/`to`
- WebSocket requires a dedicated WebSocket client in Postman
