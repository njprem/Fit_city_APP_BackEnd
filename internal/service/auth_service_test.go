package service

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/util"
)

type fakeUserRepo struct {
	createEmailEmail  string
	createEmailHash   []byte
	createEmailSalt   []byte
	createEmailResult *domain.User
	createEmailErr    error

	upsertGoogleEmail  string
	upsertGoogleName   *string
	upsertGoogleImg    *string
	upsertGoogleResult *domain.User
	upsertGoogleErr    error

	findByEmailInput  string
	findByEmailResult *domain.User
	findByEmailErr    error

	findByIDInput  uuid.UUID
	findByIDResult *domain.User
	findByIDErr    error

	updateProfileInput struct {
		id               uuid.UUID
		fullName         *string
		username         *string
		imageURL         *string
		profileCompleted bool
	}
	updateProfileResult *domain.User
	updateProfileErr    error

	updatePasswordInput struct {
		id   uuid.UUID
		hash []byte
		salt []byte
	}
	updatePasswordErr error

	listInputs []struct {
		limit  int
		offset int
	}
	listResult []domain.User
	listErr    error

	deleteInput uuid.UUID
	deleteErr   error
}

func (f *fakeUserRepo) CreateEmailUser(ctx context.Context, email string, passwordHash, passwordSalt []byte) (*domain.User, error) {
	f.createEmailEmail = email
	f.createEmailHash = append([]byte(nil), passwordHash...)
	f.createEmailSalt = append([]byte(nil), passwordSalt...)
	return f.createEmailResult, f.createEmailErr
}

func (f *fakeUserRepo) UpsertGoogleUser(ctx context.Context, email string, fullName *string, imageURL *string) (*domain.User, error) {
	f.upsertGoogleEmail = email
	f.upsertGoogleName = fullName
	f.upsertGoogleImg = imageURL
	return f.upsertGoogleResult, f.upsertGoogleErr
}

func (f *fakeUserRepo) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	f.findByEmailInput = email
	return f.findByEmailResult, f.findByEmailErr
}

func (f *fakeUserRepo) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	f.findByIDInput = id
	return f.findByIDResult, f.findByIDErr
}

func (f *fakeUserRepo) UpdateProfile(ctx context.Context, id uuid.UUID, fullName *string, username *string, imageURL *string, profileCompleted bool) (*domain.User, error) {
	f.updateProfileInput = struct {
		id               uuid.UUID
		fullName         *string
		username         *string
		imageURL         *string
		profileCompleted bool
	}{id: id, fullName: fullName, username: username, imageURL: imageURL, profileCompleted: profileCompleted}
	return f.updateProfileResult, f.updateProfileErr
}

func (f *fakeUserRepo) UpdatePassword(ctx context.Context, id uuid.UUID, passwordHash, passwordSalt []byte) error {
	f.updatePasswordInput = struct {
		id   uuid.UUID
		hash []byte
		salt []byte
	}{
		id:   id,
		hash: append([]byte(nil), passwordHash...),
		salt: append([]byte(nil), passwordSalt...),
	}
	return f.updatePasswordErr
}

func (f *fakeUserRepo) List(ctx context.Context, limit, offset int) ([]domain.User, error) {
	f.listInputs = append(f.listInputs, struct {
		limit  int
		offset int
	}{limit: limit, offset: offset})
	if f.listErr != nil {
		return nil, f.listErr
	}
	return append([]domain.User(nil), f.listResult...), nil
}

func (f *fakeUserRepo) Delete(ctx context.Context, id uuid.UUID) error {
	f.deleteInput = id
	return f.deleteErr
}

type fakeRoleRepo struct {
	roleResult *domain.Role
	roleErr    error

	assignedPairs []struct {
		userID uuid.UUID
		roleID uuid.UUID
	}
	assignErr error
}

func (f *fakeRoleRepo) GetOrCreateRole(ctx context.Context, name, description string) (*domain.Role, error) {
	if f.roleErr != nil {
		return nil, f.roleErr
	}
	return f.roleResult, nil
}

func (f *fakeRoleRepo) AssignUserRole(ctx context.Context, userID, roleID uuid.UUID) error {
	f.assignedPairs = append(f.assignedPairs, struct {
		userID uuid.UUID
		roleID uuid.UUID
	}{userID: userID, roleID: roleID})
	return f.assignErr
}

type fakeSessionRepo struct {
	createdSessions []struct {
		userID    uuid.UUID
		token     string
		expiresAt time.Time
	}
	createResult *domain.Session
	createErr    error

	findActiveToken  string
	findActiveResult *domain.Session
	findActiveErr    error

	deactivatedToken string
	deactivateErr    error
}

