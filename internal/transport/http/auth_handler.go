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

func (h *AuthHandler) me(c echo.Context) error {
	user := c.Get(contextUserKey).(*domain.User)
	return c.JSON(http.StatusOK, util.Envelope{"user": sanitizeUser(user)})
}

func (h *AuthHandler) logout(c echo.Context) error {
	token := c.Get(contextTokenKey).(string)
	if err := h.auth.Logout(c.Request().Context(), token); err != nil {
		return c.JSON(http.StatusInternalServerError, util.Error(err.Error()))
	}
	return c.JSON(http.StatusOK, util.Envelope{"success": true})
}

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
		"role_id":           user.RoleID,
		"profile_completed": user.ProfileCompleted,
		"created_at":        user.CreatedAt,
		"updated_at":        user.UpdatedAt,
	}
	if user.RoleName != nil {
		payload["role_name"] = *user.RoleName
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
