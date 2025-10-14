package service

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"google.golang.org/api/idtoken"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/util"
)

var (
	ErrEmailAlreadyUsed    = errors.New("email already registered")
	ErrInvalidCredentials  = errors.New("invalid credentials")
	ErrUsernameAlreadyUsed = errors.New("username already in use")
	ErrTokenInvalid        = errors.New("token invalid")
	ErrPasswordTooWeak     = errors.New("new password does not meet requirements")
	ErrPasswordMismatch    = errors.New("current password is incorrect")
	ErrResetOTPInvalid     = errors.New("password reset code invalid")
	ErrResetOTPExpired     = errors.New("password reset code expired")
	ErrForbidden           = errors.New("forbidden")
	ErrUserNotFound        = errors.New("user not found")
)

const maxGoogleProfileImageBytes int64 = 5 * 1024 * 1024
const defaultOTPLength = 6

type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

type PasswordResetSender interface {
	SendPasswordReset(ctx context.Context, email, otp string) error
}

type ProfileImage struct {
	Reader      io.Reader
	Size        int64
	FileName    string
	ContentType string
}

type AuthResult struct {
	Token     string
	ExpiresAt time.Time
	User      *domain.User
}

type AuthService struct {
	users           ports.UserRepository
	roles           ports.RoleRepository
	sessions        ports.SessionRepository
	passwordResets  ports.PasswordResetRepository
	storage         ports.ObjectStorage
	jwt             *util.JWTManager
	googleAudience  string
	profileBucket   string
	defaultRoleName string
	adminRoleName   string
	httpClient      httpDoer
	mailer          PasswordResetSender
	resetTTL        time.Duration
	otpLength       int
}

func NewAuthService(users ports.UserRepository, roles ports.RoleRepository, sessions ports.SessionRepository, resets ports.PasswordResetRepository, storage ports.ObjectStorage, mailer PasswordResetSender, jwtManager *util.JWTManager, googleAudience, profileBucket string, resetTTL time.Duration, otpLength int) *AuthService {
	if resetTTL <= 0 {
		resetTTL = 15 * time.Minute
	}
	if otpLength <= 0 {
		otpLength = defaultOTPLength
	}
	return &AuthService{
		users:           users,
		roles:           roles,
		sessions:        sessions,
		passwordResets:  resets,
		storage:         storage,
		jwt:             jwtManager,
		googleAudience:  googleAudience,
		profileBucket:   profileBucket,
		defaultRoleName: "user",
		adminRoleName:   "admin",
		httpClient:      &http.Client{Timeout: 10 * time.Second},
		mailer:          mailer,
		resetTTL:        resetTTL,
		otpLength:       otpLength,
	}
}

func (s *AuthService) RegisterWithEmail(ctx context.Context, email, password string) (*AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return nil, fmt.Errorf("email and password required")
	}
	if err := util.ValidatePassword(password); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPasswordTooWeak, err)
	}

	role, err := s.roles.GetOrCreateRole(ctx, s.defaultRoleName, "Default application role")
	if err != nil {
		return nil, err
	}

	hash, salt, err := util.DerivePassword(password)
	if err != nil {
		return nil, err
	}

	user, err := s.users.CreateEmailUser(ctx, email, hash, salt)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailAlreadyUsed
		}
		return nil, err
	}

	if err = s.roles.AssignUserRole(ctx, user.ID, role.ID); err != nil {
		return nil, err
	}

	if user, err = s.users.FindByID(ctx, user.ID); err != nil {
		return nil, err
	}

	return s.issueSession(ctx, user)
}

func (s *AuthService) LoginWithEmail(ctx context.Context, email, password string) (*AuthResult, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	if ok := util.VerifyPassword(password, user.PasswordSalt, user.PasswordHash); !ok {
		return nil, ErrInvalidCredentials
	}

	return s.issueSession(ctx, user)
}

