# Contributing to Larsa Academy Microservice

Thank you for your interest in contributing to this project! This guide will help you understand our development process and coding standards.

## 🚀 Getting Started

1. **Fork the repository** and clone your fork
2. **Set up your development environment** following the README.md instructions
3. **Create a feature branch** from `main`
4. **Make your changes** following our coding standards
5. **Test your changes** thoroughly
6. **Submit a merge request**

## 📋 Development Workflow

### Branch Naming Convention

- `feature/description` - for new features
- `fix/description` - for bug fixes
- `refactor/description` - for code refactoring
- `docs/description` - for documentation updates

Examples:
- `feature/user-authentication`
- `fix/database-connection-leak`
- `refactor/dependency-injection`

### Commit Message Format

Follow the conventional commit specification:

```
<type>(<scope>): <description>

[optional body]

[optional footer(s)]
```

**Types:**
- `feat`: A new feature
- `fix`: A bug fix
- `docs`: Documentation only changes
- `style`: Changes that do not affect the meaning of the code
- `refactor`: A code change that neither fixes a bug nor adds a feature
- `perf`: A code change that improves performance
- `test`: Adding missing tests or correcting existing tests
- `chore`: Changes to the build process or auxiliary tools

**Examples:**
```
feat(auth): add JWT token validation middleware
fix(db): resolve connection pool exhaustion
docs(api): update endpoint documentation
refactor(services): extract common validation logic
```

## 🏗️ Code Style and Standards

### Go Code Standards

1. **Follow Go conventions**
   - Use `gofmt` to format your code
   - Use `golint` for style checking
   - Use `go vet` for static analysis

2. **Package organization**
   - Keep packages focused and cohesive
   - Use clear, descriptive package names
   - Avoid circular dependencies

3. **Naming conventions**
   - Use camelCase for functions and variables
   - Use PascalCase for exported types and functions
   - Use descriptive names over abbreviations

4. **Error handling**
   - Always handle errors explicitly
   - Use custom error types when appropriate
   - Include context in error messages

5. **Comments and documentation**
   - Add package-level documentation
   - Document exported functions and types
   - Use meaningful comments for complex logic

### Project-Specific Standards

#### Dependency Injection Pattern

```go
// ✅ Good: Follow the established DI pattern
func NewYourService(i *do.Injector) (*YourService, error) {
    repo := do.MustInvoke[YourRepository](i)
    return &YourService{repo: repo}, nil
}

// ❌ Bad: Direct instantiation
func NewYourService() *YourService {
    repo := &YourRepository{}
    return &YourService{repo: repo}
}
```

#### HTTP Handlers

```go
// ✅ Good: Use consistent handler signature and error handling
func (s *Service) HandleEndpoint(w http.ResponseWriter, r *http.Request) {
    // Get user from context if needed
    user := util.GetReqUser(r.Context())
    
    // Validate input
    var req RequestDTO
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        helpers.ErrorResponse(w, "Invalid request", http.StatusBadRequest)
        return
    }
    
    // Business logic
    result, err := s.businessLogic(req)
    if err != nil {
        helpers.ErrorResponse(w, err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Success response
    helpers.SuccessResponse(w, result)
}
```

#### Database Operations

```go
// ✅ Good: Use transactions for multi-operation changes
func (r *Repository) UpdateWithTransaction(ctx context.Context, data UpdateData) error {
    return r.db.WithTransaction(ctx, func(tx *db.Transaction) error {
        if err := tx.Update(collection1, filter1, update1); err != nil {
            return err
        }
        return tx.Update(collection2, filter2, update2)
    })
}
```

#### Service Layer Structure

```go
// ✅ Good: Clear service structure
type YourService struct {
    repo YourRepository
    gateway Gateway
    validator *validator.Validate
}

func (s *YourService) BusinessMethod(ctx context.Context, input Input) (*Output, error) {
    // 1. Validate input
    if err := s.validator.Struct(input); err != nil {
        return nil, fmt.Errorf("validation failed: %w", err)
    }
    
    // 2. Business logic
    processed := s.processInput(input)
    
    // 3. Data access
    result, err := s.repo.Save(ctx, processed)
    if err != nil {
        return nil, fmt.Errorf("failed to save: %w", err)
    }
    
    return result, nil
}
```

## 🧪 Testing Guidelines

### Test Structure

1. **Unit tests** for business logic
2. **Integration tests** for database operations
3. **API tests** for endpoint behavior

### Test Naming

```go
func TestServiceMethod_ValidInput_ReturnsExpectedResult(t *testing.T) {
    // Test implementation
}

func TestServiceMethod_InvalidInput_ReturnsError(t *testing.T) {
    // Test implementation
}
```

### Test Organization

```
pkg/
├── services/
│   └── your-service/
│       ├── service.go
│       ├── service_test.go
│       └── repo/
│           ├── repository.go
│           └── repository_test.go
```

## 📁 File Organization

### Adding New Services

1. Create the service directory: `pkg/services/your-service/`
2. Add the main service file: `service.go`
3. Add dependency injection: `di/service.di.go`
4. Add repository layer: `repo/repository.go`
5. Add models if needed: `models/your-model.go`
6. Add tests: `*_test.go` files

### Directory Structure Template

```
pkg/services/your-service/
├── di/
│   └── service.di.go       # Dependency injection setup
├── models/
│   └── your-model.go       # Service-specific models
├── repo/
│   ├── repository.go       # Data access layer
│   └── repository_test.go  # Repository tests
├── service.go              # Main service logic
├── service_test.go         # Service tests
└── handlers.go             # HTTP handlers (if applicable)
```

## 🔍 Code Review Guidelines

### For Contributors

- **Self-review** your code before submitting
- **Write descriptive** merge request descriptions
- **Include tests** for new functionality
- **Update documentation** if needed
- **Respond promptly** to review feedback

### For Reviewers

- **Be constructive** and helpful in feedback
- **Focus on** code quality, not personal preferences
- **Check for** proper error handling and testing
- **Verify** adherence to project patterns
- **Test locally** if needed

## 🚫 Common Mistakes to Avoid

1. **Changing service initialization order** in `pkg/api/api.go`
2. **Not handling errors** properly
3. **Creating circular dependencies** between packages
4. **Ignoring the dependency injection pattern**
5. **Not adding proper validation** to input data
6. **Hardcoding configuration values** instead of using environment variables
7. **Not following the established project structure**

## 📚 Useful Resources

- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)
- [Effective Go](https://golang.org/doc/effective_go.html)
- [Go Package Layout](https://medium.com/@benbjohnson/standard-package-layout-7cdbc8391fc1)
- [Chi Router Documentation](https://go-chi.io/)
- [Samber/do Documentation](https://github.com/samber/do)

## 🆘 Getting Help

- **Project Documentation**: Check the `docs/` folder
- **Team Communication**: Use internal Slack channels
- **Issues**: Create a GitLab issue for bugs or feature requests
- **Architecture Questions**: Refer to `ARCHITECTURE.md`

## 🎯 Performance Guidelines

- **Database queries**: Use appropriate indexes and avoid N+1 queries
- **Caching**: Leverage Redis for frequently accessed data
- **Memory management**: Be mindful of goroutine and memory usage
- **Error handling**: Don't ignore errors, log appropriately

---

Thank you for contributing to Larsa Academy Microservice! Your efforts help make this project better for everyone.
