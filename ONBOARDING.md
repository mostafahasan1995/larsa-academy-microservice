# Developer Onboarding Guide

Welcome to the Larsa Academy Microservice project! This guide will help you get up to speed quickly and become a productive member of our development team.

## 📋 Prerequisites Checklist

Before you start, ensure you have:

- [ ] Go 1.24+ installed
- [ ] Docker and Docker Compose installed
- [ ] Git configured with access to GitLab
- [ ] GitLab access token for private dependencies
- [ ] VS Code or preferred Go IDE
- [ ] Basic understanding of Go, MongoDB, and microservices

## 🚀 Quick Start (15 minutes)

### 1. Clone and Setup (5 minutes)

```bash
# Clone the repository
git clone https://git.larsa.io/larsa/larsa-academy-microservice.git
cd larsa-academy-microservice

# Set up environment
export GITLAB_ACCESS_TOKEN=your_token_here

# Start the development environment
docker-compose up -d
```

### 2. Verify Setup (5 minutes)

```bash
# Check if services are running
docker-compose ps

# Test the API
curl http://localhost:3278/health

# Check logs
docker-compose logs -f larsa-academy
```

### 3. Explore the Code (5 minutes)

- Read the [README.md](README.md) for project overview
- Browse the `pkg/` directory structure
- Look at `examples/actual-service-example.go` for patterns
- Check `pkg/api/api.go` for service initialization

## 📚 Essential Reading (First Day)

### Must Read (Priority 1)
1. **[README.md](README.md)** - Project overview and setup
2. **[ARCHITECTURE.md](ARCHITECTURE.md)** - System architecture and design
3. **[CONTRIBUTING.md](CONTRIBUTING.md)** - Development guidelines
4. **[docs/PATTERNS.md](docs/PATTERNS.md)** - Code patterns and best practices

### Should Read (Priority 2)
1. **[DEVELOPMENT.md](DEVELOPMENT.md)** - Development environment details
2. **[docs/api.md](docs/api.md)** - API documentation
3. **[examples/actual-service-example.go](examples/actual-service-example.go)** - Complete service example

## 🏗️ Understanding the Architecture

### Key Concepts

1. **Clean Architecture**: We separate concerns into layers
2. **Dependency Injection**: Using `samber/do` for loose coupling
3. **Repository Pattern**: Abstracting data access
4. **Service Layer**: Business logic encapsulation
5. **Middleware**: Cross-cutting concerns (auth, logging, etc.)

### Project Structure Overview

```
pkg/
├── api/          # HTTP server and routing
├── services/     # Business logic organized by domain (your main work area)
│   ├── contact/  # Contact management (students, teachers, etc.)
│   ├── content/  # Content management
│   ├── db/       # Database utilities and sorting
│   ├── messaging/# Email and notification services
│   ├── sales/    # Sales and enrollment
│   └── support/  # Support and help desk
├── middleware/   # HTTP middleware
├── gateway/      # External service communication
├── types/        # Domain models and DTOs
├── helpers/      # Utility functions and response helpers
├── util/         # Common utilities
├── query/        # Query building and conditions
└── transl/       # Internationalization with localization
```

## 🛠️ Development Workflow

### Your First Code Change

1. **Create a feature branch**:
   ```bash
   git checkout -b feature/your-feature-name
   ```

2. **Make your changes** following our patterns

3. **Test your changes**:
   ```bash
   go test ./...
   go fmt ./...
   go vet ./...
   ```

4. **Commit with conventional format**:
   ```bash
   git commit -m "feat(service): add new feature description"
   ```

5. **Create a merge request**

### Daily Development

1. **Start your environment**:
   ```bash
   docker-compose up -d mongodb redis nats
   air -c .air.toml  # Hot reload
   ```

2. **Run tests frequently**:
   ```bash
   go test ./pkg/services/your-service/...
   ```

3. **Check code quality**:
   ```bash
   golangci-lint run
   ```

## 📝 Common Tasks

### Adding a New Service

1. **Create the directory structure**:
   ```bash
   mkdir -p pkg/services/your-domain/{di,enums,handler,models,repo}
   ```

2. **Follow the established patterns** (see `examples/actual-service-example.go`)

3. **Add dependency injection setup**

4. **Register in main DI container**

5. **Add tests**

### Adding a New Endpoint

1. **Define the handler in your service**
2. **Add route in DI setup**
3. **Use appropriate middleware**
4. **Update API documentation**
5. **Add tests**

### Database Changes

1. **Update models in `models/` directory**
2. **Add/update repository methods**
3. **Create migration scripts if needed**
4. **Update indexes in `mongo-init.js`**