func (s *AuthService) LoginWithGoogle(ctx context.Context, idToken string) (*AuthResult, error) {
	if strings.TrimSpace(idToken) == "" {
		return nil, errors.New("id token required")
	}
	if s.googleAudience == "" {
		return nil, errors.New("google audience not configured")
	}

	payload, err := idtoken.Validate(ctx, idToken, s.googleAudience)
	if err != nil {
		return nil, errors.New("invalid google token")
	}

	email, _ := payload.Claims["email"].(string)
	if email == "" {
		return nil, errors.New("google token missing email")
	}

	role, err := s.roles.GetOrCreateRole(ctx, s.defaultRoleName, "Default application role")
	if err != nil {
		return nil, err
	}

	var namePtr *string
	if name, ok := payload.Claims["name"].(string); ok {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			namePtr = &trimmed
		}
	}

	var picturePtr *string
	if picture, ok := payload.Claims["picture"].(string); ok {
		trimmed := strings.TrimSpace(picture)
		if trimmed != "" {
			picturePtr = &trimmed
		}
	}

	var existingUser *domain.User
	if picturePtr != nil {
		if found, findErr := s.users.FindByEmail(ctx, email); findErr == nil {
			existingUser = found
		} else if !isNotFound(findErr) {
			return nil, findErr
		}
		if existingUser != nil && !s.shouldCacheGooglePicture(existingUser.ImageURL, *picturePtr) {
			picturePtr = nil
		}
	}

	user, err := s.users.UpsertGoogleUser(ctx, email, namePtr, picturePtr)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrEmailAlreadyUsed
		}
		return nil, err
	}

	if err = s.roles.AssignUserRole(ctx, user.ID, role.ID); err != nil {
		return nil, err
	}

	if user, err = s.users.FindByID(ctx, user.ID); err != nil {
		return nil, err
	}

	if picturePtr != nil && s.shouldCacheGooglePicture(user.ImageURL, *picturePtr) && s.storage != nil && s.profileBucket != "" {
		if cachedURL, cacheErr := s.cacheGoogleProfileImage(ctx, user.ID, *picturePtr); cacheErr == nil && cachedURL != nil {
			updated, updateErr := s.users.UpdateProfile(ctx, user.ID, nil, nil, cachedURL, user.ProfileCompleted)
			if updateErr != nil {
				return nil, updateErr
			}
			user = updated
		}
	}

	return s.issueSession(ctx, user)
}

func (s *AuthService) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return fmt.Errorf("email required")
	}

	if s.passwordResets == nil {
		return errors.New("password reset repository not configured")
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if isNotFound(err) {
			return nil
		}
		return err
	}

	if err := s.passwordResets.ConsumeByUser(ctx, user.ID); err != nil {
		return err
	}

	otp, err := util.GenerateNumericOTP(s.otpLength)
	if err != nil {
		return err
	}

	hash, salt, err := util.DerivePassword(otp)
	if err != nil {
		return err
	}

	reset, err := s.passwordResets.Create(ctx, user.ID, hash, salt, time.Now().Add(s.resetTTL))
	if err != nil {
		return err
	}

	if s.mailer != nil {
		if err := s.mailer.SendPasswordReset(ctx, user.Email, otp); err != nil {
			_ = s.passwordResets.MarkConsumed(ctx, reset.ID)
			return err
		}
	}

	return nil
}

