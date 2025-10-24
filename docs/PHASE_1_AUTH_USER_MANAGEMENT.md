# Phase 1: Authentication & User Management (Week 3-5)

## 🎯 Objectives

- Implement authentication system (login/logout)
- JWT token generation & validation
- Role-based access control (RBAC)
- User management APIs
- Establish base patterns for all future APIs
- Run in parallel with existing NestJS backend

## Key Principles for Phase 1

1. **Zero Downtime**: New APIs run alongside existing backend
2. **Gradual Migration**: Frontend switches endpoints progressively
3. **Data Consistency**: Both backends use same database
4. **Backward Compatible**: Maintain existing API contracts

---

## 1.1 Database Models

### Users Table (Existing Schema)

```sql
-- Existing table from NestJS backend
CREATE TABLE users (
    id VARCHAR(255) PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    full_name VARCHAR(255) NOT NULL,
    role VARCHAR(50) NOT NULL, -- 'SUPERADMIN', 'ADMIN', 'TEACHER', 'STUDENT'
    active BOOLEAN DEFAULT true,
    last_login TIMESTAMP,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_role ON users(role);
```

### Refresh Tokens Table (New/Migration)

```sql
-- For refresh token management
CREATE TABLE refresh_tokens (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token VARCHAR(500) UNIQUE NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    revoked BOOLEAN DEFAULT false,
    revoked_at TIMESTAMP,
    ip_address VARCHAR(45),
    user_agent TEXT
);

CREATE INDEX idx_refresh_tokens_user_id ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_token ON refresh_tokens(token);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
```

### Audit Logs Table (New)

```sql
-- For security & compliance
CREATE TABLE audit_logs (
    id VARCHAR(255) PRIMARY KEY,
    user_id VARCHAR(255) REFERENCES users(id) ON DELETE SET NULL,
    action VARCHAR(100) NOT NULL, -- 'LOGIN', 'LOGOUT', 'CREATE', 'UPDATE', 'DELETE'
    resource VARCHAR(100) NOT NULL, -- 'USER', 'STUDENT', 'GRADE', etc.
    resource_id VARCHAR(255),
    old_values JSONB,
    new_values JSONB,
    ip_address VARCHAR(45),
    user_agent TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_audit_logs_user_id ON audit_logs(user_id);
CREATE INDEX idx_audit_logs_action ON audit_logs(action);
CREATE INDEX idx_audit_logs_resource ON audit_logs(resource);
CREATE INDEX idx_audit_logs_created_at ON audit_logs(created_at DESC);
```

---

## 1.2 Go Models & Structs

### internal/models/user.go

```go
package models

import (
    "time"
)

type UserRole string

const (
    RoleSuperAdmin UserRole = "SUPERADMIN"
    RoleAdmin      UserRole = "ADMIN"
    RoleTeacher    UserRole = "TEACHER"
    RoleStudent    UserRole = "STUDENT"
)

type User struct {
    ID           string    `db:"id" json:"id"`
    Email        string    `db:"email" json:"email"`
    PasswordHash string    `db:"password_hash" json:"-"` // Never expose in JSON
    FullName     string    `db:"full_name" json:"fullName"`
    Role         UserRole  `db:"role" json:"role"`
    Active       bool      `db:"active" json:"active"`
    LastLogin    *time.Time `db:"last_login" json:"lastLogin,omitempty"`
    CreatedAt    time.Time `db:"created_at" json:"createdAt"`
    UpdatedAt    time.Time `db:"updated_at" json:"updatedAt"`
}

type RefreshToken struct {
    ID        string    `db:"id" json:"id"`
    UserID    string    `db:"user_id" json:"userId"`
    Token     string    `db:"token" json:"token"`
    ExpiresAt time.Time `db:"expires_at" json:"expiresAt"`
    CreatedAt time.Time `db:"created_at" json:"createdAt"`
    Revoked   bool      `db:"revoked" json:"revoked"`
    RevokedAt *time.Time `db:"revoked_at" json:"revokedAt,omitempty"`
    IPAddress string    `db:"ip_address" json:"ipAddress"`
    UserAgent string    `db:"user_agent" json:"userAgent"`
}

type AuditLog struct {
    ID         string                 `db:"id" json:"id"`
    UserID     *string                `db:"user_id" json:"userId,omitempty"`
    Action     string                 `db:"action" json:"action"`
    Resource   string                 `db:"resource" json:"resource"`
    ResourceID *string                `db:"resource_id" json:"resourceId,omitempty"`
    OldValues  map[string]interface{} `db:"old_values" json:"oldValues,omitempty"`
    NewValues  map[string]interface{} `db:"new_values" json:"newValues,omitempty"`
    IPAddress  string                 `db:"ip_address" json:"ipAddress"`
    UserAgent  string                 `db:"user_agent" json:"userAgent"`
    CreatedAt  time.Time              `db:"created_at" json:"createdAt"`
}
```

