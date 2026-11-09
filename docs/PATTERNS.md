# Development Patterns and Best Practices

This document outlines the key patterns, conventions, and best practices used in the Larsa Academy Microservice project.

## 🏗️ Architecture Patterns

### 1. Clean Architecture

The project follows Clean Architecture principles with clear layer separation:

```
┌─────────────────────────────────────────────────────────────┐
│                    Presentation Layer                       │
│                 (HTTP Handlers, Middleware)                 │
├─────────────────────────────────────────────────────────────┤
│                     Application Layer                       │
│                   (Services, Use Cases)                     │
├─────────────────────────────────────────────────────────────┤
│                      Domain Layer                           │
│                 (Entities, Business Rules)                  │
├─────────────────────────────────────────────────────────────┤
│                   Infrastructure Layer                      │
│             (Repositories, External Services)               │
└─────────────────────────────────────────────────────────────┘
```

**Benefits:**
- **Testability**: Easy to mock dependencies
- **Maintainability**: Clear separation of concerns
- **Flexibility**: Easy to change implementations
- **Independence**: Business logic doesn't depend on frameworks

### 2. Dependency Injection Pattern

Using `github.com/samber/do` for dependency management:

```go
// Service registration
func Init(i *do.Injector) *do.Injector {
    do.Provide(i, NewRepository)
    do.Provide(i, NewService)
    do.Provide(i, NewHandlers)
    return i
}

// Service constructor
func NewService(i *do.Injector) (Service, error) {
    repo := do.MustInvoke[Repository](i)
    validator := do.MustInvoke[*validator.Validate](i)
    
    return &serviceImpl{
        repo:      repo,
        validator: validator,
    }, nil
}
```

**Key Rules:**
- Always use interfaces for dependencies
- Register dependencies in DI container
- Use constructor functions for initialization
- Follow the established initialization order

### 3. Repository Pattern

Uses generic base repository with domain-specific extensions:

```go
// Extend the generic MainRepo interface
type StudentRepo interface {
    dbrepo.MainRepo[models.Student]
}

// Implementation embeds generic implementation
type studentrepo struct {
    dbrepo.MainRepoImpl[models.Student]
}

// Constructor with DI
func NewStudentRepo(i *do.Injector) (StudentRepo, error) {
    return &studentrepo{
        MainRepoImpl: dbrepo.MainRepoImpl[models.Student]{
            Db:       do.MustInvoke[*mongo.Client](i),
            CollName: "academyStudents",
        },
    }, nil
}
```

**Key Features:**
- Generic CRUD operations with type safety
- MongoDB aggregation pipeline support
- Bulk operations and transactions
- Context-aware database name resolution
- Automatic soft delete support

## 🔧 Code Organization Patterns

### 1. Package Structure

```
pkg/services/your-domain/
├── di/
│   └── domain.di.go        # Dependency injection setup
├── enums/
│   └── entity.go           # Domain enums (statuses, types, etc.)
├── handler/
│   └── entity.handler.go   # HTTP handlers
├── models/
│   └── entity.model.go     # Domain models and DTOs
├── repo/
│   ├── entity.repo.go      # Repository implementation
│   └── entity.repo_test.go # Repository tests
├── entity.go               # Service interface & implementation
├── entity_test.go          # Service tests
└── other-entity.go         # Additional entities in same domain
```

### 2. Naming Conventions

**Files:**
- `entity.go` - Service interface and implementation (e.g., `student.go`)
- `entity.repo.go` - Repository implementation (e.g., `student.repo.go`)
- `entity.handler.go` - HTTP handlers (e.g., `student.handler.go`)
- `entity.model.go` - Domain models and DTOs (e.g., `student.model.go`)
- `domain.di.go` - Dependency injection setup (e.g., `contact.di.go`)

