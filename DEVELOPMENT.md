# Development Environment Setup

This guide will help you set up a local development environment for the Larsa Academy Microservice.

## 🚀 Quick Start (Recommended)

### Option 1: Docker Compose (Easiest)

1. **Prerequisites**:
   - Docker and Docker Compose installed
   - GitLab access token for private dependencies

2. **Setup**:
   ```bash
   # Clone the repository
   git clone https://git.larsa.io/larsa/larsa-academy-microservice.git
   cd larsa-academy-microservice

   # Set your GitLab access token
   export GITLAB_ACCESS_TOKEN=your_token_here

   # Start all services
   docker-compose up -d

   # View logs
   docker-compose logs -f larsa-academy
   ```

3. **Services Available**:
   - **API**: http://localhost:3278
   - **MongoDB**: localhost:27017
   - **Redis**: localhost:6379
   - **NATS**: localhost:4222
   - **Mongo Express**: http://localhost:8081 (admin/admin)
   - **Redis Commander**: http://localhost:8082

### Option 2: Development with Hot Reload

```bash
# Start infrastructure services only
docker-compose up -d mongodb redis nats

# Run the application locally with hot reload
export GITLAB_ACCESS_TOKEN=your_token_here
export MONGODB_URI=mongodb://localhost:27017/larsa_academy_dev
export REDIS_URL=redis://localhost:6379
export NATS_URL=nats://localhost:4222

# Install Air for hot reload (if not installed)
go install github.com/air-verse/air@latest

# Start with hot reload
air -c .air.toml
```

## 🛠️ Manual Setup

### Prerequisites

