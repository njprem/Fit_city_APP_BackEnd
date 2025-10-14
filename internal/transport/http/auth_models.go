package http

import "time"

// ErrorResponse represents a generic error payload.
type ErrorResponse struct {
	Error string `json:"error" example:"invalid credentials"`
}

// AuthRole models the role information included in auth responses.
type AuthRole struct {
	ID          string    `json:"id" example:"f4bb0e02-5f91-4ce0-a6c0-7f63f3a8d5e2"`
	RoleName    string    `json:"role_name" example:"admin"`
	Description *string   `json:"description,omitempty"`
	CreatedAt   time.Time `json:"created_at" example:"2024-01-01T12:00:00Z"`
	UpdatedAt   time.Time `json:"updated_at" example:"2024-01-01T12:00:00Z"`
}

// AuthUser models the sanitized user representation returned by auth endpoints.
type AuthUser struct {
	ID               string     `json:"id" example:"9fd13fd2-63c5-4f29-a210-4a1a8e285f74"`
	Email            string     `json:"email" example:"user@example.com"`
	Username         *string    `json:"username,omitempty" example:"fitcityuser"`
	FullName         *string    `json:"full_name,omitempty" example:"Fit City"`
	UserImageURL     *string    `json:"user_image_url,omitempty" example:"https://cdn.example.com/avatar.png"`
	RoleID           *string    `json:"role_id,omitempty" example:"6a4f2f1e-1c7b-4a5e-a938-f1ed9b1fad10"`
	RoleName         *string    `json:"role_name,omitempty" example:"member"`
	Roles            []AuthRole `json:"roles,omitempty"`
	ProfileCompleted bool       `json:"profile_completed" example:"true"`
	CreatedAt        time.Time  `json:"created_at" example:"2024-01-01T12:00:00Z"`
	UpdatedAt        time.Time  `json:"updated_at" example:"2024-01-02T09:30:00Z"`
}

// AuthTokenResponse is returned by endpoints that issue JWT tokens.
type AuthTokenResponse struct {
	Token     string   `json:"token" example:"eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."`
	ExpiresAt string   `json:"expires_at" example:"2024-01-02T09:30:00Z"`
	User      AuthUser `json:"user"`
}

// AuthUserResponse wraps a user object.
type AuthUserResponse struct {
	User AuthUser `json:"user"`
}

// SuccessResponse denotes a simple success flag.
type SuccessResponse struct {
	Success bool `json:"success" example:"true"`
}

// UsersMeta describes pagination metadata for user listings.
type UsersMeta struct {
	Limit  int `json:"limit" example:"20"`
	Offset int `json:"offset" example:"0"`
	Count  int `json:"count" example:"2"`
}

// UsersListResponse is returned by the users listing endpoint.
type UsersListResponse struct {
	Users []AuthUser `json:"users"`
	Meta  UsersMeta  `json:"meta"`
}

// RegisterRequest carries email registration fields.
type RegisterRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"StrongPass!23"`
}

// LoginRequest carries email login fields.
type LoginRequest struct {
	Email    string `json:"email" example:"user@example.com"`
	Password string `json:"password" example:"StrongPass!23"`
}

// GoogleLoginRequest carries the Google ID token for login.
type GoogleLoginRequest struct {
	IDToken string `json:"id_token" example:"eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9..."`
}

// ChangePasswordRequest captures the payload for password updates.
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" example:"OldPass!23"`
	NewPassword     string `json:"new_password" example:"NewPass!45"`
}

// PasswordResetRequest captures the payload for requesting a reset code.
type PasswordResetRequest struct {
	Email string `json:"email" example:"user@example.com"`
}

// PasswordResetConfirmRequest captures the payload for confirming a reset.
type PasswordResetConfirmRequest struct {
	Email       string `json:"email" example:"user@example.com"`
	OTP         string `json:"otp" example:"123456"`
	NewPassword string `json:"new_password" example:"NewPass!45"`
}