## 🧪 Testing Guidelines

### Test Types

1. **Unit Tests**: Test business logic in isolation
2. **Integration Tests**: Test with real database
3. **API Tests**: Test HTTP endpoints

### Running Tests

```bash
# All tests
go test ./...

# Specific package
go test ./pkg/services/your-service/...

# With coverage
go test ./... -cover

# Integration tests only
go test ./tests/integration/...
```

### Writing Tests

Follow the pattern: `TestFunction_Scenario_ExpectedResult`

```go
func TestUserService_CreateUser_ValidInput_ReturnsUser(t *testing.T) {
    // Arrange, Act, Assert
}
```

## 🔧 Tools and Commands

### Essential Commands

```bash
# Development
go run main.go                 # Start server
air -c .air.toml              # Hot reload
go mod tidy                   # Clean dependencies

# Code Quality
go fmt ./...                  # Format code
go vet ./...                  # Static analysis
golangci-lint run            # Comprehensive linting

# Testing
go test ./...                 # Run tests
go test -race ./...           # Race detection
go test -cover ./...          # Coverage report

# Docker
docker-compose up -d          # Start services
docker-compose logs -f app    # View logs
docker-compose down           # Stop services
```

### Useful VS Code Extensions

- Go (official Go extension)
- MongoDB for VS Code
- REST Client
- GitLens
- Docker
- Thunder Client (API testing)

## 🔍 Debugging Tips

### Common Issues

1. **Service won't start**:
   - Check if ports are available
   - Verify environment variables
   - Check Docker services are running

2. **Database connection failed**:
   - Ensure MongoDB is running
   - Check connection string
   - Verify network connectivity

3. **Tests failing**:
   - Run `go mod tidy`
   - Check test database setup
   - Verify mock configurations

### Debugging Tools

```bash
# Check service health
curl http://localhost:3278/health

# MongoDB connection
mongo mongodb://localhost:27017/larsa_academy_dev

# Redis connection
redis-cli -h localhost -p 6379 ping

# NATS monitoring
curl http://localhost:8222/varz
```

## 📖 Learning Path

### Week 1: Foundation
- [ ] Complete environment setup
- [ ] Read all essential documentation
- [ ] Understand project structure
- [ ] Make your first small change
- [ ] Write your first test

### Week 2: Deep Dive
- [ ] Implement a simple service
- [ ] Add new endpoints
- [ ] Work with database operations
- [ ] Understand middleware usage
- [ ] Practice testing patterns

### Week 3: Advanced
- [ ] Implement complex business logic
- [ ] Add authentication/authorization
- [ ] Work with external services
- [ ] Optimize performance
- [ ] Handle error scenarios

### Week 4: Mastery
- [ ] Design new features
- [ ] Review other developers' code
- [ ] Mentor new team members
- [ ] Contribute to architecture decisions

## 🤝 Getting Help

### When You're Stuck

1. **Check documentation** first
2. **Search existing issues** in GitLab
3. **Ask in team chat** for quick questions
4. **Create GitLab issue** for bugs or feature requests
5. **Schedule pairing session** for complex problems

### Resources

- **Project Documentation**: All docs in the repository
- **Team Chat**: Internal Slack channels
- **Code Reviews**: Learn from MR feedback
- **Architecture Discussions**: Team meetings
- **External Resources**: Go documentation, MongoDB docs, etc.

### Team Contacts

- **Tech Lead**: For architecture questions
- **Senior Developers**: For implementation guidance
- **DevOps**: For infrastructure and deployment
- **Product**: For business requirements

## ✅ Onboarding Checklist

### First Day
- [ ] Environment setup completed
- [ ] Read essential documentation
- [ ] Understand project structure
- [ ] Identify team contacts
- [ ] Join relevant chat channels

### First Week
- [ ] Make first code contribution
- [ ] Understand testing approach
- [ ] Familiar with development workflow
- [ ] Completed first code review
- [ ] Understand deployment process

### First Month
- [ ] Implemented complete feature
- [ ] Comfortable with all patterns
- [ ] Can help onboard others
- [ ] Contributing to architecture discussions
- [ ] Mentoring new developers

## 🎯 Success Metrics

You're successfully onboarded when you can:

- [ ] Set up the development environment independently
- [ ] Navigate the codebase confidently
- [ ] Follow established patterns consistently
- [ ] Write tests for your code
- [ ] Contribute to code reviews
- [ ] Debug issues effectively
- [ ] Help onboard new team members

---

Welcome to the team! We're excited to have you aboard. Don't hesitate to ask questions – we're here to help you succeed! 🚀
