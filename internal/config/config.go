package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                         string
	DatabaseURL                  string
	JWTSecret                    string
	GoogleAudience               string
	AllowOrigins                 []string
	LogstashTCPAddr              string
	MinIOEndpoint                string
	MinIOAccessKey               string
	MinIOSecretKey               string
	MinIOUseSSL                  bool
	MinIOBucketProfile           string
	MinIOBucketDestinations      string
	MinIOBucketReviews           string
	MinIOPublicURL               string
	SessionTTL                   string
	FrontendBaseURL              string
	SMTPHost                     string
	SMTPPort                     string
	SMTPUsername                 string
	SMTPPassword                 string
	SMTPFrom                     string
	SMTPUseTLS                   bool
	PasswordResetTTL             string
	PasswordResetOTPLength       int
	DestinationImageMaxBytes     int64
	DestinationAllowedCategories []string
	EnableDestinationView        bool
	EnableDestinationCreate      bool
	EnableDestinationUpdate      bool
	EnableDestinationDelete      bool
	DestinationHardDeleteAllowed bool
	DestinationApprovalRequired  bool
}

func Load() Config {
	if err := godotenv.Load(); err != nil {
		log.Printf("Warning: .env file not found: %v", err)
	}

	otpLen := 6
	if v, err := strconv.Atoi(getenv("PASSWORD_RESET_OTP_LENGTH", "6")); err == nil && v > 0 {
		otpLen = v
	}

	imageMax := int64(5 * 1024 * 1024)
	if v, err := strconv.ParseInt(getenv("DESTINATION_IMAGE_MAX_BYTES", "5242880"), 10, 64); err == nil && v > 0 {
		imageMax = v
	}

	rawCategories := getenv("DESTINATION_ALLOWED_CATEGORIES", "")
	var allowedCategories []string
	if strings.TrimSpace(rawCategories) != "" {
		allowedCategories = splitAndTrim(rawCategories)
	}

	return Config{
		Port:                         getenv("PORT", "8080"),
		DatabaseURL:                  must("DATABASE_URL"),
		JWTSecret:                    must("JWT_SECRET"),
		GoogleAudience:               getenv("GOOGLE_AUDIENCE", ""),
		LogstashTCPAddr:              getenv("LOGSTASH_TCP_ADDR", ""),
		MinIOEndpoint:                must("MINIO_ENDPOINT"),
		MinIOAccessKey:               must("MINIO_ACCESS_KEY"),
		MinIOSecretKey:               must("MINIO_SECRET_KEY"),
		MinIOUseSSL:                  getenv("MINIO_USE_SSL", "false") == "true",
		MinIOBucketProfile:           must("MINIO_BUCKET_PROFILE"),
		MinIOBucketDestinations:      must("MINIO_BUCKET_DESTINATIONS"),
		MinIOBucketReviews:           getenv("MINIO_BUCKET_REVIEWS", "fitcity-reviews"),
		MinIOPublicURL:               getenv("MINIO_PUBLIC_URL", ""),
		SessionTTL:                   getenv("SESSION_TTL", "24h"),
		FrontendBaseURL:              getenv("FRONTEND_BASE_URL", ""),
		AllowOrigins:                 splitAndTrim(getenv("ALLOW_ORIGINS", "*")),
		SMTPHost:                     getenv("SMTP_HOST", ""),
		SMTPPort:                     getenv("SMTP_PORT", ""),
		SMTPUsername:                 getenv("SMTP_USERNAME", ""),
		SMTPPassword:                 getenv("SMTP_PASSWORD", ""),
		SMTPFrom:                     getenv("SMTP_FROM", ""),
		SMTPUseTLS:                   getenv("SMTP_USE_TLS", "false") == "true",
		PasswordResetTTL:             getenv("PASSWORD_RESET_TTL", "15m"),
		PasswordResetOTPLength:       otpLen,
		DestinationImageMaxBytes:     imageMax,
		DestinationAllowedCategories: allowedCategories,
		EnableDestinationView:        getenv("ENABLE_DESTINATION_VIEW", "true") == "true",
		EnableDestinationCreate:      getenv("ENABLE_DESTINATION_CREATE", "true") == "true",
		EnableDestinationUpdate:      getenv("ENABLE_DESTINATION_UPDATE", "true") == "true",
		EnableDestinationDelete:      getenv("ENABLE_DESTINATION_DELETE", "true") == "true",
		DestinationHardDeleteAllowed: getenv("DESTINATION_HARD_DELETE_ALLOWED", "false") == "true",
		DestinationApprovalRequired:  getenv("DESTINATION_APPROVAL_REQUIRED", "true") == "true",
	}
}

func splitAndTrim(input string) []string {
	parts := strings.Split(input, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		trimmed := strings.TrimSpace(p)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return []string{"*"}
	}
	return out
}

func getenv(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func must(k string) string {
	v := os.Getenv(k)
	if v == "" {
		panic("missing env: " + k)
	}
	return v
}