func (f *fakeSessionRepo) CreateSession(ctx context.Context, userID uuid.UUID, token string, expiresAt time.Time) (*domain.Session, error) {
	f.createdSessions = append(f.createdSessions, struct {
		userID    uuid.UUID
		token     string
		expiresAt time.Time
	}{userID: userID, token: token, expiresAt: expiresAt})
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResult != nil {
		return f.createResult, nil
	}
	return &domain.Session{ID: 1, UserID: userID, Token: token, ExpiresAt: expiresAt, IsActive: true}, nil
}

func (f *fakeSessionRepo) DeactivateSession(ctx context.Context, token string) error {
	f.deactivatedToken = token
	return f.deactivateErr
}

func (f *fakeSessionRepo) FindActiveSession(ctx context.Context, token string) (*domain.Session, error) {
	f.findActiveToken = token
	if f.findActiveErr != nil {
		return nil, f.findActiveErr
	}
	if f.findActiveResult != nil {
		return f.findActiveResult, nil
	}
	return &domain.Session{ID: 1, Token: token, IsActive: true, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

type fakeStorage struct {
	uploaded []struct {
		bucket      string
		objectName  string
		contentType string
		size        int64
	}
	url string
	err error
}

func (f *fakeStorage) Upload(ctx context.Context, bucket, objectName, contentType string, reader io.Reader, size int64) (string, error) {
	f.uploaded = append(f.uploaded, struct {
		bucket      string
		objectName  string
		contentType string
		size        int64
	}{bucket: bucket, objectName: objectName, contentType: contentType, size: size})
	if f.err != nil {
		return "", f.err
	}
	if f.url != "" {
		return f.url, nil
	}
	return "https://storage/" + objectName, nil
}

type fakePasswordResetRepo struct {
	consumeCalls []uuid.UUID
	consumeErr   error

	createCalls []struct {
		userID    uuid.UUID
		hash      []byte
		salt      []byte
		expiresAt time.Time
	}
	createResult *domain.PasswordReset
	createErr    error

	findCalls []struct {
		userID uuid.UUID
		now    time.Time
	}
	findErr    error
	findResult *domain.PasswordReset
	findByUser map[uuid.UUID]*domain.PasswordReset

	markCalls []int64
	markErr   error
}

func (f *fakePasswordResetRepo) ConsumeByUser(ctx context.Context, userID uuid.UUID) error {
	f.consumeCalls = append(f.consumeCalls, userID)
	if f.consumeErr != nil {
		return f.consumeErr
	}
	return nil
}

func (f *fakePasswordResetRepo) Create(ctx context.Context, userID uuid.UUID, otpHash, otpSalt []byte, expiresAt time.Time) (*domain.PasswordReset, error) {
	f.createCalls = append(f.createCalls, struct {
		userID    uuid.UUID
		hash      []byte
		salt      []byte
		expiresAt time.Time
	}{userID: userID, hash: append([]byte(nil), otpHash...), salt: append([]byte(nil), otpSalt...), expiresAt: expiresAt})
	if f.createErr != nil {
		return nil, f.createErr
	}
	if f.createResult != nil {
		clone := *f.createResult
		return &clone, nil
	}
	return &domain.PasswordReset{
		ID:        int64(len(f.createCalls)),
		UserID:    userID,
		OTPHash:   append([]byte(nil), otpHash...),
		OTPSalt:   append([]byte(nil), otpSalt...),
		ExpiresAt: expiresAt,
		Consumed:  false,
		CreatedAt: time.Now(),
	}, nil
}

func (f *fakePasswordResetRepo) FindActiveByUser(ctx context.Context, userID uuid.UUID, now time.Time) (*domain.PasswordReset, error) {
	f.findCalls = append(f.findCalls, struct {
		userID uuid.UUID
		now    time.Time
	}{userID: userID, now: now})
	if f.findErr != nil {
		return nil, f.findErr
	}
	if f.findByUser != nil {
		if reset, ok := f.findByUser[userID]; ok && reset != nil {
			clone := *reset
			return &clone, nil
		}
	}
	if f.findResult != nil {
		clone := *f.findResult
		return &clone, nil
	}
	return nil, sql.ErrNoRows
}

func (f *fakePasswordResetRepo) MarkConsumed(ctx context.Context, id int64) error {
	f.markCalls = append(f.markCalls, id)
	if f.markErr != nil {
		return f.markErr
	}
	return nil
}

type fakeResetMailer struct {
	sent []struct {
		email string
		otp   string
	}
	err error
}

func (f *fakeResetMailer) SendPasswordReset(ctx context.Context, email, otp string) error {
	f.sent = append(f.sent, struct {
		email string
		otp   string
	}{email: email, otp: otp})
	if f.err != nil {
		return f.err
	}
	return nil
}

type fakeHTTPClient struct {
	resp     *http.Response
	err      error
	requests []*http.Request
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	if f.resp == nil {
		return nil, errors.New("no response configured")
	}
	return f.resp, nil
}

func newAuthServiceForTests(user *fakeUserRepo, role *fakeRoleRepo, session *fakeSessionRepo, storage *fakeStorage, resets *fakePasswordResetRepo, mailer PasswordResetSender) *AuthService {
	if role == nil {
		trole := &fakeRoleRepo{}
		role = trole
	}
	if session == nil {
		session = &fakeSessionRepo{}
	}
	if storage == nil {
		storage = &fakeStorage{}
	}
	if resets == nil {
		resets = &fakePasswordResetRepo{}
	}
	jwtManager := util.NewJWTManager("test-secret", time.Hour)
	svc := NewAuthService(user, role, session, resets, storage, mailer, jwtManager, "google-audience", "profile-bucket", 15*time.Minute, 6)
	svc.httpClient = &fakeHTTPClient{resp: &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(bytes.NewReader([]byte{})), Header: http.Header{}}}
	return svc
}

func TestRegisterWithEmailSuccess(t *testing.T) {
	ctx := context.Background()
	roleID := uuid.New()
	userID := uuid.New()
	role := domain.Role{ID: roleID, Name: "user", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	userRepo := &fakeUserRepo{
		createEmailResult: &domain.User{ID: userID, Email: "test@example.com", CreatedAt: time.Now(), UpdatedAt: time.Now()},
		findByIDResult:    &domain.User{ID: userID, Email: "test@example.com", CreatedAt: time.Now(), UpdatedAt: time.Now(), Roles: []domain.Role{role}},
	}
	roleRepo := &fakeRoleRepo{roleResult: &domain.Role{ID: roleID}}
	sessionRepo := &fakeSessionRepo{}
	storage := &fakeStorage{}

	svc := newAuthServiceForTests(userRepo, roleRepo, sessionRepo, storage, nil, nil)

	result, err := svc.RegisterWithEmail(ctx, "Test@Example.com ", "SuperSecret1!")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if result.User == nil || result.User.ID != userRepo.createEmailResult.ID {
		t.Fatalf("unexpected user in result: %+v", result.User)
	}
	if userRepo.createEmailEmail != "test@example.com" {
		t.Fatalf("email should be normalized, got %q", userRepo.createEmailEmail)
	}
	if len(userRepo.createEmailHash) == 0 || len(userRepo.createEmailSalt) == 0 {
		t.Fatalf("expected password hash and salt to be set")
	}
	if len(sessionRepo.createdSessions) != 1 {
		t.Fatalf("expected session to be created, got %d", len(sessionRepo.createdSessions))
	}
	if len(roleRepo.assignedPairs) != 1 || roleRepo.assignedPairs[0].roleID != roleID {
		t.Fatalf("expected role assignment to be recorded")
	}
	if userRepo.findByIDInput != userID {
		t.Fatalf("expected user to be reloaded after role assignment")
	}
	if result.User == nil || !result.User.HasRole(roleID) {
		t.Fatal("expected resulting user to include assigned role")
	}
	if result.Token == "" {
		t.Fatal("expected JWT token in result")
	}
}

func TestRegisterWithEmailWeakPassword(t *testing.T) {
	ctx := context.Background()
	roleID := uuid.New()
	userRepo := &fakeUserRepo{}
	roleRepo := &fakeRoleRepo{roleResult: &domain.Role{ID: roleID}}
	sessionRepo := &fakeSessionRepo{}
	storage := &fakeStorage{}

	svc := newAuthServiceForTests(userRepo, roleRepo, sessionRepo, storage, nil, nil)

	_, err := svc.RegisterWithEmail(ctx, "weak@example.com", "weakpass")
	if !errors.Is(err, ErrPasswordTooWeak) {
		t.Fatalf("expected ErrPasswordTooWeak, got %v", err)
	}
	if len(userRepo.createEmailHash) != 0 {
		t.Fatal("expected no password hash to be stored for invalid password")
	}
}

func TestRegisterWithEmailEmailExists(t *testing.T) {
	ctx := context.Background()
	roleID := uuid.New()
	userRepo := &fakeUserRepo{createEmailErr: &pgconn.PgError{Code: "23505"}}
	roleRepo := &fakeRoleRepo{roleResult: &domain.Role{ID: roleID}}
	sessionRepo := &fakeSessionRepo{}
	storage := &fakeStorage{}

	svc := newAuthServiceForTests(userRepo, roleRepo, sessionRepo, storage, nil, nil)

	_, err := svc.RegisterWithEmail(ctx, "duplicate@example.com", "ValidPass123!")
	if !errors.Is(err, ErrEmailAlreadyUsed) {
		t.Fatalf("expected ErrEmailAlreadyUsed, got %v", err)
	}
	if len(sessionRepo.createdSessions) != 0 {
		t.Fatalf("expected no session to be created on error")
	}
}

func TestLoginWithEmailInvalidCredentials(t *testing.T) {
	t.Run("user not found", func(t *testing.T) {
		userRepo := &fakeUserRepo{findByEmailErr: sql.ErrNoRows}
		svc := newAuthServiceForTests(userRepo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)

		_, err := svc.LoginWithEmail(context.Background(), "none@example.com", "password")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("password mismatch", func(t *testing.T) {
		hash, salt, _ := util.DerivePassword("different")
		user := &domain.User{ID: uuid.New(), Email: "test@example.com", PasswordHash: hash, PasswordSalt: salt}
		userRepo := &fakeUserRepo{findByEmailResult: user}
		svc := newAuthServiceForTests(userRepo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)

		_, err := svc.LoginWithEmail(context.Background(), "test@example.com", "password")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestLoginWithEmailSuccess(t *testing.T) {
	hash, salt, _ := util.DerivePassword("right-password")
	role := domain.Role{ID: uuid.New(), Name: "user", CreatedAt: time.Now(), UpdatedAt: time.Now()}
	user := &domain.User{ID: uuid.New(), Email: "test@example.com", PasswordHash: hash, PasswordSalt: salt, CreatedAt: time.Now(), UpdatedAt: time.Now(), Roles: []domain.Role{role}}
	userRepo := &fakeUserRepo{findByEmailResult: user}
	sessionRepo := &fakeSessionRepo{}
	svc := newAuthServiceForTests(userRepo, &fakeRoleRepo{}, sessionRepo, &fakeStorage{}, nil, nil)

	result, err := svc.LoginWithEmail(context.Background(), "test@example.com", "right-password")
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if len(sessionRepo.createdSessions) != 1 {
		t.Fatalf("expected session to be created, got %d", len(sessionRepo.createdSessions))
	}
	if result.User == nil || result.User.ID != user.ID {
		t.Fatalf("unexpected user in response")
	}
	if !result.User.HasRole(role.ID) {
		t.Fatal("expected returned user to retain roles")
	}
}

func TestChangePassword(t *testing.T) {
	ctx := context.Background()

	t.Run("success when current password matches", func(t *testing.T) {
		hash, salt, _ := util.DerivePassword("old-pass")
		user := &domain.User{ID: uuid.New(), PasswordHash: hash, PasswordSalt: salt}
		repo := &fakeUserRepo{findByIDResult: user}
		svc := newAuthServiceForTests(repo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)

		if err := svc.ChangePassword(ctx, user.ID, "old-pass", "NewPassword1!"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.updatePasswordInput.id != user.ID {
			t.Fatalf("expected password update for user %s", user.ID)
		}
		if len(repo.updatePasswordInput.hash) == 0 || len(repo.updatePasswordInput.salt) == 0 {
			t.Fatalf("expected new hash and salt to be set")
		}
		if string(repo.updatePasswordInput.hash) == string(hash) {
			t.Fatalf("expected new hash to differ from old hash")
		}
	})

	t.Run("fails when current password mismatches", func(t *testing.T) {
		hash, salt, _ := util.DerivePassword("old-pass")
		user := &domain.User{ID: uuid.New(), PasswordHash: hash, PasswordSalt: salt}
		repo := &fakeUserRepo{findByIDResult: user}
		svc := newAuthServiceForTests(repo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)

		err := svc.ChangePassword(ctx, user.ID, "wrong-pass", "NewPassword1!")
		if !errors.Is(err, ErrPasswordMismatch) {
			t.Fatalf("expected ErrPasswordMismatch, got %v", err)
		}
	})

	t.Run("fails when new password empty", func(t *testing.T) {
		repo := &fakeUserRepo{findByIDResult: &domain.User{ID: uuid.New()}}
		svc := newAuthServiceForTests(repo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)

		err := svc.ChangePassword(ctx, repo.findByIDResult.ID, "", "   ")
		if !errors.Is(err, ErrPasswordTooWeak) {
			t.Fatalf("expected ErrPasswordTooWeak, got %v", err)
		}
	})

	t.Run("fails when new password lacks complexity", func(t *testing.T) {
		user := &domain.User{ID: uuid.New()}
		repo := &fakeUserRepo{findByIDResult: user}
		svc := newAuthServiceForTests(repo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)

		err := svc.ChangePassword(ctx, user.ID, "", "alllowercase123")
		if !errors.Is(err, ErrPasswordTooWeak) {
			t.Fatalf("expected ErrPasswordTooWeak, got %v", err)
		}
	})

	t.Run("allows setting password when none exists", func(t *testing.T) {
		user := &domain.User{ID: uuid.New()}
		repo := &fakeUserRepo{findByIDResult: user}
		svc := newAuthServiceForTests(repo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)

		if err := svc.ChangePassword(ctx, user.ID, "", "FreshPass12!"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.updatePasswordInput.id != user.ID {
			t.Fatalf("expected password update for user %s", user.ID)
		}
	})

	t.Run("propagates missing user", func(t *testing.T) {
		repo := &fakeUserRepo{findByIDErr: sql.ErrNoRows}
		svc := newAuthServiceForTests(repo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)

		err := svc.ChangePassword(ctx, uuid.New(), "old", "NewPassword1!")
		if !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestCacheGoogleProfileImage(t *testing.T) {
	userID := uuid.New()
	storage := &fakeStorage{}
	svc := newAuthServiceForTests(&fakeUserRepo{}, &fakeRoleRepo{}, &fakeSessionRepo{}, storage, nil, nil)

	imageBytes := bytes.Repeat([]byte{0x89}, 1024)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Body:       io.NopCloser(bytes.NewReader(imageBytes)),
		Header:     http.Header{"Content-Type": []string{"image/png"}},
	}
	fakeHTTP := &fakeHTTPClient{resp: resp}
	svc.httpClient = fakeHTTP

	url, err := svc.cacheGoogleProfileImage(context.Background(), userID, "https://example.com/avatar.png")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if url == nil || *url == "" {
		t.Fatalf("expected uploaded url, got %v", url)
	}
	if len(storage.uploaded) != 1 {
		t.Fatalf("expected one upload, got %d", len(storage.uploaded))
	}
	if storage.uploaded[0].bucket != "profile-bucket" {
		t.Fatalf("unexpected bucket: %s", storage.uploaded[0].bucket)
	}
	if storage.uploaded[0].contentType != "image/png" {
		t.Fatalf("unexpected content type: %s", storage.uploaded[0].contentType)
	}
	if storage.uploaded[0].size != int64(len(imageBytes)) {
		t.Fatalf("unexpected size: %d", storage.uploaded[0].size)
	}
	if !strings.Contains(storage.uploaded[0].objectName, fmt.Sprintf("profiles/%s/google/", userID.String())) {
		t.Fatalf("unexpected object name: %s", storage.uploaded[0].objectName)
	}
	if len(fakeHTTP.requests) != 1 {
		t.Fatalf("expected one HTTP request, got %d", len(fakeHTTP.requests))
	}
	if fakeHTTP.requests[0].URL.String() != "https://example.com/avatar.png" {
		t.Fatalf("unexpected request url: %s", fakeHTTP.requests[0].URL)
	}
}

func TestShouldCacheGooglePicture(t *testing.T) {
	svc := newAuthServiceForTests(&fakeUserRepo{}, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)

	googleURL := "https://lh3.googleusercontent.com/avatar"
	otherURL := "https://cdn.example.com/avatar.png"

	tests := []struct {
		name     string
		existing *string
		picture  string
		want     bool
	}{
		{name: "nil existing", existing: nil, picture: googleURL, want: true},
		{name: "blank existing", existing: authStringPtr("  "), picture: googleURL, want: true},
		{name: "same url", existing: authStringPtr(googleURL), picture: googleURL, want: true},
		{name: "google domain", existing: authStringPtr(googleURL + "?sz=64"), picture: "https://photos.googleusercontent.com/avatar2", want: true},
		{name: "different domain", existing: authStringPtr(otherURL), picture: googleURL, want: false},
		{name: "empty picture", existing: authStringPtr(otherURL), picture: "  ", want: false},
	}

	for _, tc := range tests {
		if got := svc.shouldCacheGooglePicture(tc.existing, tc.picture); got != tc.want {
			t.Fatalf("%s: expected %v, got %v", tc.name, tc.want, got)
		}
	}
}

func TestRequestPasswordReset(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{ID: uuid.New(), Email: "reset@example.com"}
	userRepo := &fakeUserRepo{findByEmailResult: user}
	resetRepo := &fakePasswordResetRepo{}
	mailer := &fakeResetMailer{}
	svc := newAuthServiceForTests(userRepo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, resetRepo, mailer)

	if err := svc.RequestPasswordReset(ctx, user.Email); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resetRepo.consumeCalls) != 1 || resetRepo.consumeCalls[0] != user.ID {
		t.Fatalf("expected consume call for user")
	}
	if len(resetRepo.createCalls) != 1 {
		t.Fatalf("expected single create call, got %d", len(resetRepo.createCalls))
	}
	if resetRepo.createCalls[0].userID != user.ID {
		t.Fatalf("expected create for user")
	}
	if len(resetRepo.createCalls[0].hash) == 0 || len(resetRepo.createCalls[0].salt) == 0 {
		t.Fatalf("expected hash and salt to be set")
	}
	if len(mailer.sent) != 1 {
		t.Fatalf("expected email to be sent")
	}
	if len(mailer.sent[0].otp) != svc.otpLength {
		t.Fatalf("expected otp length %d, got %d", svc.otpLength, len(mailer.sent[0].otp))
	}

	t.Run("user not found returns nil", func(t *testing.T) {
		missingRepo := &fakeUserRepo{findByEmailErr: sql.ErrNoRows}
		svc := newAuthServiceForTests(missingRepo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, &fakePasswordResetRepo{}, mailer)
		if err := svc.RequestPasswordReset(ctx, "none@example.com"); err != nil {
			t.Fatalf("expected no error, got %v", err)
		}
	})

	t.Run("mailer failure marks token consumed", func(t *testing.T) {
		userRepo := &fakeUserRepo{findByEmailResult: user}
		resetRepo := &fakePasswordResetRepo{}
		mailer := &fakeResetMailer{err: errors.New("smtp down")}
		svc := newAuthServiceForTests(userRepo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, resetRepo, mailer)
		err := svc.RequestPasswordReset(ctx, user.Email)
		if err == nil {
			t.Fatalf("expected error when mailer fails")
		}
		if len(resetRepo.markCalls) == 0 {
			t.Fatalf("expected reset to be marked consumed when mail fails")
		}
	})
}

func TestConfirmPasswordReset(t *testing.T) {
	ctx := context.Background()
	user := &domain.User{ID: uuid.New(), Email: "reset@example.com"}
	hash, salt, _ := util.DerivePassword("123456")
	reset := &domain.PasswordReset{
		ID:        1,
		UserID:    user.ID,
		OTPHash:   hash,
		OTPSalt:   salt,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	userRepo := &fakeUserRepo{findByEmailResult: user}
	resetRepo := &fakePasswordResetRepo{findByUser: map[uuid.UUID]*domain.PasswordReset{user.ID: reset}}
	svc := newAuthServiceForTests(userRepo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, resetRepo, &fakeResetMailer{})

	if err := svc.ConfirmPasswordReset(ctx, user.Email, "123456", "ResetPass12!"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resetRepo.markCalls) == 0 {
		t.Fatalf("expected reset to be marked consumed")
	}
	if len(userRepo.updatePasswordInput.hash) == 0 {
		t.Fatalf("expected password to be updated")
	}

	t.Run("fails when new password weak", func(t *testing.T) {
		userRepo := &fakeUserRepo{findByEmailResult: user}
		resetRepo := &fakePasswordResetRepo{findByUser: map[uuid.UUID]*domain.PasswordReset{user.ID: reset}}
		svc := newAuthServiceForTests(userRepo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, resetRepo, &fakeResetMailer{})
		err := svc.ConfirmPasswordReset(ctx, user.Email, "123456", "weakpassword")
		if !errors.Is(err, ErrPasswordTooWeak) {
			t.Fatalf("expected ErrPasswordTooWeak, got %v", err)
		}
	})

	t.Run("invalid otp", func(t *testing.T) {
		userRepo := &fakeUserRepo{findByEmailResult: user}
		resetRepo := &fakePasswordResetRepo{findByUser: map[uuid.UUID]*domain.PasswordReset{user.ID: reset}}
		svc := newAuthServiceForTests(userRepo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, resetRepo, &fakeResetMailer{})
		err := svc.ConfirmPasswordReset(ctx, user.Email, "000000", "ResetPass12!")
		if !errors.Is(err, ErrResetOTPInvalid) {
			t.Fatalf("expected ErrResetOTPInvalid, got %v", err)
		}
	})

	t.Run("expired otp", func(t *testing.T) {
		expired := *reset
		expired.ExpiresAt = time.Now().Add(-time.Minute)
		userRepo := &fakeUserRepo{findByEmailResult: user}
		resetRepo := &fakePasswordResetRepo{findByUser: map[uuid.UUID]*domain.PasswordReset{user.ID: &expired}}
		svc := newAuthServiceForTests(userRepo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, resetRepo, &fakeResetMailer{})
		err := svc.ConfirmPasswordReset(ctx, user.Email, "123456", "ResetPass12!")
		if !errors.Is(err, ErrResetOTPExpired) {
			t.Fatalf("expected ErrResetOTPExpired, got %v", err)
		}
		if len(resetRepo.markCalls) == 0 {
			t.Fatalf("expected mark consumed to be called")
		}
	})

	t.Run("missing user", func(t *testing.T) {
		userRepo := &fakeUserRepo{findByEmailErr: sql.ErrNoRows}
		resetRepo := &fakePasswordResetRepo{}
		svc := newAuthServiceForTests(userRepo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, resetRepo, &fakeResetMailer{})
		err := svc.ConfirmPasswordReset(ctx, user.Email, "123456", "ResetPass12!")
		if !errors.Is(err, ErrResetOTPInvalid) {
			t.Fatalf("expected ErrResetOTPInvalid, got %v", err)
		}
	})
}

func TestCompleteProfileUploadAndNormalize(t *testing.T) {
	userID := uuid.New()
	storage := &fakeStorage{url: "https://cdn.example.com/profiles/avatar.png"}
	updatedUser := &domain.User{ID: userID, Username: authStringPtr("trimmed"), FullName: authStringPtr("Trimmed"), ImageURL: authStringPtr("https://cdn.example.com/profiles/avatar.png"), ProfileCompleted: true}
	userRepo := &fakeUserRepo{updateProfileResult: updatedUser}
	svc := newAuthServiceForTests(userRepo, &fakeRoleRepo{}, &fakeSessionRepo{}, storage, nil, nil)

	imgData := "fake image data"
	fullName := "  Trimmed  "
	username := "  trimmed "
	reader := strings.NewReader(imgData)
	profileImage := &ProfileImage{Reader: reader, Size: int64(len(imgData)), FileName: "avatar.PNG", ContentType: "image/png"}

	result, err := svc.CompleteProfile(context.Background(), userID, &fullName, &username, profileImage)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(storage.uploaded) != 1 {
		t.Fatalf("expected one upload, got %d", len(storage.uploaded))
	}
	if !strings.HasPrefix(storage.uploaded[0].objectName, "profiles/") {
		t.Fatalf("expected object name to live in profiles bucket, got %q", storage.uploaded[0].objectName)
	}
	if userRepo.updateProfileInput.profileCompleted != true {
		t.Fatalf("expected profile completed flag true")
	}
	if userRepo.updateProfileInput.fullName == nil || *userRepo.updateProfileInput.fullName != "Trimmed" {
		t.Fatalf("expected full name to be trimmed, got %+v", userRepo.updateProfileInput.fullName)
	}
	if userRepo.updateProfileInput.username == nil || *userRepo.updateProfileInput.username != "trimmed" {
		t.Fatalf("expected username to be trimmed, got %+v", userRepo.updateProfileInput.username)
	}
	if result.ImageURL == nil || *result.ImageURL != storage.url {
		t.Fatalf("expected returned user to include storage url")
	}
}

func TestAuthenticateSuccess(t *testing.T) {
	user := &domain.User{ID: uuid.New(), Email: "auth@example.com", ProfileCompleted: true}
	userRepo := &fakeUserRepo{findByIDResult: user}
	sessionRepo := &fakeSessionRepo{findActiveResult: &domain.Session{Token: ""}}
	svc := newAuthServiceForTests(userRepo, &fakeRoleRepo{}, sessionRepo, &fakeStorage{}, nil, nil)

	token, _, err := svc.jwt.Generate(user.ID, user.Email, nil, true)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	sessionRepo.findActiveResult.Token = token

	authenticated, err := svc.Authenticate(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if authenticated == nil || authenticated.ID != user.ID {
		t.Fatalf("expected user to be returned")
	}
	if sessionRepo.findActiveToken != token {
		t.Fatalf("expected session lookup with token")
	}
	if userRepo.findByIDInput != user.ID {
		t.Fatalf("expected user lookup by id")
	}
}

func TestLogoutDeactivatesSession(t *testing.T) {
	sessionRepo := &fakeSessionRepo{}
	svc := newAuthServiceForTests(&fakeUserRepo{}, &fakeRoleRepo{}, sessionRepo, &fakeStorage{}, nil, nil)

	if err := svc.Logout(context.Background(), "token123"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sessionRepo.deactivatedToken != "token123" {
		t.Fatalf("expected session to be deactivated with token123")
	}
}

func TestListUsers(t *testing.T) {
	ctx := context.Background()
	userA := domain.User{ID: uuid.New(), Email: "a@example.com"}
	userB := domain.User{ID: uuid.New(), Email: "b@example.com"}
	repo := &fakeUserRepo{listResult: []domain.User{userA, userB}}
	svc := newAuthServiceForTests(repo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)

	users, err := svc.ListUsers(ctx, 25, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.listInputs) != 1 {
		t.Fatalf("expected one call to list, got %d", len(repo.listInputs))
	}
	if repo.listInputs[0].limit != 25 || repo.listInputs[0].offset != 10 {
		t.Fatalf("expected limit=25 offset=10, got limit=%d offset=%d", repo.listInputs[0].limit, repo.listInputs[0].offset)
	}
	if len(users) != 2 || users[0].ID != userA.ID || users[1].ID != userB.ID {
		t.Fatalf("unexpected users returned: %+v", users)
	}

	t.Run("applies defaults and clamps", func(t *testing.T) {
		repo := &fakeUserRepo{listResult: []domain.User{}}
		svc := newAuthServiceForTests(repo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)

		if _, err := svc.ListUsers(ctx, 0, -5); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(repo.listInputs) != 1 {
			t.Fatalf("expected one call to list, got %d", len(repo.listInputs))
		}
		if repo.listInputs[0].limit != 50 || repo.listInputs[0].offset != 0 {
			t.Fatalf("expected defaults limit=50 offset=0, got limit=%d offset=%d", repo.listInputs[0].limit, repo.listInputs[0].offset)
		}

		repo = &fakeUserRepo{listResult: []domain.User{}}
		svc = newAuthServiceForTests(repo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)
		if _, err := svc.ListUsers(ctx, 500, 1); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.listInputs[0].limit != 200 {
			t.Fatalf("expected limit clamped to 200, got %d", repo.listInputs[0].limit)
		}
	})

	t.Run("propagates repository error", func(t *testing.T) {
		repo := &fakeUserRepo{listErr: errors.New("boom")}
		svc := newAuthServiceForTests(repo, &fakeRoleRepo{}, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)

		if _, err := svc.ListUsers(ctx, 10, 0); err == nil {
			t.Fatalf("expected error from repository")
		}
	})
}

func TestDeleteUser(t *testing.T) {
	ctx := context.Background()
	adminRoleID := uuid.New()
	roleRepo := &fakeRoleRepo{roleResult: &domain.Role{ID: adminRoleID}}

	t.Run("denies when actor missing", func(t *testing.T) {
		svc := newAuthServiceForTests(&fakeUserRepo{}, roleRepo, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)
		err := svc.DeleteUser(ctx, nil, uuid.New())
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
	})

	t.Run("allows self deletion", func(t *testing.T) {
		repo := &fakeUserRepo{}
		svc := newAuthServiceForTests(repo, roleRepo, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)
		userID := uuid.New()
		actor := &domain.User{ID: userID}
		if err := svc.DeleteUser(ctx, actor, userID); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.deleteInput != userID {
			t.Fatalf("expected delete call for %s, got %s", userID, repo.deleteInput)
		}
	})

	t.Run("denies non-admin deleting others", func(t *testing.T) {
		repo := &fakeUserRepo{}
		svc := newAuthServiceForTests(repo, roleRepo, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)
		actor := &domain.User{ID: uuid.New(), Roles: []domain.Role{{ID: uuid.New(), Name: "user", CreatedAt: time.Now(), UpdatedAt: time.Now()}}}
		err := svc.DeleteUser(ctx, actor, uuid.New())
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("expected ErrForbidden, got %v", err)
		}
		if repo.deleteInput != uuid.Nil {
			t.Fatalf("expected repository delete not called, got %s", repo.deleteInput)
		}
	})

	t.Run("admin can delete others", func(t *testing.T) {
		repo := &fakeUserRepo{}
		svc := newAuthServiceForTests(repo, roleRepo, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)
		actor := &domain.User{ID: uuid.New(), Roles: []domain.Role{{ID: adminRoleID, Name: "admin", CreatedAt: time.Now(), UpdatedAt: time.Now()}}}
		target := uuid.New()
		if err := svc.DeleteUser(ctx, actor, target); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if repo.deleteInput != target {
			t.Fatalf("expected delete call for %s, got %s", target, repo.deleteInput)
		}
	})

	t.Run("translates missing user", func(t *testing.T) {
		repo := &fakeUserRepo{deleteErr: sql.ErrNoRows}
		svc := newAuthServiceForTests(repo, roleRepo, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)
		actor := &domain.User{ID: uuid.New(), Roles: []domain.Role{{ID: adminRoleID, Name: "admin", CreatedAt: time.Now(), UpdatedAt: time.Now()}}}
		err := svc.DeleteUser(ctx, actor, uuid.New())
		if !errors.Is(err, ErrUserNotFound) {
			t.Fatalf("expected ErrUserNotFound, got %v", err)
		}
	})

	t.Run("propagates delete error", func(t *testing.T) {
		repo := &fakeUserRepo{deleteErr: errors.New("db down")}
		svc := newAuthServiceForTests(repo, roleRepo, &fakeSessionRepo{}, &fakeStorage{}, nil, nil)
		actor := &domain.User{ID: uuid.New(), Roles: []domain.Role{{ID: adminRoleID, Name: "admin", CreatedAt: time.Now(), UpdatedAt: time.Now()}}}
		err := svc.DeleteUser(ctx, actor, uuid.New())
		if err == nil || !strings.Contains(err.Error(), "db down") {
			t.Fatalf("expected underlying error, got %v", err)
		}
	})
}

func authStringPtr(v string) *string {
	return &v
}
