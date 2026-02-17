# Quick Start Guide

## 🚀 Deploy in 5 Minutes

### Prerequisites

- Docker & Docker Compose installed
- Git installed
- 5 minutes of your time

### Step 1: Clone & Navigate

```bash
cd /home/josh/JOSH/cloud-autoscaler/deployments
```

### Step 2: Create Environment File

```bash
cp .env.example .env
```

### Step 3: Generate Secrets

```bash
# Generate JWT Secret (required)
echo "JWT_SECRET=$(openssl rand -base64 48)" >> .env

# Generate DB Password (required)
echo "DB_PASSWORD=$(openssl rand -base64 32)" >> .env

# Update your domain (optional, defaults to *)
echo "ALLOWED_ORIGIN=https://yourdomain.com" >> .env
```

### Step 4: Deploy

```bash
docker-compose up -d
```

### Step 5: Verify

```bash
# Check services are running
docker-compose ps

# Test health endpoint
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy","timestamp":"..."}
```

## 🎯 Next Steps

### 1. Create Your First User

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "SecurePass123!"
  }'
```

**Password Requirements:**

- Minimum 8 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one number
- At least one special character

### 2. Login & Get Token

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "SecurePass123!"
  }'
```

Save the token from the response:

```json
{
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expires_in": 86400,
  "username": "admin"
}
```

### 3. Create a Cluster

```bash
export TOKEN="your-token-here"

curl -X POST http://localhost:8080/clusters \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{
    "name": "my-first-cluster",
    "min_servers": 2,
    "max_servers": 10
  }'
```

### 4. View Cluster Metrics

```bash
# Get cluster ID from previous response
export CLUSTER_ID="cluster-id-here"

curl -X GET "http://localhost:8080/clusters/$CLUSTER_ID/metrics" \
  -H "Authorization: Bearer $TOKEN"
```

### 5. Access the API Documentation

Open in your browser:

```
http://localhost:8080/swagger/index.html
```

## 📊 Available Endpoints

### Public Endpoints

- `GET /health` - Health check
- `POST /auth/register` - Register new user
- `POST /auth/login` - Login user

### Protected Endpoints (require Bearer token)

- `GET /clusters` - List clusters
- `POST /clusters` - Create cluster
- `GET /clusters/:id` - Get cluster details
- `PUT /clusters/:id` - Update cluster
- `DELETE /clusters/:id` - Delete cluster
- `GET /clusters/:id/metrics` - Get cluster metrics
- `GET /clusters/:id/events` - Get scaling events

### WebSocket

- `GET /ws` - WebSocket connection for real-time updates

## 🔧 Common Commands

### View Logs

```bash
# All services
docker-compose logs -f

# Specific service
docker-compose logs -f autoscaler
docker-compose logs -f simulator
```

### Restart Services

```bash
# All services
docker-compose restart

# Specific service
docker-compose restart autoscaler
```

### Stop Services

```bash
docker-compose down
```

### Stop and Remove Data

```bash
docker-compose down -v  # WARNING: This deletes all data!
```

## 🐛 Troubleshooting

### Service Won't Start

```bash
# Check logs
docker-compose logs autoscaler

# Common issues:
# 1. JWT_SECRET not set
# 2. DB_PASSWORD not set
# 3. Port already in use
```

### Can't Connect to API

```bash
# Verify service is running
docker-compose ps

# Check if port is accessible
curl http://localhost:8080/health

# Check firewall rules
sudo ufw status
```

### Database Connection Error

```bash
# Check database is healthy
docker-compose ps timescaledb

# Check database logs
docker-compose logs timescaledb

# Verify connection settings in .env
```

## 📚 More Information

- **Full Deployment Guide**: See `docs/DEPLOYMENT.md`
- **Security Guide**: See `docs/SECURITY_IMPROVEMENTS.md`
- **API Documentation**: http://localhost:8080/swagger/index.html

## 🎓 Example Workflow

Here's a complete workflow example:

```bash
# 1. Deploy
cd deployments
cp .env.example .env
echo "JWT_SECRET=$(openssl rand -base64 48)" >> .env
echo "DB_PASSWORD=$(openssl rand -base64 32)" >> .env
docker-compose up -d

# 2. Create user
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{"username":"myuser","password":"SecurePass123!"}'

# 3. Login
TOKEN=$(curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"myuser","password":"SecurePass123!"}' | jq -r '.token')

# 4. Create cluster
CLUSTER=$(curl -X POST http://localhost:8080/clusters \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $TOKEN" \
  -d '{"name":"production","min_servers":2,"max_servers":10}' | jq -r '.id')

# 5. Monitor metrics
watch -n 5 "curl -s http://localhost:8080/clusters/$CLUSTER/metrics \
  -H 'Authorization: Bearer $TOKEN' | jq '.'"
```

## 🎉 Success!

Your Cloud Autoscaler is now running and ready to manage your infrastructure!

**What's Next?**

- Set up monitoring with Prometheus (port 9090)
- Configure HTTPS with a reverse proxy
- Set up automated backups
- Review security settings for production
- Connect your actual infrastructure (replace simulator)