### internal/models/auth.go

```go
package models

import "time"

// Request DTOs
type LoginRequest struct {
    Email    string `json:"email" binding:"required,email"`
    Password string `json:"password" binding:"required,min=6"`
}

type RefreshTokenRequest struct {
    RefreshToken string `json:"refreshToken" binding:"required"`
}

type ChangePasswordRequest struct {
    OldPassword string `json:"oldPassword" binding:"required,min=6"`
    NewPassword string `json:"newPassword" binding:"required,min=6"`
}

type ResetPasswordRequest struct {
    Email string `json:"email" binding:"required,email"`
}

type ConfirmResetPasswordRequest struct {
    Token       string `json:"token" binding:"required"`
    NewPassword string `json:"newPassword" binding:"required,min=6"`
}

// Response DTOs
type LoginResponse struct {
    AccessToken  string    `json:"accessToken"`
    RefreshToken string    `json:"refreshToken"`
    ExpiresIn    int64     `json:"expiresIn"` // seconds
    TokenType    string    `json:"tokenType"` // "Bearer"
    User         *UserInfo `json:"user"`
}

type UserInfo struct {
    ID       string   `json:"id"`
    Email    string   `json:"email"`
    FullName string   `json:"fullName"`
    Role     UserRole `json:"role"`
}

type RefreshTokenResponse struct {
    AccessToken string `json:"accessToken"`
    ExpiresIn   int64  `json:"expiresIn"`
    TokenType   string `json:"tokenType"`
}

// JWT Claims
type JWTClaims struct {
    UserID   string   `json:"userId"`
    Email    string   `json:"email"`
    Role     UserRole `json:"role"`
    IssuedAt int64    `json:"iat"`
    ExpiresAt int64   `json:"exp"`
}
```

---

## 1.3 API Endpoints Specification

### Base URL

```
Development: http://localhost:8080/api/v1
Production:  https://api.yourdomain.com/api/v1
```

### Authentication Endpoints

#### 1. POST /auth/login

**Description**: Authenticate user and get access token

**Request:**

```json
{
  "email": "admin@example.com",
  "password": "password123"
}
```

