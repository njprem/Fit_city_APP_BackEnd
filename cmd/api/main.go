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
	"context"
	"io"
	"log"
	"os"
	"strings"
	"time"

	"github.com/elastic/go-elasticsearch/v8"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/config"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/logging"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/media"
	minioRepo "github.com/njprem/Fit_city_APP_BackEnd/internal/repository/minio"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/postgres"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/service"
	httpx "github.com/njprem/Fit_city_APP_BackEnd/internal/transport/http"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/transport/mail"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/util"
)

func main() {
	cfg := config.Load()

	if cfg.LogstashTCPAddr != "" {
		logstashWriter, err := logging.NewLogstashWriter(cfg.LogstashTCPAddr)
		if err != nil {
			log.Fatalf("logstash writer: %v", err)
		}
		log.SetOutput(io.MultiWriter(os.Stderr, logstashWriter))
		defer logstashWriter.Close()
	}
	log.SetFlags(0)

	db, err := postgres.New(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("connect database: %v", err)
	}

	minioClient, err := minioRepo.NewClient(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOUseSSL)
	if err != nil {
		log.Fatalf("minio client: %v", err)
	}
	objectStorage := minioRepo.NewStorage(minioClient, cfg.MinIOPublicURL)
	imageProcessor := media.NewFFMPEGProcessor(cfg.FFMPEGPath, cfg.ImageMaxDimension)

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

	authService := service.NewAuthService(userRepo, roleRepo, sessionRepo, passwordResetRepo, objectStorage, resetMailer, jwtManager, cfg.GoogleAudience, cfg.MinIOBucketProfile, resetTTL, cfg.PasswordResetOTPLength, imageProcessor, cfg.ProfileImageMaxDimension)

	destinationRepo := postgres.NewDestinationRepo(db)
	destinationChangeRepo := postgres.NewDestinationChangeRepo(db)
	destinationVersionRepo := postgres.NewDestinationVersionRepo(db)
	destinationImportRepo := postgres.NewDestinationImportRepo(db)
	reviewRepo := postgres.NewReviewRepo(db)
	reviewMediaRepo := postgres.NewReviewMediaRepo(db)
	favoriteRepo := postgres.NewFavoriteRepo(db)
	viewStatsRepo := postgres.NewDestinationViewStatsRepo(db)

	var esClient *elasticsearch.Client
	if cfg.ElasticsearchBaseURL != "" {
		esConfig := elasticsearch.Config{
			Addresses: []string{cfg.ElasticsearchBaseURL},
		}
		if cfg.ElasticsearchUsername != "" || cfg.ElasticsearchPassword != "" {
			esConfig.Username = cfg.ElasticsearchUsername
			esConfig.Password = cfg.ElasticsearchPassword
		}
		esClient, err = elasticsearch.NewClient(esConfig)
		if err != nil {
			log.Printf("elasticsearch client: %v", err)
		}
	}

	viewStatsTimeout, err := time.ParseDuration(cfg.DestinationViewStatsTimeout)
	if err != nil {
		log.Printf("invalid DEST_VIEW_STATS_TIMEOUT, fallback to 5s: %v", err)
		viewStatsTimeout = 5 * time.Second
	}
	viewStatsCacheTTL, err := time.ParseDuration(cfg.DestinationViewStatsCacheTTL)
	if err != nil {
		log.Printf("invalid DEST_VIEW_STATS_CACHE_TTL, fallback to 10m: %v", err)
		viewStatsCacheTTL = 10 * time.Minute
	}

	viewStatsService := service.NewDestinationViewStatsService(
		viewStatsRepo,
		esClient,
		service.DestinationViewStatsConfig{
			LogIndex:       cfg.ElasticsearchLogIndex,
			CacheTTL:       viewStatsCacheTTL,
			RequestTimeout: viewStatsTimeout,
		},
	)

	destinationPublicBase := cfg.MinIOPublicURL
	if destinationPublicBase != "" && cfg.MinIOBucketProfile != "" {
		destinationPublicBase = strings.Replace(destinationPublicBase, cfg.MinIOBucketProfile, cfg.MinIOBucketDestinations, 1)
	}
	reviewPublicBase := cfg.MinIOPublicURL
	if reviewPublicBase != "" && cfg.MinIOBucketProfile != "" {
		reviewPublicBase = strings.Replace(reviewPublicBase, cfg.MinIOBucketProfile, cfg.MinIOBucketReviews, 1)
	}

	workflowService := service.NewDestinationWorkflowService(
		destinationRepo,
		destinationChangeRepo,
		destinationVersionRepo,
		objectStorage,
		service.DestinationWorkflowConfig{
			Bucket:            cfg.MinIOBucketDestinations,
			PublicBaseURL:     destinationPublicBase,
			ImageMaxBytes:     cfg.DestinationImageMaxBytes,
			ImageMaxDimension: cfg.ImageMaxDimension,
			AllowedCategories: cfg.DestinationAllowedCategories,
			ApprovalRequired:  cfg.DestinationApprovalRequired,
			HardDeleteAllowed: cfg.DestinationHardDeleteAllowed,
			ImageProcessor:    imageProcessor,
		},
	)

	destinationService := service.NewDestinationService(destinationRepo)
	importService := service.NewDestinationImportService(
		destinationImportRepo,
		destinationRepo,
		workflowService,
		objectStorage,
		service.DestinationImportServiceConfig{
			Bucket:        cfg.MinIOBucketDestinations,
			MaxRows:       cfg.DestinationImportMaxRows,
			MaxFileBytes:  cfg.DestinationImportMaxFileBytes,
			MaxPendingIDs: cfg.DestinationImportMaxPendingIDs,
		},
	)
	reviewService := service.NewReviewService(
		reviewRepo,
		reviewMediaRepo,
		destinationRepo,
		objectStorage,
		service.ReviewServiceConfig{
			Bucket:            cfg.MinIOBucketReviews,
			MaxImageBytes:     cfg.DestinationImageMaxBytes,
			ImageProcessor:    imageProcessor,
			ImageMaxDimension: cfg.ImageMaxDimension,
			PublicBaseURL:     reviewPublicBase,
		},
	)
	favoriteService := service.NewFavoriteService(favoriteRepo, destinationRepo)

	router := httpx.NewRouter(cfg.AllowOrigins)
	httpx.RegisterPages(router, cfg.FrontendBaseURL)
	httpx.RegisterAuth(router, authService)
	httpx.RegisterDestinations(router, authService, destinationService, workflowService, httpx.DestinationFeatures{
		View:   cfg.EnableDestinationView,
		Create: cfg.EnableDestinationCreate,
		Update: cfg.EnableDestinationUpdate,
		Delete: cfg.EnableDestinationDelete,
	})
	httpx.RegisterDestinationImports(router, authService, importService, cfg.EnableDestinationBulkImport, cfg.DestinationImportMaxFileBytes)
	httpx.RegisterReviews(router, authService, reviewService)
	httpx.RegisterFavorites(router, authService, favoriteService)
	httpx.RegisterDestinationStats(router, authService, destinationService, viewStatsService)
	httpx.RegisterSwagger(router)

	if cfg.EnableDestinationViewStatsRollup {
		rollupInterval, err := time.ParseDuration(cfg.DestinationViewStatsRollupInterval)
		if err != nil || rollupInterval <= 0 {
			log.Printf("invalid DEST_VIEW_STATS_ROLLUP_INTERVAL, fallback to 1h: %v", err)
			rollupInterval = time.Hour
		}
		go viewStatsService.RunRollup(context.Background(), rollupInterval)
	}

	router.Logger.Fatal(router.Start(":" + cfg.Port))
}
