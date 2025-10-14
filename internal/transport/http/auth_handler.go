package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/service"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/util"
)

const (
	contextUserKey  = "auth.user"
	contextTokenKey = "auth.token"
)

type AuthHandler struct {
	auth *service.AuthService
}

func RegisterAuth(e *echo.Echo, auth *service.AuthService) {
	handler := &AuthHandler{auth: auth}
	group := e.Group("/api/v1/auth")

	group.POST("/register", handler.registerEmail)
	group.POST("/login", handler.loginEmail)
	group.POST("/google", handler.loginGoogle)
	group.POST("/logout", handler.logout, handler.requireAuth())
	group.POST("/password", handler.changePassword, handler.requireAuth())
	group.POST("/password/reset-request", handler.resetPasswordRequest)
	group.POST("/password/reset-confirm", handler.resetPasswordConfirm)
	group.GET("/me", handler.me, handler.requireAuth())
	group.POST("/profile", handler.completeProfile, handler.requireAuth())
	group.GET("/users", handler.listUsers, handler.requireAuth())
	group.DELETE("/users/:id", handler.deleteUser, handler.requireAuth())
}

// registerEmail handles user registration with email and password.
// @Summary      Register with email
// @Description  Create a new Fit City account using an email address and password.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        payload  body      RegisterRequest  true  "Registration payload"
// @Success      201      {object}  AuthTokenResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      409      {object}  ErrorResponse
// @Router       /auth/register [post]
func (h *AuthHandler) registerEmail(c echo.Context) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid request body"))
	}

	result, err := h.auth.RegisterWithEmail(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		switch err {
		case service.ErrEmailAlreadyUsed:
			return c.JSON(http.StatusConflict, util.Error(err.Error()))
		default:
			return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
		}
	}

	return c.JSON(http.StatusCreated, util.Envelope{
		"token":      result.Token,
		"expires_at": result.ExpiresAt.UTC().Format(time.RFC3339),
		"user":       sanitizeUser(result.User),
	})
}

// loginEmail issues a JWT for email and password credentials.
// @Summary      Login with email
// @Description  Authenticate using email/password credentials.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        payload  body      LoginRequest  true  "Login payload"
// @Success      200      {object}  AuthTokenResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      401      {object}  ErrorResponse
// @Router       /auth/login [post]
func (h *AuthHandler) loginEmail(c echo.Context) error {
	var req struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid request body"))
	}

	result, err := h.auth.LoginWithEmail(c.Request().Context(), req.Email, req.Password)
	if err != nil {
		if err == service.ErrInvalidCredentials {
			return c.JSON(http.StatusUnauthorized, util.Error(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
	}

	return c.JSON(http.StatusOK, util.Envelope{
		"token":      result.Token,
		"expires_at": result.ExpiresAt.UTC().Format(time.RFC3339),
		"user":       sanitizeUser(result.User),
	})
}

// loginGoogle exchanges a Google ID token for a Fit City session token.
// @Summary      Login with Google
// @Description  Authenticate using a Google Sign-In ID token.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        payload  body      GoogleLoginRequest  true  "Google login payload"
// @Success      200      {object}  AuthTokenResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      401      {object}  ErrorResponse
// @Router       /auth/google [post]
func (h *AuthHandler) loginGoogle(c echo.Context) error {
	var req struct {
		IDToken string `json:"id_token"`
	}
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.IDToken) == "" {
		return c.JSON(http.StatusBadRequest, util.Error("id_token required"))
	}

	result, err := h.auth.LoginWithGoogle(c.Request().Context(), req.IDToken)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, util.Error(err.Error()))
	}

	return c.JSON(http.StatusOK, util.Envelope{
		"token":      result.Token,
		"expires_at": result.ExpiresAt.UTC().Format(time.RFC3339),
		"user":       sanitizeUser(result.User),
	})
}