**Response (200 OK):**

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 86400,
  "tokenType": "Bearer",
  "user": {
    "id": "usr_abc123",
    "email": "admin@example.com",
    "fullName": "Admin User",
    "role": "ADMIN"
  }
}
```

**Error Responses:**

- `400 Bad Request`: Invalid request body
- `401 Unauthorized`: Invalid credentials
- `403 Forbidden`: Account is inactive
- `429 Too Many Requests`: Rate limit exceeded

---

#### 2. POST /auth/refresh

**Description**: Get new access token using refresh token

**Request:**

```json
{
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response (200 OK):**

```json
{
  "accessToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...",
  "expiresIn": 86400,
  "tokenType": "Bearer"
}
```

**Error Responses:**

- `401 Unauthorized`: Invalid or expired refresh token
- `403 Forbidden`: Refresh token revoked

---

#### 3. POST /auth/logout

**Description**: Logout user and revoke refresh token

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "refreshToken": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Response (200 OK):**

```json
{
  "message": "Logout successful"
}
```

---

#### 4. POST /auth/change-password

**Description**: Change user password (authenticated)

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "oldPassword": "oldpass123",
  "newPassword": "newpass456"
}
```

**Response (200 OK):**

```json
{
  "message": "Password changed successfully"
}
```

**Error Responses:**

- `400 Bad Request`: Invalid request
- `401 Unauthorized`: Invalid old password
- `422 Unprocessable Entity`: Password validation failed

---

#### 5. POST /auth/forgot-password

**Description**: Request password reset link (Future: send email)

**Request:**

```json
{
  "email": "user@example.com"
}
```

**Response (200 OK):**

```json
{
  "message": "Password reset link sent to email"
}
```

**Note**: For security, always return success even if email doesn't exist

---

#### 6. POST /auth/reset-password

**Description**: Reset password with token

**Request:**

```json
{
  "token": "reset_token_here",
  "newPassword": "newpass123"
}
```

**Response (200 OK):**

```json
{
  "message": "Password reset successful"
}
```

---

#### 7. GET /auth/me

**Description**: Get current authenticated user info

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "id": "usr_abc123",
  "email": "admin@example.com",
  "fullName": "Admin User",
  "role": "ADMIN",
  "active": true,
  "lastLogin": "2025-10-24T10:30:00Z",
  "createdAt": "2024-01-01T00:00:00Z",
  "updatedAt": "2025-10-24T10:30:00Z"
}
```

---

### User Management Endpoints

#### 8. GET /users

**Description**: List all users (paginated)

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Query Parameters:**

- `page` (integer, default: 1)
- `limit` (integer, default: 10, max: 100)
- `role` (string, optional): Filter by role
- `active` (boolean, optional): Filter by active status
- `search` (string, optional): Search by name or email
- `sort` (string, default: "created_at"): Sort field
- `order` (string, default: "desc"): Sort order (asc/desc)

**Example:**

```
GET /users?page=1&limit=20&role=TEACHER&active=true&search=john
```

**Response (200 OK):**

```json
{
  "data": [
    {
      "id": "usr_abc123",
      "email": "teacher@example.com",
      "fullName": "John Doe",
      "role": "TEACHER",
      "active": true,
      "lastLogin": "2025-10-24T10:30:00Z",
      "createdAt": "2024-01-01T00:00:00Z",
      "updatedAt": "2025-10-24T10:30:00Z"
    }
  ],
  "pagination": {
    "page": 1,
    "limit": 20,
    "total": 150,
    "totalPages": 8
  }
}
```

**Permissions Required**: ADMIN or SUPERADMIN

---

#### 9. GET /users/:id

**Description**: Get user by ID

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "id": "usr_abc123",
  "email": "teacher@example.com",
  "fullName": "John Doe",
  "role": "TEACHER",
  "active": true,
  "lastLogin": "2025-10-24T10:30:00Z",
  "createdAt": "2024-01-01T00:00:00Z",
  "updatedAt": "2025-10-24T10:30:00Z"
}
```

**Error Responses:**

- `404 Not Found`: User not found

**Permissions Required**:

- SUPERADMIN: Can view any user
- ADMIN: Can view any user
- TEACHER/STUDENT: Can only view own profile

---

#### 10. POST /users

**Description**: Create new user

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "email": "newuser@example.com",
  "password": "password123",
  "fullName": "New User",
  "role": "TEACHER",
  "active": true
}
```

**Response (201 Created):**

```json
{
  "id": "usr_xyz789",
  "email": "newuser@example.com",
  "fullName": "New User",
  "role": "TEACHER",
  "active": true,
  "createdAt": "2025-10-24T11:00:00Z",
  "updatedAt": "2025-10-24T11:00:00Z"
}
```

