# larsa-academy-microservice

Academy microservice for the Larsa learning platform.

**Stack:** Go, MongoDB, Redis, NATS

> Built by the Larsa engineering team and published with the company's permission. My contribution: 153 of 693 commits. This is a clean snapshot: git history is removed and secrets are replaced with placeholders.

## Setup

```bash
go run .
```

A Go-based microservice for the Larsa Academy platform, built with clean architecture principles and modern Go practices.

## 🏗️ Architecture Overview

This microservice follows a clean architecture pattern with clear separation of concerns:

```
pkg/
├── api/           # HTTP server setup and routing
├── gateway/       # External service communication
├── services/      # Business logic organized by domain
│   ├── contact/   # Contact management (students, teachers, etc.)
│   ├── content/   # Content management
│   ├── db/        # Database utilities and sorting
│   ├── messaging/ # Email and notification services
│   ├── sales/     # Sales and enrollment
│   └── support/   # Support and help desk
├── middleware/    # HTTP middleware (auth, logging, etc.)
├── types/         # Domain models and DTOs
├── helpers/       # Utility functions and response helpers
├── util/          # Common utilities
├── nats/          # Message queue integration
├── caching/       # Redis caching layer
├── query/         # Query building and conditions
└── transl/        # Internationalization with localization
```

## 🚀 Quick Start

### Prerequisites

- Go 1.24+
- MongoDB
- Redis
- NATS Streaming Server
- Docker (optional)

### Local Development Setup

1. **Clone the repository**
   ```bash
   git clone https://git.larsa.io/larsa/larsa-academy-microservice.git
   cd larsa-academy-microservice
   ```

2. **Install dependencies**
   ```bash
   go mod download
   ```

3. **Set up environment variables**
   Create a `.env` file in the root directory:
   ```env
   PORT=3278
   MONGODB_URI=mongodb://localhost:27017
   REDIS_URL=redis://localhost:6379
   NATS_URL=nats://localhost:4222
   # Add other required environment variables
   ```

4. **Start the service**
   ```bash
   go run main.go
   ```

### Docker Setup

1. **Build the Docker image**
   ```bash
   docker build --build-arg GITLAB_ACCESS_TOKEN=your_token -t larsa-academy .
   ```

2. **Run with Docker Compose** (recommended)
   ```bash
   docker-compose up -d
   ```

## 📁 Project Structure

### Key Components

- **Dependency Injection**: Uses `github.com/samber/do` for clean dependency management
- **HTTP Router**: Built on `chi/v5` for lightweight, fast routing
- **Database**: MongoDB with transaction support
- **Caching**: Redis for performance optimization
- **Messaging**: NATS Streaming for event-driven architecture
- **Validation**: `go-playground/validator` for request validation

### Service Initialization Order

⚠️ **Important**: The order of service initialization matters. Do not change the order in `pkg/api/api.go`:

1. Database connection
2. Validator instance
3. Gateway services
4. Messaging services
5. User behavior services
6. Automation services

## 🔧 Development Guidelines

### Adding a New Service

1. Create the domain directory in `pkg/services/your-domain/`
2. Create service files (e.g., `your-entity.go`) with service interface and implementation
3. Add models in `pkg/services/your-domain/models/`
4. Add repository in `pkg/services/your-domain/repo/` extending `MainRepo[T]`
5. Add handlers in `pkg/services/your-domain/handler/`
6. Add enums in `pkg/services/your-domain/enums/` if needed
7. Create DI setup in `pkg/services/your-domain/di/` and register all components
8. Register in the main DI container following the established pattern

### Adding New Endpoints

1. Create handler functions in `pkg/services/domain/handler/`
2. Use `helpers.Make()` wrapper for error handling
3. Define routes in the service's DI initialization with Chi router
4. Use middleware for authentication (`middleware.Auth("authenticate")`)
5. Follow the established patterns: `helpers.WriteJsonCtx()` for responses
6. Use context with `util.AddCtxAppCfg(r)` for request context

### Authentication

- Use `middleware.Auth()` for protected endpoints
- Use `middleware.OptionalAuth()` for endpoints that work with/without auth
- Access user context via `util.GetReqUser(ctx)`

## 📋 Available Scripts

```bash
# Development
go run main.go                 # Start the server
go mod tidy                    # Clean up dependencies
go fmt ./...                   # Format code
go vet ./...                   # Static analysis

# Testing
go test ./...                  # Run tests
go test -race ./...            # Run tests with race detection
go test -cover ./...           # Run tests with coverage

# Build
go build -o bin/app main.go    # Build binary
```

## 🌍 Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `PORT` | Server port | `3278` | No |
| `MONGODB_URI` | MongoDB connection string | - | Yes |
| `REDIS_URL` | Redis connection string | - | Yes |
| `NATS_URL` | NATS server URL | - | Yes |
| `IS_LOCAL` | Local development flag | `false` | No |

## 📚 Additional Documentation

- [Contributing Guidelines](CONTRIBUTING.md)
- [Architecture Details](ARCHITECTURE.md)
- [API Documentation](docs/api.md)

## 🤝 Contributing

Please read [CONTRIBUTING.md](CONTRIBUTING.md) for details on our code of conduct and the process for submitting pull requests.

## 📞 Support

- **Issues**: Use GitLab Issues for bug reports and feature requests
- **Documentation**: Check the `docs/` folder for detailed guides
- **Team**: Contact the development team via internal Slack channels

## 📝 Todo

See the [todo](todo) file for current development priorities and known issues.

---

**Note**: This microservice is part of the larger Larsa Academy platform. Ensure changes are compatible with other microservices in the ecosystem.