**Types:**
```go
// Service interfaces: Use domain + "Svcs" suffix
type StudentSvcs interface { ... }
type MessagingSvcs interface { ... }

// Repository interfaces: Use entity + "Repo" suffix
type StudentRepo interface { ... }

// Implementations: Use lowercase
type studentSvcs struct { ... }
type studentrepo struct { ... }

// DTOs: Use entity + "Dto" suffix
type StudentDto struct { ... }

// Domain entities: Use entity name
type Student struct { ... }

// Responses: Use descriptive names with "Pagination" for lists
type StudentsPagination struct { ... }
```

### 3. Error Handling Pattern

```go
// Wrap errors with context
func (s *service) GetUser(ctx context.Context, id string) (*User, error) {
    user, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return nil, fmt.Errorf("failed to get user %s: %w", id, err)
    }
    return user, nil
}

// Use custom error types for business logic
var (
    ErrUserNotFound = errors.New("user not found")
    ErrInvalidInput = errors.New("invalid input")
    ErrUnauthorized = errors.New("unauthorized")
)
```

## 🌐 HTTP Patterns

### 1. Handler Pattern

```go
// Handler with error return for helpers.Make() wrapper
func (h *StudentHandler) Add(w http.ResponseWriter, r *http.Request) error {
    // 1. Add app configuration to context
    ctx, _ := util.AddCtxAppCfg(r)

    // 2. Parse and validate input
    var data models.StudentDto
    if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
        return err
    }

    // 3. Validate using struct tags
    if err := h.validationInstance.Struct(data); err != nil {
        return helpers.ValidationError(err.Error())
    }

    // 4. Call service layer
    result, err := h.studentsvcs.Add(ctx, &data)
    if err != nil {
        return err
    }

    // 5. Return JSON response
    return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Route registration in constructor
func NewStudentHandler(i *do.Injector, r *chi.Mux) {
    h := &StudentHandler{
        studentsvcs:        do.MustInvoke[contact.StudentSvcs](i),
        validationInstance: do.MustInvoke[*validator.Validate](i),
    }

    r.Route("/students", func(r chi.Router) {
        r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
    })
}
```

### 2. Middleware Pattern

```go
func AuthMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // Extract and validate token
        token := extractToken(r)
        user, err := validateToken(token)
        if err != nil {
            http.Error(w, "Unauthorized", http.StatusUnauthorized)
            return
        }

        // Add user to context
        ctx := context.WithValue(r.Context(), "user", user)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

### 3. Route Organization

```go
func InitRoutes(i *do.Injector, r *chi.Mux) {
    handlers := do.MustInvoke[*Handlers](i)
    
    r.Route("/api/v1/users", func(r chi.Router) {
        // Public endpoints
        r.Post("/register", handlers.Register)
        r.Post("/login", handlers.Login)
        
        // Protected endpoints
        r.Group(func(r chi.Router) {
            r.Use(middleware.Auth())
            r.Get("/profile", handlers.GetProfile)
            r.Put("/profile", handlers.UpdateProfile)
        })
        
        // Admin endpoints
        r.Group(func(r chi.Router) {
            r.Use(middleware.Auth("admin"))
            r.Get("/", handlers.ListUsers)
            r.Delete("/{id}", handlers.DeleteUser)
        })
    })
}
```

## 💾 Data Patterns

### 1. Model Definition

```go
// DTO for input/output
type StudentDto struct {
    FirstName   transl.Localizable[string] `bson:"firstName" json:"firstName"`
    LastName    transl.Localizable[string] `bson:"lastName" json:"lastName"`
    Gender      string                     `bson:"gender" json:"gender"`
    DateOfBirth time.Time                  `bson:"dateOfBirth" json:"dateOfBirth"`
    Status      enums.StudentStatus        `bson:"status" json:"status"`
    Picture     types.FileField            `bson:"picture" json:"picture"`
}

// Domain entity embedding DTO
type Student struct {
    Id         primitive.ObjectID `bson:"_id" json:"id"`
    StudentDto `bson:",inline"`   // Embed DTO
    CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
    UpdatedAt  time.Time          `bson:"updatedAt" json:"updatedAt"`
    CreatedBy  primitive.ObjectID `bson:"createdBy" json:"createdBy"`
    UpdatedBy  primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
    Trash      bool               `bson:"trash" json:"trash"` // Soft delete
}