**Error Responses:**

- `400 Bad Request`: Invalid data
- `409 Conflict`: Email already exists
- `422 Unprocessable Entity`: Validation failed

**Permissions Required**: SUPERADMIN or ADMIN

---

#### 11. PUT /users/:id

**Description**: Update user

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Request:**

```json
{
  "fullName": "Updated Name",
  "role": "ADMIN",
  "active": false
}
```

**Response (200 OK):**

```json
{
  "id": "usr_xyz789",
  "email": "user@example.com",
  "fullName": "Updated Name",
  "role": "ADMIN",
  "active": false,
  "updatedAt": "2025-10-24T11:30:00Z"
}
```

**Permissions Required**: SUPERADMIN or ADMIN

---

#### 12. DELETE /users/:id

**Description**: Delete user (soft delete - mark as inactive)

**Headers:**

```
Authorization: Bearer {accessToken}
```

**Response (200 OK):**

```json
{
  "message": "User deleted successfully"
}
```

**Permissions Required**: SUPERADMIN only

---

## 1.4 Implementation Structure

### Repository Layer (internal/repository/user_repository.go)

```go
package repository

import (
    "context"
    "database/sql"

    "github.com/jmoiron/sqlx"
    "admin-panel-sma-backend/internal/models"
)

type UserRepository interface {
    // Authentication
    FindByEmail(ctx context.Context, email string) (*models.User, error)
    FindByID(ctx context.Context, id string) (*models.User, error)
    UpdateLastLogin(ctx context.Context, userID string) error
    UpdatePassword(ctx context.Context, userID, passwordHash string) error

    // User Management
    List(ctx context.Context, filters UserFilters) ([]*models.User, int, error)
    Create(ctx context.Context, user *models.User) error
    Update(ctx context.Context, user *models.User) error
    Delete(ctx context.Context, id string) error

    // Refresh Token
    CreateRefreshToken(ctx context.Context, token *models.RefreshToken) error
    FindRefreshToken(ctx context.Context, token string) (*models.RefreshToken, error)
    RevokeRefreshToken(ctx context.Context, token string) error
    RevokeAllUserTokens(ctx context.Context, userID string) error
}

type UserFilters struct {
    Page   int
    Limit  int
    Role   string
    Active *bool
    Search string
    Sort   string
    Order  string
}

type userRepository struct {
    db *sqlx.DB
}

func NewUserRepository(db *sqlx.DB) UserRepository {
    return &userRepository{db: db}
}

func (r *userRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
    var user models.User
    query := `SELECT id, email, password_hash, full_name, role, active,
                     last_login, created_at, updated_at
              FROM users WHERE email = $1`

    err := r.db.GetContext(ctx, &user, query, email)
    if err == sql.ErrNoRows {
        return nil, nil
    }
    return &user, err
}

// ... implement other methods
```

### Service Layer (internal/service/auth_service.go)