func (s *AuthService) ConfirmPasswordReset(ctx context.Context, email, otp, newPassword string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" {
		return ErrResetOTPInvalid
	}
	otp = strings.TrimSpace(otp)
	if otp == "" {
		return ErrResetOTPInvalid
	}
	newPassword = strings.TrimSpace(newPassword)
	if newPassword == "" {
		return ErrPasswordTooWeak
	}
	if err := util.ValidatePassword(newPassword); err != nil {
		return fmt.Errorf("%w: %v", ErrPasswordTooWeak, err)
	}

	if s.passwordResets == nil {
		return errors.New("password reset repository not configured")
	}

	user, err := s.users.FindByEmail(ctx, email)
	if err != nil {
		if isNotFound(err) {
			return ErrResetOTPInvalid
		}
		return err
	}

	now := time.Now()
	reset, err := s.passwordResets.FindActiveByUser(ctx, user.ID, now)
	if err != nil {
		if isNotFound(err) {
			return ErrResetOTPInvalid
		}
		return err
	}

	if reset.ExpiresAt.Before(now) {
		_ = s.passwordResets.MarkConsumed(ctx, reset.ID)
		return ErrResetOTPExpired
	}

	if !util.VerifyPassword(otp, reset.OTPSalt, reset.OTPHash) {
		return ErrResetOTPInvalid
	}

	hash, salt, err := util.DerivePassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.users.UpdatePassword(ctx, user.ID, hash, salt); err != nil {
		return err
	}

	if err := s.passwordResets.MarkConsumed(ctx, reset.ID); err != nil {
		return err
	}

	if err := s.passwordResets.ConsumeByUser(ctx, user.ID); err != nil {
		return err
	}

	return nil
}

func (s *AuthService) CompleteProfile(ctx context.Context, userID uuid.UUID, fullName, username *string, image *ProfileImage) (*domain.User, error) {
	normalizedFullName := normalizePtr(fullName)
	normalizedUsername := normalizePtr(username)

	var imageURL *string
	if image != nil && image.Reader != nil && image.Size > 0 {
		objectName := fmt.Sprintf("profiles/%s/%d", userID.String(), time.Now().UnixNano())
		if ext := sanitizeExt(image.FileName); ext != "" {
			objectName += ext
		}
		url, err := s.storage.Upload(ctx, s.profileBucket, objectName, image.ContentType, image.Reader, image.Size)
		if err != nil {
			return nil, err
		}
		imageURL = &url
	}

	profileCompleted := normalizedFullName != nil && normalizedUsername != nil

	user, err := s.users.UpdateProfile(ctx, userID, normalizedFullName, normalizedUsername, imageURL, profileCompleted)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, ErrUsernameAlreadyUsed
		}
		return nil, err
	}
	return user, nil
}

func (s *AuthService) cacheGoogleProfileImage(ctx context.Context, userID uuid.UUID, pictureURL string) (*string, error) {
	if pictureURL == "" || s.storage == nil || s.profileBucket == "" {
		return nil, nil
	}
	if s.httpClient == nil {
		return nil, errors.New("http client not configured")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, pictureURL, nil)
	if err != nil {
		return nil, err
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("fetch google picture: status %d", resp.StatusCode)
	}

	limitedReader := io.LimitReader(resp.Body, maxGoogleProfileImageBytes+1)
	data, err := io.ReadAll(limitedReader)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxGoogleProfileImageBytes {
		return nil, fmt.Errorf("google profile image exceeds %d bytes", maxGoogleProfileImageBytes)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = http.DetectContentType(data)
	}
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	objectName := fmt.Sprintf("profiles/%s/google/%d", userID.String(), time.Now().UnixNano())
	if ext := extensionFromContentType(contentType); ext != "" {
		objectName += ext
	}

	uploadURL, err := s.storage.Upload(ctx, s.profileBucket, objectName, contentType, bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, err
	}

	return &uploadURL, nil
}

func (s *AuthService) ChangePassword(ctx context.Context, userID uuid.UUID, currentPassword, newPassword string) error {
	newPassword = strings.TrimSpace(newPassword)
	if newPassword == "" {
		return ErrPasswordTooWeak
	}
	if err := util.ValidatePassword(newPassword); err != nil {
		return fmt.Errorf("%w: %v", ErrPasswordTooWeak, err)
	}

	user, err := s.users.FindByID(ctx, userID)
	if err != nil {
		if isNotFound(err) {
			return ErrInvalidCredentials
		}
		return err
	}

	hasPassword := len(user.PasswordSalt) > 0 && len(user.PasswordHash) > 0
	if hasPassword {
		if !util.VerifyPassword(currentPassword, user.PasswordSalt, user.PasswordHash) {
			return ErrPasswordMismatch
		}
	}

	hash, salt, err := util.DerivePassword(newPassword)
	if err != nil {
		return err
	}

	if err := s.users.UpdatePassword(ctx, userID, hash, salt); err != nil {
		return err
	}

	return nil
}