// Pagination response
type StudentsPagination struct {
    Students   []Student        `bson:"students" json:"students"`
    Pagination types.Pagination `bson:"pagination" json:"pagination"`
}
```

### 2. Validation Pattern

```go
// Validation in DTO struct tags
type StudentDto struct {
    FirstName   transl.Localizable[string] `bson:"firstName" json:"firstName" validate:"required"`
    LastName    transl.Localizable[string] `bson:"lastName" json:"lastName" validate:"required"`
    Gender      string                     `bson:"gender" json:"gender" validate:"required,oneof=male female"`
    DateOfBirth time.Time                  `bson:"dateOfBirth" json:"dateOfBirth" validate:"required"`
    Status      enums.StudentStatus        `bson:"status" json:"status" validate:"oneof=active inactive"`
}

// Validation in handler (not service)
func (h *StudentHandler) Add(w http.ResponseWriter, r *http.Request) error {
    var data models.StudentDto
    if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
        return err
    }

    // Validate using struct tags
    if err := h.validationInstance.Struct(data); err != nil {
        return helpers.ValidationError(err.Error())
    }

    // Service handles business logic only
    result, err := h.studentsvcs.Add(ctx, &data)
    if err != nil {
        return err
    }

    return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
```

### 3. Database Transaction Pattern

```go
func (r *repository) CreateUserWithProfile(ctx context.Context, user *User, profile *Profile) error {
    return r.db.WithTransaction(ctx, func(tx *db.Transaction) error {
        if err := tx.Collection("users").InsertOne(ctx, user); err != nil {
            return err
        }
        
        profile.UserID = user.ID
        if err := tx.Collection("profiles").InsertOne(ctx, profile); err != nil {
            return err
        }
        
        return nil
    })
}
```

## 🧪 Testing Patterns

### 1. Unit Test Structure

```go
func TestUserService_CreateUser_ValidInput_ReturnsUser(t *testing.T) {
    // Arrange
    mockRepo := &MockUserRepository{}
    service := NewUserService(mockRepo, validator.New())
    req := CreateUserRequest{
        Email: "test@example.com",
        Name:  "Test User",
    }

    // Act
    user, err := service.CreateUser(context.Background(), req)

    // Assert
    assert.NoError(t, err)
    assert.Equal(t, req.Email, user.Email)
    assert.Equal(t, req.Name, user.Name)
    mockRepo.AssertExpectations(t)
}
```

### 2. Integration Test Pattern

```go
func TestUserRepository_Integration(t *testing.T) {
    // Setup test database
    db := setupTestDB(t)
    defer cleanupTestDB(t, db)
    
    repo := NewUserRepository(db)
    
    // Test create
    user := &User{
        Email: "test@example.com",
        Name:  "Test User",
    }
    
    err := repo.Create(context.Background(), user)
    assert.NoError(t, err)
    assert.NotZero(t, user.ID)
}
```

### 3. Mock Pattern

```go
type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) Create(ctx context.Context, user *User) error {
    args := m.Called(ctx, user)
    return args.Error(0)
}

func (m *MockUserRepository) GetByID(ctx context.Context, id primitive.ObjectID) (*User, error) {
    args := m.Called(ctx, id)
    return args.Get(0).(*User), args.Error(1)
}
```

## 🔐 Security Patterns

### 1. Authentication Context

```go
// Extract user from context
func GetUserFromContext(ctx context.Context) (*User, error) {
    user, ok := ctx.Value("user").(*User)
    if !ok {
        return nil, errors.New("user not found in context")
    }
    return user, nil
}