```go
package service

import (
    "context"
    "errors"
    "time"

    "golang.org/x/crypto/bcrypt"
    "github.com/golang-jwt/jwt/v5"
    "github.com/google/uuid"

    "admin-panel-sma-backend/internal/models"
    "admin-panel-sma-backend/internal/repository"
    "admin-panel-sma-backend/pkg/config"
)

type AuthService interface {
    Login(ctx context.Context, req *models.LoginRequest, ipAddress, userAgent string) (*models.LoginResponse, error)
    RefreshToken(ctx context.Context, req *models.RefreshTokenRequest) (*models.RefreshTokenResponse, error)
    Logout(ctx context.Context, refreshToken string) error
    ChangePassword(ctx context.Context, userID string, req *models.ChangePasswordRequest) error
    ValidateToken(tokenString string) (*models.JWTClaims, error)
}

type authService struct {
    userRepo repository.UserRepository
    config   *config.Config
}

func NewAuthService(userRepo repository.UserRepository, cfg *config.Config) AuthService {
    return &authService{
        userRepo: userRepo,
        config:   cfg,
    }
}

func (s *authService) Login(ctx context.Context, req *models.LoginRequest, ipAddress, userAgent string) (*models.LoginResponse, error) {
    // 1. Find user by email
    user, err := s.userRepo.FindByEmail(ctx, req.Email)
    if err != nil {
        return nil, err
    }
    if user == nil {
        return nil, errors.New("invalid credentials")
    }

    // 2. Check if user is active
    if !user.Active {
        return nil, errors.New("account is inactive")
    }

    // 3. Verify password
    err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password))
    if err != nil {
        return nil, errors.New("invalid credentials")
    }

    // 4. Generate JWT tokens
    accessToken, err := s.generateAccessToken(user)
    if err != nil {
        return nil, err
    }

    refreshToken, err := s.generateRefreshToken(user, ipAddress, userAgent)
    if err != nil {
        return nil, err
    }

    // 5. Update last login
    _ = s.userRepo.UpdateLastLogin(ctx, user.ID)

    // 6. Return response
    return &models.LoginResponse{
        AccessToken:  accessToken,
        RefreshToken: refreshToken,
        ExpiresIn:    int64(s.config.JWT.Expiration.Seconds()),
        TokenType:    "Bearer",
        User: &models.UserInfo{
            ID:       user.ID,
            Email:    user.Email,
            FullName: user.FullName,
            Role:     user.Role,
        },
    }, nil
}

func (s *authService) generateAccessToken(user *models.User) (string, error) {
    now := time.Now()
    expiresAt := now.Add(s.config.JWT.Expiration)

    claims := &models.JWTClaims{
        UserID:    user.ID,
        Email:     user.Email,
        Role:      user.Role,
        IssuedAt:  now.Unix(),
        ExpiresAt: expiresAt.Unix(),
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
        "userId": claims.UserID,
        "email":  claims.Email,
        "role":   claims.Role,
        "iat":    claims.IssuedAt,
        "exp":    claims.ExpiresAt,
    })

    return token.SignedString([]byte(s.config.JWT.Secret))
}

// ... implement other methods
```

### Handler/Controller Layer (internal/handler/auth_handler.go)

```go
package handler

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "admin-panel-sma-backend/internal/models"
    "admin-panel-sma-backend/internal/service"
    "admin-panel-sma-backend/pkg/errors"
)

type AuthHandler struct {
    authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
    return &AuthHandler{authService: authService}
}

// @Summary User Login
// @Description Authenticate user and get access token
// @Tags Authentication
// @Accept json
// @Produce json
// @Param request body models.LoginRequest true "Login credentials"
// @Success 200 {object} models.LoginResponse
// @Failure 400 {object} errors.ErrorResponse
// @Failure 401 {object} errors.ErrorResponse
// @Router /auth/login [post]
func (h *AuthHandler) Login(c *gin.Context) {
    var req models.LoginRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, errors.NewErrorResponse("invalid request", err))
        return
    }

    ipAddress := c.ClientIP()
    userAgent := c.GetHeader("User-Agent")

    resp, err := h.authService.Login(c.Request.Context(), &req, ipAddress, userAgent)
    if err != nil {
        c.JSON(http.StatusUnauthorized, errors.NewErrorResponse("authentication failed", err))
        return
    }

    c.JSON(http.StatusOK, resp)
}

// ... implement other handlers
```

### Middleware (internal/middleware/auth.go)