func (s *AuthService) Authenticate(ctx context.Context, token string) (*domain.User, error) {
	claims, err := s.jwt.Parse(token)
	if err != nil {
		return nil, ErrTokenInvalid
	}

	if _, err = s.sessions.FindActiveSession(ctx, token); err != nil {
		if isNotFound(err) {
			return nil, ErrTokenInvalid
		}
		return nil, err
	}

	user, err := s.users.FindByID(ctx, claims.UserID)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrTokenInvalid
		}
		return nil, err
	}
	return user, nil
}

func (s *AuthService) Logout(ctx context.Context, token string) error {
	return s.sessions.DeactivateSession(ctx, token)
}

func (s *AuthService) ListUsers(ctx context.Context, limit, offset int) ([]domain.User, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	return s.users.List(ctx, limit, offset)
}

func (s *AuthService) DeleteUser(ctx context.Context, actor *domain.User, target uuid.UUID) error {
	if actor == nil {
		return ErrForbidden
	}

	if actor.ID != target {
		adminRole, err := s.roles.GetOrCreateRole(ctx, s.adminRoleName, "Administrator role with elevated permissions")
		if err != nil {
			return err
		}
		if !actor.HasRole(adminRole.ID) {
			return ErrForbidden
		}
	}

	if err := s.users.Delete(ctx, target); err != nil {
		if isNotFound(err) {
			return ErrUserNotFound
		}
		return err
	}
	return nil
}

func (s *AuthService) issueSession(ctx context.Context, user *domain.User) (*AuthResult, error) {
	token, expiresAt, err := s.jwt.Generate(user.ID, user.Email, user.Username, user.ProfileCompleted)
	if err != nil {
		return nil, err
	}

	if _, err = s.sessions.CreateSession(ctx, user.ID, token, expiresAt); err != nil {
		return nil, err
	}

	return &AuthResult{Token: token, ExpiresAt: expiresAt, User: user}, nil
}

func sanitizeExt(name string) string {
	if name == "" {
		return ""
	}
	idx := strings.LastIndex(name, ".")
	if idx == -1 {
		return ""
	}
	ext := strings.ToLower(name[idx:])
	if len(ext) > 10 {
		return ""
	}
	return ext
}

func normalizePtr(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func (s *AuthService) shouldCacheGooglePicture(existing *string, pictureURL string) bool {
	pictureURL = strings.TrimSpace(pictureURL)
	if pictureURL == "" {
		return false
	}
	if existing == nil {
		return true
	}
	current := strings.TrimSpace(*existing)
	if current == "" {
		return true
	}
	if strings.EqualFold(current, pictureURL) {
		return true
	}
	lowerCurrent := strings.ToLower(current)
	if strings.Contains(lowerCurrent, "googleusercontent.com") {
		return true
	}
	if strings.Contains(lowerCurrent, "/google/") {
		return true
	}
	return false
}

func extensionFromContentType(contentType string) string {
	if contentType == "" {
		return ""
	}
	exts, err := mime.ExtensionsByType(contentType)
	if err != nil {
		return ""
	}
	for _, ext := range exts {
		ext = strings.ToLower(strings.TrimSpace(ext))
		if ext == "" {
			continue
		}
		if len(ext) > 10 {
			continue
		}
		return ext
	}
	return ""
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func isNotFound(err error) bool {
	return errors.Is(err, sql.ErrNoRows)
}
