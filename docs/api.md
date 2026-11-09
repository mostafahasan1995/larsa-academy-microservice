# API Documentation

This document provides comprehensive information about the Larsa Academy Microservice API endpoints, request/response formats, and authentication requirements.

## 🔐 Authentication

### Bearer Token Authentication

Most endpoints require authentication using JWT tokens in the Authorization header:

```
Authorization: Bearer <your-jwt-token>
```

### Middleware Usage

- `middleware.Auth()` - Required authentication
- `middleware.OptionalAuth()` - Optional authentication (endpoint works with/without auth)

## 📋 Common Response Formats

### Success Response

```json
{
  "success": true,
  "data": {
    // Response data here
  },
  "message": "Operation completed successfully"
}
```

### Error Response

```json
{
  "success": false,
  "error": {
    "code": "ERROR_CODE",
    "message": "Human readable error message"
  }
}
```

### Validation Error Response

```json
{
  "success": false,
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Validation failed",
    "details": [
      {
        "field": "email",
        "message": "Email is required"
      }
    ]
  }
}
```

## 📚 API Endpoints

### Health Check

#### GET /health
**Description**: Check service health status

**Authentication**: None required

**Response**:
```json
{
  "success": true,
  "data": {
    "status": "healthy",
    "timestamp": "2024-01-15T10:30:00Z",
    "version": "1.0.0"
  }
}
```

---

### User Management

#### GET /api/v1/users/profile
**Description**: Get current user profile

**Authentication**: Required

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "60d5ecb54b4b1c001f5b9b8a",
    "email": "user@example.com",
    "name": "John Doe",
    "role": "student",
    "created_at": "2024-01-01T00:00:00Z",
    "updated_at": "2024-01-15T10:30:00Z"
  }
}
```

#### PUT /api/v1/users/profile
**Description**: Update user profile

**Authentication**: Required

**Request Body**:
```json
{
  "name": "John Smith",
  "phone": "+1234567890",
  "preferences": {
    "language": "en",
    "notifications": true
  }
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "60d5ecb54b4b1c001f5b9b8a",
    "email": "user@example.com",
    "name": "John Smith",
    "phone": "+1234567890",
    "updated_at": "2024-01-15T11:00:00Z"
  }
}
```

---

### Course Management

#### GET /api/v1/courses
**Description**: List available courses

**Authentication**: Optional

**Query Parameters**:
- `page` (integer, default: 1) - Page number
- `limit` (integer, default: 10) - Items per page
- `category` (string) - Filter by category
- `search` (string) - Search in title and description

**Example Request**:
```
GET /api/v1/courses?page=1&limit=10&category=programming&search=javascript
```

**Response**:
```json
{
  "success": true,
  "data": {
    "courses": [
      {
        "id": "60d5ecb54b4b1c001f5b9b8b",
        "title": "JavaScript Fundamentals",
        "description": "Learn the basics of JavaScript programming",
        "category": "programming",
        "difficulty": "beginner",
        "duration_hours": 20,
        "price": 99.99,
        "instructor": {
          "name": "Jane Doe",
          "avatar": "https://example.com/avatars/jane.jpg"
        },
        "enrolled_count": 1250,
        "rating": 4.8,
        "created_at": "2024-01-01T00:00:00Z"
      }
    ],
    "pagination": {
      "current_page": 1,
      "total_pages": 5,
      "total_items": 47,
      "items_per_page": 10
    }
  }
}
```

#### GET /api/v1/courses/{id}
**Description**: Get course details

**Authentication**: Optional

**Path Parameters**:
- `id` (string) - Course ID

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "60d5ecb54b4b1c001f5b9b8b",
    "title": "JavaScript Fundamentals",
    "description": "Comprehensive course covering JavaScript basics",
    "long_description": "This course will teach you...",
    "category": "programming",
    "difficulty": "beginner",
    "duration_hours": 20,
    "price": 99.99,
    "instructor": {
      "id": "60d5ecb54b4b1c001f5b9b8c",
      "name": "Jane Doe",
      "bio": "Expert JavaScript developer with 10+ years experience",
      "avatar": "https://example.com/avatars/jane.jpg"
    },
    "modules": [
      {
        "id": "module1",
        "title": "Introduction to JavaScript",
        "duration_minutes": 120,
        "lessons_count": 5
      }
    ],
    "requirements": ["Basic computer skills"],
    "learning_outcomes": ["Understand JavaScript syntax", "Build simple applications"],
    "enrolled_count": 1250,
    "rating": 4.8,
    "reviews_count": 324,
    "is_enrolled": false,
    "created_at": "2024-01-01T00:00:00Z"
  }
}
```

---

### Enrollment Management

#### POST /api/v1/enrollments
**Description**: Enroll in a course

**Authentication**: Required

**Request Body**:
```json
{
  "course_id": "60d5ecb54b4b1c001f5b9b8b",
  "payment_method": "credit_card",
  "payment_token": "<payment_token>"
}
```

