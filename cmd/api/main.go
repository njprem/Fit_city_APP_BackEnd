// @title           Fit City API
// @version         1.0
// @description     Authentication and profile services for the Fit City platform.
// @BasePath        /api/v1
// @schemes         http https
// @securityDefinitions.apikey BearerAuth
// @in              header
// @name            Authorization

package main

import (
	"log"
	"time"

	"github.com/njprem/Fit_city_APP_BackEnd/docs"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/config"
	minioRepo "github.com/njprem/Fit_city_APP_BackEnd/internal/repository/minio"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/postgres"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/service"
	httpx "github.com/njprem/Fit_city_APP_BackEnd/internal/transport/http"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/transport/mail"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/util"
)

func main() {
	cfg := config.Load()

	docs.SwaggerInfo.Title = "Fit City API"
	docs.SwaggerInfo.Description = "API documentation for Fit City backend services."
	docs.SwaggerInfo.Version = "1.0"
	docs.SwaggerInfo.BasePath = "/api/v1"

	db, err := postgres.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	minioClient, err := minioRepo.NewClient(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOUseSSL)
	if err != nil {
		log.Fatalf("minio client: %v", err)
	}
	objectStorage := minioRepo.NewStorage(minioClient, cfg.MinIOPublicURL)

	sessionTTL, err := time.ParseDuration(cfg.SessionTTL)
	if err != nil {
		log.Printf("invalid SESSION_TTL, fallback to 24h: %v", err)
		sessionTTL = 24 * time.Hour
	}

	jwtManager := util.NewJWTManager(cfg.JWTSecret, sessionTTL)

	userRepo := postgres.NewUserRepo(db)
	roleRepo := postgres.NewRoleRepo(db)
	sessionRepo := postgres.NewSessionRepo(db)
	passwordResetRepo := postgres.NewPasswordResetRepo(db)

	resetTTL, err := time.ParseDuration(cfg.PasswordResetTTL)
	if err != nil {
		log.Printf("invalid PASSWORD_RESET_TTL, fallback to 15m: %v", err)
		resetTTL = 15 * time.Minute
	}

	var resetMailer service.PasswordResetSender
	if cfg.SMTPHost != "" && cfg.SMTPPort != "" && cfg.SMTPFrom != "" {
		resetMailer = mail.NewPasswordResetMailer(cfg.SMTPHost, cfg.SMTPPort, cfg.SMTPUsername, cfg.SMTPPassword, cfg.SMTPFrom, cfg.SMTPUseTLS)
	}

	authService := service.NewAuthService(userRepo, roleRepo, sessionRepo, passwordResetRepo, objectStorage, resetMailer, jwtManager, cfg.GoogleAudience, cfg.MinIOBucket, resetTTL, cfg.PasswordResetOTPLength)

	router := httpx.NewRouter(cfg.AllowOrigins)
	httpx.RegisterPages(router, cfg.FrontendBaseURL)
	httpx.RegisterAuth(router, authService)
	httpx.RegisterSwagger(router)

	router.Logger.Fatal(router.Start(":" + cfg.Port))
}