// changePassword updates the current user's password.
// @Summary      Change password
// @Description  Update the authenticated user's password.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Security     BearerAuth
// @Param        payload  body      ChangePasswordRequest  true  "Password change payload"
// @Success      200      {object}  SuccessResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      401      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /auth/password [post]
func (h *AuthHandler) changePassword(c echo.Context) error {
	user, ok := c.Get(contextUserKey).(*domain.User)
	if !ok || user == nil {
		return c.JSON(http.StatusInternalServerError, util.Error("user context missing"))
	}

	var req struct {
		CurrentPassword string `json:"current_password"`
		NewPassword     string `json:"new_password"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid request body"))
	}

	if strings.TrimSpace(req.NewPassword) == "" {
		return c.JSON(http.StatusBadRequest, util.Error("new_password required"))
	}

	if err := h.auth.ChangePassword(c.Request().Context(), user.ID, req.CurrentPassword, req.NewPassword); err != nil {
		switch err {
		case service.ErrPasswordTooWeak:
			return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
		case service.ErrPasswordMismatch, service.ErrInvalidCredentials:
			return c.JSON(http.StatusUnauthorized, util.Error(err.Error()))
		default:
			return c.JSON(http.StatusInternalServerError, util.Error("unable to change password"))
		}
	}

	return c.JSON(http.StatusOK, util.Envelope{"success": true})
}

// resetPasswordRequest issues a password reset code to the provided email.
// @Summary      Request password reset
// @Description  Send a reset OTP to the provided email address.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        payload  body      PasswordResetRequest  true  "Password reset request payload"
// @Success      200      {object}  SuccessResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /auth/password/reset-request [post]
func (h *AuthHandler) resetPasswordRequest(c echo.Context) error {
	var req struct {
		Email string `json:"email"`
	}
	if err := c.Bind(&req); err != nil || strings.TrimSpace(req.Email) == "" {
		return c.JSON(http.StatusBadRequest, util.Error("email required"))
	}

	if err := h.auth.RequestPasswordReset(c.Request().Context(), req.Email); err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error("unable to initiate password reset"))
	}

	return c.JSON(http.StatusOK, util.Envelope{"success": true})
}

// resetPasswordConfirm finalizes a password reset using an OTP.
// @Summary      Confirm password reset
// @Description  Set a new password using a reset OTP sent to email.
// @Tags         Auth
// @Accept       json
// @Produce      json
// @Param        payload  body      PasswordResetConfirmRequest  true  "Password reset confirmation payload"
// @Success      200      {object}  SuccessResponse
// @Failure      400      {object}  ErrorResponse
// @Failure      500      {object}  ErrorResponse
// @Router       /auth/password/reset-confirm [post]
func (h *AuthHandler) resetPasswordConfirm(c echo.Context) error {
	var req struct {
		Email       string `json:"email"`
		OTP         string `json:"otp"`
		NewPassword string `json:"new_password"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid request body"))
	}

	if strings.TrimSpace(req.Email) == "" || strings.TrimSpace(req.OTP) == "" || strings.TrimSpace(req.NewPassword) == "" {
		return c.JSON(http.StatusBadRequest, util.Error("email, otp, and new_password are required"))
	}

	if err := h.auth.ConfirmPasswordReset(c.Request().Context(), req.Email, req.OTP, req.NewPassword); err != nil {
		switch err {
		case service.ErrPasswordTooWeak:
			return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
		case service.ErrResetOTPExpired, service.ErrResetOTPInvalid:
			return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
		default:
			return c.JSON(http.StatusInternalServerError, util.Error("unable to reset password"))
		}
	}

	return c.JSON(http.StatusOK, util.Envelope{"success": true})
}

// completeProfile updates profile details for the current user.
// @Summary      Complete profile
// @Description  Update profile metadata and optionally upload an avatar.
// @Tags         Auth
// @Accept       multipart/form-data
// @Produce      json
// @Security     BearerAuth
// @Param        full_name  formData  string  false  "Full name"
// @Param        username   formData  string  false  "Username"
// @Param        avatar     formData  file    false  "Avatar image"
// @Success      200        {object}  AuthUserResponse
// @Failure      400        {object}  ErrorResponse
// @Failure      401        {object}  ErrorResponse
// @Failure      409        {object}  ErrorResponse
// @Router       /auth/profile [post]
func (h *AuthHandler) completeProfile(c echo.Context) error {
	user := c.Get(contextUserKey).(*domain.User)

	fullName := c.FormValue("full_name")
	username := c.FormValue("username")

	var profileImage *service.ProfileImage
	file, err := c.FormFile("avatar")
	if err == nil && file != nil {
		src, openErr := file.Open()
		if openErr != nil {
			return c.JSON(http.StatusBadRequest, util.Error("invalid file upload"))
		}
		defer src.Close()
		contentType := file.Header.Get("Content-Type")
		if contentType == "" {
			contentType = "application/octet-stream"
		}
		profileImage = &service.ProfileImage{
			Reader:      src,
			Size:        file.Size,
			FileName:    file.Filename,
			ContentType: contentType,
		}
	}

	updated, err := h.auth.CompleteProfile(
		c.Request().Context(),
		user.ID,
		toPtrOrNil(fullName),
		toPtrOrNil(username),
		profileImage,
	)
	if err != nil {
		switch err {
		case service.ErrUsernameAlreadyUsed:
			return c.JSON(http.StatusConflict, util.Error(err.Error()))
		default:
			return c.JSON(http.StatusBadRequest, util.Error(err.Error()))
		}
	}

	return c.JSON(http.StatusOK, util.Envelope{"user": sanitizeUser(updated)})
}

// me returns the authenticated user's profile.
// @Summary      Retrieve profile
// @Description  Fetch details for the authenticated user.
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  AuthUserResponse
// @Failure      401  {object}  ErrorResponse
// @Router       /auth/me [get]
func (h *AuthHandler) me(c echo.Context) error {
	user := c.Get(contextUserKey).(*domain.User)
	return c.JSON(http.StatusOK, util.Envelope{"user": sanitizeUser(user)})
}

