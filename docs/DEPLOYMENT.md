# Production Deployment Guide

## 🔒 Security Checklist

Before deploying to production, ensure you've completed all these security steps:

### Essential Security Requirements

- [ ] **JWT Secret**: Generate a strong, random JWT secret (minimum 32 characters)

  ```bash
  # Generate a secure random secret
  openssl rand -base64 48
  ```

- [ ] **Database Password**: Use a strong database password (not the default)

  ```bash
  # Generate a secure password
  openssl rand -base64 32
  ```

- [ ] **SSL/TLS**: Enable SSL for database connections
- [ ] **HTTPS**: Deploy behind a reverse proxy with SSL/TLS (nginx, Traefik, etc.)
- [ ] **CORS**: Configure allowed origins (don't use `*` in production)
- [ ] **Environment Variables**: Never commit secrets to version control

### Security Features Implemented

✅ **Input Validation & Sanitization**

- Username validation (alphanumeric + underscores, 3-50 chars)
- Cluster name validation (prevents SQL injection, XSS)
- Password strength requirements (8+ chars, uppercase, lowercase, number, special char)
- Server count validation (prevents resource exhaustion)

✅ **Authentication & Authorization**

- JWT-based authentication with configurable expiration
- Secure HTTP-only cookies
- User-specific cluster access control
- Token validation on all protected endpoints

✅ **Rate Limiting**

- Global rate limiting (configurable requests per minute)
- Stricter auth endpoint limiting (5 requests/minute)
- Per-IP tracking to prevent abuse

✅ **Security Headers**

- X-Frame-Options: DENY (prevents clickjacking)
- X-Content-Type-Options: nosniff (prevents MIME sniffing)
- X-XSS-Protection: enabled
- Content-Security-Policy (CSP)
- Referrer-Policy: strict-origin-when-cross-origin

✅ **Request Protection**

- Request body size limits (10MB max)
- Timeout protection
- Circuit breaker for external services

## 📦 Deployment Options

### Option 1: Docker Compose (Recommended for Single Host)

1. **Set Environment Variables**

Create a `.env` file in the `deployments/` directory:

```bash
# Required
JWT_SECRET=your-super-secret-jwt-key-minimum-32-characters-long
DB_PASSWORD=your-strong-database-password-here

# Optional
ALLOWED_ORIGIN=https://yourdomain.com
API_PORT=8080
```

2. **Deploy Services**

```bash
cd deployments
docker-compose up -d
```

3. **Verify Deployment**

```bash
# Check service health
docker-compose ps

# View logs
docker-compose logs -f autoscaler

# Test health endpoint
curl http://localhost:8080/health
```

### Option 2: Kubernetes (Recommended for Production at Scale)

1. **Create Secrets**

```bash
kubectl create secret generic autoscaler-secrets \
  --from-literal=jwt-secret=$(openssl rand -base64 48) \
  --from-literal=db-password=$(openssl rand -base64 32)
```

2. **Apply Kubernetes Manifests**

```bash
kubectl apply -f deployments/k8s/
```

### Option 3: Manual Binary Deployment

1. **Build Binaries**

```bash
# Build server
go build -ldflags="-w -s" -o bin/autoscaler ./cmd/autoscaler

# Build simulator
go build -ldflags="-w -s" -o bin/simulator ./cmd/simulator
```

2. **Set Environment Variables**

```bash
export JWT_SECRET="your-secret-key"
export DB_PASSWORD="your-db-password"
# ... other environment variables
```

3. **Run Services**

```bash
# Start database (TimescaleDB)
# ... configure your TimescaleDB instance

# Run migrations
./bin/autoscaler -config configs/config.prod.yaml -migrate

# Start simulator
./bin/simulator -port 9000 &

# Start server
./bin/autoscaler -config configs/config.prod.yaml
```

## 🔧 Configuration

### Production Configuration Template

See `configs/config.prod.yaml` for the production configuration template.

**Key Configuration Points:**

```yaml
app:
  mode: production # REQUIRED: Enforces security validations
  log_level: info # Use 'info' or 'warn' in production

database:
  ssl_mode: require # REQUIRED in production
  max_connections: 50 # Adjust based on your load

api:
  jwt_secret: ${JWT_SECRET} # From environment variable
  cookie_secure: true # REQUIRED: Only send over HTTPS
  cookie_http_only: true # REQUIRED: Prevent XSS
  rate_limit: 60 # Requests per minute per IP

  cors:
    allowed_origins:
      - https://yourdomain.com # Specific domain, not '*'
```

### Environment Variable Override

All configuration values can be overridden with environment variables:

```bash
# Format: AUTOSCALER_SECTION_KEY
export AUTOSCALER_APP_MODE=production
export AUTOSCALER_DATABASE_HOST=db.example.com
export AUTOSCALER_API_JWT_SECRET="your-secret"
```

## 🚀 Initial Setup

### 1. Database Migrations

Run migrations automatically on first startup:

```bash
docker-compose run autoscaler -config /app/configs/config.prod.yaml -migrate
```

Or manually:

```bash
./bin/autoscaler -config configs/config.prod.yaml -migrate
```

### 2. Create First User

Use the API to register the first admin user:

```bash
curl -X POST http://localhost:8080/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "SecurePassword123!"
  }'
```

**Password Requirements:**

- Minimum 8 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one number
- At least one special character

### 3. Login and Get Token

```bash
curl -X POST http://localhost:8080/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "SecurePassword123!"
  }'
```

Save the returned JWT token for authenticated requests.

## 🔍 Monitoring & Health Checks

### Health Endpoints

```bash
# Overall health
curl http://localhost:8080/health

# Readiness check (for K8s)
curl http://localhost:8080/health/ready

# Liveness check (for K8s)
curl http://localhost:8080/health/live
```

### Prometheus Metrics

Metrics are exposed on port 9090 (if enabled):

```bash
curl http://localhost:9090/metrics
```

### Logging

Logs are structured JSON in production mode. Configure log aggregation:

- **CloudWatch** (AWS)
- **Stackdriver** (GCP)
- **ELK Stack** (Self-hosted)
- **Loki** (Grafana)

## 🔐 Security Best Practices

### 1. Network Security

- Deploy behind a reverse proxy (nginx, Traefik)
- Enable HTTPS/TLS
- Use firewall rules to restrict database access
- Consider a VPN or private network for internal services

### 2. Secrets Management

- Use a secrets manager (AWS Secrets Manager, HashiCorp Vault, etc.)
- Rotate JWT secrets periodically
- Never log sensitive information
- Use encrypted connections for all external communication

### 3. Database Security

- Enable SSL/TLS for database connections
- Use strong authentication
- Implement connection pooling limits
- Regular backups with encryption
- Apply principle of least privilege for database users

### 4. Application Security

- Keep dependencies updated
- Run security scans regularly
- Implement audit logging
- Monitor for unusual activity
- Set up alerting for security events

## 🚨 Troubleshooting

### Common Issues

**1. JWT Validation Errors**

```
Error: api.jwt_secret must be at least 32 characters in production
```

Solution: Generate a proper JWT secret (see Security Checklist)

**2. Database Connection Failed**

```
Error: failed to ping database
```

Solutions:

- Verify database is running
- Check connection credentials
- Ensure SSL mode matches database configuration
- Verify network connectivity

**3. Rate Limit Exceeded**

```
Error: too many authentication attempts
```

Solution: Wait 60 seconds before trying again (auth endpoints: 5 req/min)

**4. CORS Errors**

```
Error: CORS policy blocked
```

Solution: Add your frontend domain to `allowed_origins` in config

### Debugging

Enable debug logging temporarily:

```bash
export AUTOSCALER_APP_LOG_LEVEL=debug
```

Check application logs:

```bash
# Docker
docker-compose logs -f autoscaler

# Manual deployment
tail -f logs/autoscaler.log
```

## 📊 Performance Tuning

### Database

```yaml
database:
  max_connections: 50 # Increase for high load
  conn_max_lifetime: 30m
  conn_max_idle_time: 5m
```

### API Server

```yaml
api:
  read_timeout: 30s
  write_timeout: 30s
  idle_timeout: 120s
  rate_limit: 100 # Adjust based on expected traffic
```

### WebSocket

```yaml
websocket:
  max_connections: 1000 # Maximum concurrent WebSocket connections
  ping_interval: 30s
  max_message_size: 1048576 # 1MB
```

## 🔄 Updates & Maintenance

### Zero-Downtime Updates

1. Build new Docker images
2. Update one instance at a time
3. Wait for health checks to pass
4. Continue with remaining instances

### Database Migrations

Always test migrations in a staging environment first:

```bash
# Backup database
pg_dump autoscaler > backup_$(date +%Y%m%d).sql

# Run migrations
./bin/autoscaler -config configs/config.prod.yaml -migrate

# Verify
curl http://localhost:8080/health
```

### Backup Strategy

- **Database**: Daily automated backups with 30-day retention
- **Configuration**: Version control (Git)
- **Secrets**: Secure backup in secrets manager

## 📞 Support

For issues or questions:

1. Check logs for error details
2. Review this guide and configuration
3. Search existing issues on GitHub
4. Create a new issue with:
   - Error messages
   - Configuration (redact secrets!)
   - Steps to reproduce

## 🔗 Additional Resources

- [API Documentation](http://localhost:8080/swagger/index.html)
- [TimescaleDB Documentation](https://docs.timescale.com/)
- [Gin Framework Documentation](https://gin-gonic.com/docs/)
- [JWT Best Practices](https://tools.ietf.org/html/rfc8725)