- **Go 1.24+**: [Download](https://golang.org/dl/)
- **MongoDB 7.0+**: [Installation Guide](https://docs.mongodb.com/manual/installation/)
- **Redis 7.2+**: [Installation Guide](https://redis.io/docs/getting-started/installation/)
- **NATS Streaming**: [Installation Guide](https://docs.nats.io/running-a-nats-service/introduction/installation)

### Step-by-Step Setup

1. **Install Go Dependencies**:
   ```bash
   # Set up Git access for private repositories
   git config --global url."https://oauth2:YOUR_TOKEN@git.larsa.io/".insteadOf "https://git.larsa.io/"
   
   # Install dependencies
   go mod download
   ```

2. **Start Infrastructure Services**:

   **MongoDB**:
   ```bash
   # Start MongoDB
   mongod --dbpath /data/db
   
   # Initialize database (optional)
   mongo < scripts/mongo-init.js
   ```

   **Redis**:
   ```bash
   # Start Redis
   redis-server
   ```

   **NATS Streaming**:
   ```bash
   # Start NATS Streaming
   nats-streaming-server -p 4222 -m 8222 -hbi 5s -hbt 5s -hbf 2 -SD -cid larsa-cluster
   ```

3. **Configure Environment**:
   ```bash
   # Create .env file
   cat > .env << EOF
   PORT=3278
   MONGODB_URI=mongodb://localhost:27017/larsa_academy_dev
   REDIS_URL=redis://localhost:6379
   NATS_URL=nats://localhost:4222
   IS_LOCAL=true
   GO_ENV=development
   EOF
   ```

4. **Run the Application**:
   ```bash
   # Development mode with hot reload
   air -c .air.toml
   
   # Or run directly
   go run main.go
   ```

## 🔧 Development Tools

### Recommended VS Code Extensions

```json
{
  "recommendations": [
    "golang.go",
    "ms-vscode.vscode-json",
    "redhat.vscode-yaml",
    "ms-vscode-remote.remote-containers",
    "mongodb.mongodb-vscode",
    "humao.rest-client"
  ]
}
```

### IDE Configuration

Create `.vscode/settings.json`:
```json
{
  "go.formatTool": "goimports",
  "go.lintTool": "golangci-lint",
  "go.testFlags": ["-v", "-race"],
  "go.buildFlags": ["-race"],
  "go.vetFlags": ["-all"],
  "editor.formatOnSave": true,
  "editor.codeActionsOnSave": {
    "source.organizeImports": true
  }
}
```

### Useful Development Commands

```bash
# Format code
go fmt ./...

# Run linter
golangci-lint run

# Run tests
go test ./... -v -race

# Run tests with coverage
go test ./... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out

# Build binary
go build -o bin/app main.go

# Clean module cache
go clean -modcache

# Update dependencies
go mod tidy
go mod vendor  # if using vendor directory
```

## 🐛 Debugging

### VS Code Debugging Configuration

Create `.vscode/launch.json`:
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch Server",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/main.go",
      "env": {
        "PORT": "3278",
        "MONGODB_URI": "mongodb://localhost:27017/larsa_academy_dev",
        "REDIS_URL": "redis://localhost:6379",
        "NATS_URL": "nats://localhost:4222",
        "IS_LOCAL": "true"
      },
      "args": [],
      "console": "integratedTerminal"
    }
  ]
}
```

### Delve Debugger

```bash
# Install Delve
go install github.com/go-delve/delve/cmd/dlv@latest

# Debug with Delve
dlv debug main.go

# Debug with environment variables
dlv debug --build-flags="-buildmode=exe" main.go -- --env-file=.env
```

## 📊 Monitoring and Observability

### Local Monitoring Setup

1. **Application Metrics**:
   ```bash
   # Health check
   curl http://localhost:3278/health
   
   # Application metrics (if implemented)
   curl http://localhost:3278/metrics
   ```

2. **Infrastructure Monitoring**:
   ```bash
   # MongoDB stats
   docker exec mongodb mongo --eval "db.stats()"
   
   # Redis info
   docker exec redis redis-cli info
   
   # NATS monitoring
   curl http://localhost:8222/varz
   ```

### Log Analysis

```bash
# Application logs
docker-compose logs -f larsa-academy

# Database logs
docker-compose logs -f mongodb

# Infrastructure logs
docker-compose logs -f redis nats
```

## 🧪 Testing

### Running Tests

```bash
# Unit tests
go test ./pkg/... -v

# Integration tests
go test ./tests/integration/... -v

# Specific package tests
go test ./pkg/services/... -v -run TestSpecificFunction

# Test with coverage
go test ./... -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
```

### Test Data Setup

```bash
# Load test data
mongo larsa_academy_dev < tests/fixtures/test_data.js

# Reset test database
mongo larsa_academy_dev --eval "db.dropDatabase()"
```

## 🔒 Security Considerations

### Local Development Security

1. **Environment Variables**: Never commit sensitive data
2. **Database Access**: Use strong passwords in production
3. **Token Management**: Rotate access tokens regularly
4. **CORS Configuration**: Restrict origins in production

### Git Hooks (Recommended)

Create `.git/hooks/pre-commit`:
```bash
#!/bin/sh
# Run tests before commit
go test ./...
if [ $? -ne 0 ]; then
    echo "Tests failed. Commit aborted."
    exit 1
fi

# Run linter
golangci-lint run
if [ $? -ne 0 ]; then
    echo "Linting failed. Commit aborted."
    exit 1
fi
```

## 🚨 Troubleshooting

### Common Issues

1. **Cannot connect to MongoDB**:
   ```bash
   # Check if MongoDB is running
   docker ps | grep mongodb
   
   # Check connection
   mongo mongodb://localhost:27017/larsa_academy_dev
   ```

2. **Redis connection failed**:
   ```bash
   # Test Redis connection
   redis-cli ping
   
   # Check Redis logs
   docker logs redis
   ```

3. **NATS connection issues**:
   ```bash
   # Check NATS status
   curl http://localhost:8222/varz
   
   # View NATS logs
   docker logs nats
   ```

4. **Module download issues**:
   ```bash
   # Clear module cache
   go clean -modcache
   
   # Verify Git access
   git ls-remote https://git.larsa.io/mahdawi/microservices-commons.git
   ```

5. **Port already in use**:
   ```bash
   # Find process using port
   lsof -i :3278
   
   # Kill process
   kill -9 <PID>
   ```

### Performance Issues

1. **Slow database queries**: Check MongoDB indexes
2. **Memory leaks**: Use `go tool pprof` for profiling
3. **High CPU usage**: Monitor goroutine count

### Getting Help

- **Documentation**: Check `docs/` folder
- **Issues**: Create GitLab issue with reproduction steps
- **Team Chat**: Internal Slack channels
- **Code Review**: Request review from team members

---

Happy coding! 🚀