// logout invalidates the current session token.
// @Summary      Logout
// @Description  Invalidate the current JWT session.
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Success      200  {object}  SuccessResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /auth/logout [post]
func (h *AuthHandler) logout(c echo.Context) error {
	token := c.Get(contextTokenKey).(string)
	if err := h.auth.Logout(c.Request().Context(), token); err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error(err.Error()))
	}
	return c.JSON(http.StatusOK, util.Envelope{"success": true})
}

// listUsers returns a paginated list of users.
// @Summary      List users
// @Description  Retrieve a paginated list of users. Limit is capped at 200.
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Param        limit   query     int     false  "Number of users to return (max 200)"  maximum(200)
// @Param        offset  query     int     false  "Offset for pagination"
// @Success      200     {object}  UsersListResponse
// @Failure      400     {object}  ErrorResponse
// @Failure      401     {object}  ErrorResponse
// @Failure      500     {object}  ErrorResponse
// @Router       /auth/users [get]
func (h *AuthHandler) listUsers(c echo.Context) error {
	limit := 50
	if raw := c.QueryParam("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return c.JSON(http.StatusBadRequest, util.Error("limit must be a positive integer"))
		}
		limit = parsed
	}
	if limit > 200 {
		limit = 200
	}

	offset := 0
	if raw := c.QueryParam("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return c.JSON(http.StatusBadRequest, util.Error("offset must be zero or a positive integer"))
		}
		offset = parsed
	}

	users, err := h.auth.ListUsers(c.Request().Context(), limit, offset)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error("unable to list users"))
	}

	items := make([]util.Envelope, len(users))
	for i := range users {
		items[i] = sanitizeUser(&users[i])
	}

	return c.JSON(http.StatusOK, util.Envelope{
		"users": items,
		"meta": util.Envelope{
			"limit":  limit,
			"offset": offset,
			"count":  len(items),
		},
	})
}

// deleteUser removes a user account.
// @Summary      Delete user
// @Description  Delete a user by ID. Requires an authorized actor with sufficient permissions.
// @Tags         Auth
// @Produce      json
// @Security     BearerAuth
// @Param        id   path      string  true  "User ID (UUID)"
// @Success      200  {object}  SuccessResponse
// @Failure      400  {object}  ErrorResponse
// @Failure      401  {object}  ErrorResponse
// @Failure      403  {object}  ErrorResponse
// @Failure      404  {object}  ErrorResponse
// @Failure      500  {object}  ErrorResponse
// @Router       /auth/users/{id} [delete]
func (h *AuthHandler) deleteUser(c echo.Context) error {
	actor, ok := c.Get(contextUserKey).(*domain.User)
	if !ok || actor == nil {
		return c.JSON(http.StatusInternalServerError, util.Error("user context missing"))
	}

	userID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		return c.JSON(http.StatusBadRequest, util.Error("invalid user id"))
	}

	if err := h.auth.DeleteUser(c.Request().Context(), actor, userID); err != nil {
		switch err {
		case service.ErrUserNotFound:
			return c.JSON(http.StatusNotFound, util.Error(err.Error()))
		case service.ErrForbidden:
			return c.JSON(http.StatusForbidden, util.Error(err.Error()))
		default:
			return c.JSON(http.StatusInternalServerError, util.Error("unable to delete user"))
		}
	}

	return c.JSON(http.StatusOK, util.Envelope{"success": true})
}

func (h *AuthHandler) requireAuth() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			authHeader := c.Request().Header.Get("Authorization")
			if authHeader == "" {
				return c.JSON(http.StatusUnauthorized, util.Error("missing authorization header"))
			}
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
				return c.JSON(http.StatusUnauthorized, util.Error("invalid authorization header"))
			}
			token := strings.TrimSpace(parts[1])
			user, err := h.auth.Authenticate(c.Request().Context(), token)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, util.Error(err.Error()))
			}
			c.Set(contextUserKey, user)
			c.Set(contextTokenKey, token)
			return next(c)
		}
	}
}

func sanitizeUser(user *domain.User) util.Envelope {
	if user == nil {
		return util.Envelope{}
	}
	payload := util.Envelope{
		"id":                user.ID,
		"email":             user.Email,
		"profile_completed": user.ProfileCompleted,
		"created_at":        user.CreatedAt,
		"updated_at":        user.UpdatedAt,
	}
	if len(user.Roles) > 0 {
		roles := make([]util.Envelope, len(user.Roles))
		for i, role := range user.Roles {
			entry := util.Envelope{
				"id":         role.ID,
				"role_name":  role.Name,
				"created_at": role.CreatedAt,
				"updated_at": role.UpdatedAt,
			}
			if role.Description != nil {
				entry["description"] = *role.Description
			}
			roles[i] = entry
		}
		payload["roles"] = roles
		payload["role_id"] = user.Roles[0].ID
		payload["role_name"] = user.Roles[0].Name
	}
	if user.Username != nil {
		payload["username"] = *user.Username
	}
	if user.FullName != nil {
		payload["full_name"] = *user.FullName
	}
	if user.ImageURL != nil {
		payload["user_image_url"] = *user.ImageURL
	}
	return payload
}

func toPtrOrNil(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
