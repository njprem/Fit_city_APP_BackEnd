package config

import (
	"os"
)

type Config struct {
	Port           string
	DatabaseURL    string
	JWTSecret      string
	GoogleAudience string
	AllowOrigins   []string

	MinIOEndpoint  string
	MinIOAccessKey string
	MinIOSecretKey string
	MinIOUseSSL    bool
}

func Load() Config {
	return Config{
		Port:           getenv("PORT", "8080"),
		DatabaseURL:    must("DATABASE_URL"),
		JWTSecret:      must("JWT_SECRET"),
		GoogleAudience: must("GOOGLE_AUD"),

		MinIOEndpoint:  must("MINIO_ENDPOINT"),
		MinIOAccessKey: must("MINIO_ACCESS_KEY"),
		MinIOSecretKey: must("MINIO_SECRET_KEY"),
		MinIOUseSSL:    getenv("MINIO_USE_SSL", "false") == "true",

		AllowOrigins:   []string{getenv("ALLOW_ORIGINS", "*")},
	}
}
func getenv(k, d string) string { if v := os.Getenv(k); v != "" { return v }; return d }
func must(k string) string { v := os.Getenv(k); if v=="" { panic("missing env: "+k) }; return v }