```go
package middleware

import (
    "net/http"
    "strings"

    "github.com/gin-gonic/gin"
    "admin-panel-sma-backend/internal/models"
    "admin-panel-sma-backend/internal/service"
)

func AuthMiddleware(authService service.AuthService) gin.HandlerFunc {
    return func(c *gin.Context) {
        // 1. Get token from header
        authHeader := c.GetHeader("Authorization")
        if authHeader == "" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "authorization header required"})
            c.Abort()
            return
        }

        // 2. Parse Bearer token
        parts := strings.Split(authHeader, " ")
        if len(parts) != 2 || parts[0] != "Bearer" {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid authorization format"})
            c.Abort()
            return
        }

        // 3. Validate token
        claims, err := authService.ValidateToken(parts[1])
        if err != nil {
            c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
            c.Abort()
            return
        }

        // 4. Set user info in context
        c.Set("userId", claims.UserID)
        c.Set("userEmail", claims.Email)
        c.Set("userRole", claims.Role)

        c.Next()
    }
}

func RequireRole(roles ...models.UserRole) gin.HandlerFunc {
    return func(c *gin.Context) {
        userRole, exists := c.Get("userRole")
        if !exists {
            c.JSON(http.StatusForbidden, gin.H{"error": "role not found in context"})
            c.Abort()
            return
        }

        role := userRole.(models.UserRole)
        allowed := false
        for _, r := range roles {
            if r == role {
                allowed = true
                break
            }
        }

        if !allowed {
            c.JSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
            c.Abort()
            return
        }

        c.Next()
    }
}
```

---

## 1.5 Router Setup

### cmd/api-gateway/routes.go

```go
package main

import (
    "github.com/gin-gonic/gin"
    "admin-panel-sma-backend/internal/handler"
    "admin-panel-sma-backend/internal/middleware"
)

func setupRoutes(r *gin.Engine, handlers *Handlers, middlewares *Middlewares) {
    // Health check
    r.GET("/health", func(c *gin.Context) {
        c.JSON(200, gin.H{"status": "ok"})
    })

    api := r.Group("/api/v1")

    // Public routes
    auth := api.Group("/auth")
    {
        auth.POST("/login", handlers.Auth.Login)
        auth.POST("/refresh", handlers.Auth.RefreshToken)
        auth.POST("/forgot-password", handlers.Auth.ForgotPassword)
        auth.POST("/reset-password", handlers.Auth.ResetPassword)
    }

    // Protected routes
    protected := api.Group("")
    protected.Use(middlewares.Auth)
    {
        // Auth endpoints (require authentication)
        protected.POST("/auth/logout", handlers.Auth.Logout)
        protected.POST("/auth/change-password", handlers.Auth.ChangePassword)
        protected.GET("/auth/me", handlers.Auth.GetCurrentUser)

        // User management (admin only)
        users := protected.Group("/users")
        users.Use(middleware.RequireRole(models.RoleSuperAdmin, models.RoleAdmin))
        {
            users.GET("", handlers.User.List)
            users.POST("", handlers.User.Create)
            users.GET("/:id", handlers.User.GetByID)
            users.PUT("/:id", handlers.User.Update)
            users.DELETE("/:id", middleware.RequireRole(models.RoleSuperAdmin), handlers.User.Delete)
        }
    }
}
```

---

## 1.6 Testing Strategy

### Unit Tests Example

```go
// internal/service/auth_service_test.go
package service

import (
    "context"
    "testing"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/mock"
)

type MockUserRepository struct {
    mock.Mock
}

func (m *MockUserRepository) FindByEmail(ctx context.Context, email string) (*models.User, error) {
    args := m.Called(ctx, email)
    if args.Get(0) == nil {
        return nil, args.Error(1)
    }
    return args.Get(0).(*models.User), args.Error(1)
}

func TestAuthService_Login_Success(t *testing.T) {
    // Setup
    mockRepo := new(MockUserRepository)
    service := NewAuthService(mockRepo, testConfig)

    // Mock data
    expectedUser := &models.User{
        ID:           "usr_123",
        Email:        "test@example.com",
        PasswordHash: hashedPassword,
        Active:       true,
    }

    mockRepo.On("FindByEmail", mock.Anything, "test@example.com").Return(expectedUser, nil)
    mockRepo.On("UpdateLastLogin", mock.Anything, "usr_123").Return(nil)

    // Execute
    req := &models.LoginRequest{
        Email:    "test@example.com",
        Password: "password123",
    }
    resp, err := service.Login(context.Background(), req, "127.0.0.1", "TestAgent")

    // Assert
    assert.NoError(t, err)
    assert.NotNil(t, resp)
    assert.NotEmpty(t, resp.AccessToken)
    assert.Equal(t, "usr_123", resp.User.ID)

    mockRepo.AssertExpectations(t)
}
```

