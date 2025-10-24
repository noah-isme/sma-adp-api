package models

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// LoginRequest holds credentials for authenticating a user.
type LoginRequest struct {
	Email     string `json:"email" validate:"required,email"`
	Password  string `json:"password" validate:"required"`
	IP        string `json:"-"`
	UserAgent string `json:"-"`
}

// LoginResponse returns the issued tokens and user info.
type LoginResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int64     `json:"expires_in"`
	User         UserInfo  `json:"user"`
	IssuedAt     time.Time `json:"issued_at"`
}

// RefreshTokenRequest exchanges a refresh token for a new access token.
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
	IP           string `json:"-"`
	UserAgent    string `json:"-"`
}

// RefreshTokenResponse returns the refreshed tokens.
type RefreshTokenResponse struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresIn    int64     `json:"expires_in"`
	IssuedAt     time.Time `json:"issued_at"`
}

// ChangePasswordRequest payload for updating password.
type ChangePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}

// ResetPasswordRequest payload for initiating reset flow.
type ResetPasswordRequest struct {
	Email string `json:"email" validate:"required,email"`
}

// ConfirmResetPasswordRequest completes reset flow.
type ConfirmResetPasswordRequest struct {
	Token       string `json:"token" validate:"required"`
	NewPassword string `json:"new_password" validate:"required,min=6"`
}

// UserInfo describes the authenticated user in responses.
type UserInfo struct {
	ID       string   `json:"id"`
	Email    string   `json:"email"`
	FullName string   `json:"full_name"`
	Role     UserRole `json:"role"`
}

// JWTClaims represents the JWT payload for access tokens.
type JWTClaims struct {
	UserID   string   `json:"user_id"`
	Role     UserRole `json:"role"`
	Email    string   `json:"email"`
	FullName string   `json:"full_name"`
	jwt.RegisteredClaims
}