// Use in service layer
func (s *service) GetUserProfile(ctx context.Context) (*Profile, error) {
    user, err := GetUserFromContext(ctx)
    if err != nil {
        return nil, err
    }
    
    return s.repo.GetProfileByUserID(ctx, user.ID)
}
```

### 2. Authorization Pattern

```go
func (s *service) UpdateUser(ctx context.Context, id string, req UpdateUserRequest) error {
    user, err := GetUserFromContext(ctx)
    if err != nil {
        return err
    }
    
    // Check if user can update this record
    if !s.canUserUpdate(user, id) {
        return ErrUnauthorized
    }
    
    return s.repo.Update(ctx, id, req)
}

func (s *service) canUserUpdate(user *User, targetID string) bool {
    // Business logic for authorization
    return user.ID.Hex() == targetID || user.HasRole("admin")
}
```

## 📊 Performance Patterns

### 1. Caching Pattern

```go
func (s *service) GetUser(ctx context.Context, id string) (*User, error) {
    // Try cache first
    cacheKey := fmt.Sprintf("user:%s", id)
    if cached, err := s.cache.Get(ctx, cacheKey); err == nil {
        var user User
        if err := json.Unmarshal(cached, &user); err == nil {
            return &user, nil
        }
    }
    
    // Get from database
    user, err := s.repo.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // Cache the result
    if data, err := json.Marshal(user); err == nil {
        s.cache.Set(ctx, cacheKey, data, 5*time.Minute)
    }
    
    return user, nil
}
```

### 2. Pagination Pattern

```go
type ListOptions struct {
    Page   int    `query:"page"`
    Limit  int    `query:"limit"`
    Sort   string `query:"sort"`
    Filter string `query:"filter"`
}

func (s *service) ListUsers(ctx context.Context, opts ListOptions) (*ListUsersResponse, error) {
    // Set defaults
    if opts.Page < 1 {
        opts.Page = 1
    }
    if opts.Limit < 1 || opts.Limit > 100 {
        opts.Limit = 10
    }
    
    users, total, err := s.repo.List(ctx, opts)
    if err != nil {
        return nil, err
    }
    
    return &ListUsersResponse{
        Users: users,
        Pagination: &PaginationInfo{
            Page:       opts.Page,
            Limit:      opts.Limit,
            Total:      total,
            TotalPages: (total + opts.Limit - 1) / opts.Limit,
        },
    }, nil
}
```

## 🚨 Anti-Patterns to Avoid

### ❌ Don't Do This

```go
// 1. Direct database access in handlers
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    // BAD: Direct database access
    user := db.Collection("users").FindOne(...)
}

// 2. Business logic in repositories
func (r *Repository) CreateUser(user *User) error {
    // BAD: Business validation in repository
    if user.Age < 18 {
        return errors.New("user too young")
    }
    return r.db.Create(user)
}

// 3. Ignoring context
func (s *Service) GetUser(id string) (*User, error) {
    // BAD: Not using context
    return s.repo.GetByID(id)
}

// 4. Hardcoded dependencies
func NewService() *Service {
    // BAD: Hardcoded dependencies
    return &Service{
        repo: &Repository{},
        db:   NewDB(),
    }
}
```

### ✅ Do This Instead

```go
// 1. Use service layer in handlers
func (h *Handler) GetUser(w http.ResponseWriter, r *http.Request) {
    user, err := h.service.GetUser(r.Context(), id)
    if err != nil {
        helpers.ErrorResponse(w, err.Error(), http.StatusNotFound)
        return
    }
    helpers.SuccessResponse(w, user)
}

// 2. Keep business logic in services
func (s *Service) CreateUser(ctx context.Context, req CreateUserRequest) (*User, error) {
    // Business validation in service
    if req.Age < 18 {
        return nil, errors.New("user must be 18 or older")
    }
    
    return s.repo.Create(ctx, user)
}

// 3. Always use context
func (s *Service) GetUser(ctx context.Context, id string) (*User, error) {
    return s.repo.GetByID(ctx, id)
}

// 4. Use dependency injection
func NewService(i *do.Injector) (Service, error) {
    repo := do.MustInvoke[Repository](i)
    return &service{repo: repo}, nil
}
```

---

Following these patterns ensures consistency, maintainability, and scalability across the entire codebase.