### Integration Tests Example

```go
// tests/integration/auth_test.go
package integration

import (
    "bytes"
    "encoding/json"
    "net/http"
    "testing"

    "github.com/stretchr/testify/assert"
)

func TestLoginEndpoint(t *testing.T) {
    // Setup test server
    router := setupTestRouter()

    // Prepare request
    loginReq := map[string]string{
        "email":    "admin@example.com",
        "password": "password123",
    }
    body, _ := json.Marshal(loginReq)

    // Make request
    w := performRequest(router, "POST", "/api/v1/auth/login", bytes.NewBuffer(body))

    // Assert
    assert.Equal(t, http.StatusOK, w.Code)

    var resp models.LoginResponse
    json.Unmarshal(w.Body.Bytes(), &resp)
    assert.NotEmpty(t, resp.AccessToken)
    assert.Equal(t, "Bearer", resp.TokenType)
}
```

---

## 1.7 Week 3-5 Task Breakdown

### Week 3: Core Authentication

- [ ] Create database migrations for refresh_tokens and audit_logs
- [ ] Implement User model and repository
- [ ] Implement AuthService (login, logout, token generation)
- [ ] Create JWT middleware
- [ ] Implement auth handlers (login, refresh, logout)
- [ ] Write unit tests for AuthService
- [ ] Create Swagger documentation
- [ ] Manual testing with Postman/curl

### Week 4: User Management & RBAC

- [ ] Implement UserService (CRUD operations)
- [ ] Create user management handlers
- [ ] Implement role-based middleware
- [ ] Add pagination helper
- [ ] Implement password change functionality
- [ ] Write unit tests for UserService
- [ ] Integration tests for user endpoints
- [ ] Update Swagger docs

### Week 5: Polish & Integration

- [ ] Implement audit logging
- [ ] Add rate limiting
- [ ] Frontend integration (update API base URL)
- [ ] End-to-end testing
- [ ] Performance testing & optimization
- [ ] Security audit
- [ ] Documentation updates
- [ ] Deployment to staging environment

---

## 1.8 Migration Strategy

### Parallel Running (Week 3-5)

```
┌─────────────┐
│  Frontend   │
└──────┬──────┘
       │
       ├──────────┐
       │          │
       ▼          ▼
┌──────────┐  ┌──────────┐
│   Go     │  │  NestJS  │
│ Backend  │  │ Backend  │
│ (New)    │  │ (Legacy) │
└────┬─────┘  └────┬─────┘
     │             │
     └──────┬──────┘
            ▼
      ┌──────────┐
      │   DB     │
      └──────────┘
```

### Frontend Migration Steps

1. **Feature Flag**: Add environment variable `USE_GO_AUTH=true`
2. **Gradual Rollout**:
   - Week 3: Internal testing only
   - Week 4: 20% of users (staff)
   - Week 5: 100% migration
3. **Rollback Plan**: Keep NestJS endpoints active for 2 weeks after full migration

---

## 1.9 Success Criteria

- [ ] All auth endpoints return < 100ms response time
- [ ] 100% test coverage for critical paths (login, token validation)
- [ ] Zero downtime during migration
- [ ] Security audit passed (no vulnerabilities)
- [ ] API documentation complete
- [ ] Frontend successfully integrated
- [ ] Monitoring & logging in place

---

**Next Phase**: [Phase 2: Academic Management](./PHASE_2_ACADEMIC_MANAGEMENT.md)
