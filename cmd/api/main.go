package main

import (
	"log"
	"os"

	"FitCity-API/internal/config"
	"FitCity-API/internal/repository/postgres"
	"FitCity-API/internal/repository/minio"
	"FitCity-API/internal/service"
	httpx "FitCity-API/internal/transport/http"
)

func main() {
	cfg := config.Load() // read env

	// DB
	db, err := postgres.New(cfg.DatabaseURL)
	if err != nil { log.Fatal(err) }

	// Repos
	userRepo := postgres.NewUserRepo(db)
	destRepo := postgres.NewDestinationRepo(db)
	reviewRepo := postgres.NewReviewRepo(db)
	favRepo := postgres.NewFavoriteRepo(db)

	// Object storage
	minioCli, err := minio.NewClient(cfg.MinIOEndpoint, cfg.MinIOAccessKey, cfg.MinIOSecretKey, cfg.MinIOUseSSL)
	if err != nil { log.Fatal(err) }
	objStore := minio.NewStorage(minioCli)

	// Services
	authSvc := service.NewAuthService(userRepo, cfg.JWTSecret, cfg.GoogleAudience)
	destSvc := service.NewDestinationService(destRepo, objStore)
	reviewSvc := service.NewReviewService(reviewRepo, destRepo)
	favSvc := service.NewFavoriteService(favRepo, destRepo)

	// HTTP
	e := httpx.NewRouter(cfg.AllowOrigins)
	httpx.RegisterAuth(e, authSvc)
	httpx.RegisterDestinations(e, destSvc)
	httpx.RegisterReviews(e, reviewSvc)
	httpx.RegisterFavorites(e, favSvc)

	e.Logger.Fatal(e.Start(":" + cfg.Port))
	_ = os.Setenv("READY", "1")
}