**Response**:
```json
{
  "success": true,
  "data": {
    "enrollment_id": "60d5ecb54b4b1c001f5b9b8d",
    "course_id": "60d5ecb54b4b1c001f5b9b8b",
    "user_id": "60d5ecb54b4b1c001f5b9b8a",
    "status": "active",
    "progress": 0,
    "enrolled_at": "2024-01-15T10:30:00Z",
    "expires_at": "2024-07-15T10:30:00Z"
  }
}
```

#### GET /api/v1/enrollments
**Description**: Get user's enrollments

**Authentication**: Required

**Query Parameters**:
- `status` (string) - Filter by status (active, completed, expired)
- `page` (integer, default: 1)
- `limit` (integer, default: 10)

**Response**:
```json
{
  "success": true,
  "data": {
    "enrollments": [
      {
        "id": "60d5ecb54b4b1c001f5b9b8d",
        "course": {
          "id": "60d5ecb54b4b1c001f5b9b8b",
          "title": "JavaScript Fundamentals",
          "thumbnail": "https://example.com/thumbnails/js-course.jpg"
        },
        "status": "active",
        "progress": 45,
        "enrolled_at": "2024-01-15T10:30:00Z",
        "last_accessed": "2024-01-20T14:22:00Z"
      }
    ],
    "pagination": {
      "current_page": 1,
      "total_pages": 1,
      "total_items": 3
    }
  }
}
```

---

### Notifications

#### GET /api/v1/notifications
**Description**: Get user notifications

**Authentication**: Required

**Query Parameters**:
- `read` (boolean) - Filter by read status
- `type` (string) - Filter by notification type
- `page` (integer, default: 1)
- `limit` (integer, default: 20)

**Response**:
```json
{
  "success": true,
  "data": {
    "notifications": [
      {
        "id": "60d5ecb54b4b1c001f5b9b8e",
        "type": "course_update",
        "title": "New lesson available",
        "message": "A new lesson has been added to JavaScript Fundamentals",
        "read": false,
        "data": {
          "course_id": "60d5ecb54b4b1c001f5b9b8b",
          "lesson_id": "lesson_5"
        },
        "created_at": "2024-01-20T09:15:00Z"
      }
    ],
    "unread_count": 5,
    "pagination": {
      "current_page": 1,
      "total_pages": 2,
      "total_items": 25
    }
  }
}
```

#### PUT /api/v1/notifications/{id}/read
**Description**: Mark notification as read

**Authentication**: Required

**Path Parameters**:
- `id` (string) - Notification ID

**Response**:
```json
{
  "success": true,
  "data": {
    "id": "60d5ecb54b4b1c001f5b9b8e",
    "read": true,
    "read_at": "2024-01-20T15:30:00Z"
  }
}
```

---

## 🚨 Error Codes

| Code | Description | HTTP Status |
|------|-------------|-------------|
| `UNAUTHORIZED` | Authentication required | 401 |
| `FORBIDDEN` | Insufficient permissions | 403 |
| `NOT_FOUND` | Resource not found | 404 |
| `VALIDATION_ERROR` | Request validation failed | 400 |
| `DUPLICATE_ENTRY` | Resource already exists | 409 |
| `RATE_LIMITED` | Too many requests | 429 |
| `INTERNAL_ERROR` | Server error | 500 |
| `SERVICE_UNAVAILABLE` | Service temporarily unavailable | 503 |

## 📊 Rate Limiting

- **General endpoints**: 100 requests per minute per IP
- **Authentication endpoints**: 10 requests per minute per IP
- **File upload endpoints**: 5 requests per minute per user

Rate limit headers included in responses:
```
X-RateLimit-Limit: 100
X-RateLimit-Remaining: 95
X-RateLimit-Reset: 1642678800
```

## 🔧 Development Guidelines

### Adding New Endpoints

1. **Define the handler function**:
```go
func (s *Service) HandleNewEndpoint(w http.ResponseWriter, r *http.Request) {
    // Implementation
}
```

2. **Add route in DI setup**:
```go
func Init(i *do.Injector, r *chi.Mux) *do.Injector {
    service := do.MustInvoke[*Service](i)
    
    r.Route("/api/v1/resource", func(r chi.Router) {
        r.With(middleware.Auth()).Post("/", service.HandleNewEndpoint)
    })
    
    return i
}
```

3. **Update API documentation**

### Request Validation

Use struct tags for automatic validation:

```go
type CreateUserRequest struct {
    Email    string `json:"email" validate:"required,email"`
    Name     string `json:"name" validate:"required,min=2,max=100"`
    Password string `json:"password" validate:"required,min=8"`
}
```

### Response Helpers

Use the established response helpers:

```go
// Success response
helpers.SuccessResponse(w, data)

// Error response
helpers.ErrorResponse(w, "Error message", http.StatusBadRequest)

// Validation error response
helpers.ValidationErrorResponse(w, validationErrors)
```

---

For more detailed examples and integration guides, see the project's code examples in the `examples/` directory.
