# Fit City Backend 🌆

Production-ready Go API for the Fit City platform — authentication, destinations, reviews, favorites, and analytics in one service.

## ✨ Highlights
- 🔐 Auth & sessions (email/password + Google ID token)
- 🗺️ Destination workflow with admin approvals and bulk CSV imports
- ⭐ Reviews, ratings, and favorites with media uploads
- 📈 View analytics via Elasticsearch with optional rollups
- 🧰 Swagger docs, health checks, and structured logging

## 🧱 Tech Stack
- **Go 1.24** with Echo HTTP framework
- **Postgres 16** as the system of record
- **MinIO / S3** for media storage
- **FFmpeg** for image processing
- Optional: **Elasticsearch + Logstash** and **SMTP** for password reset emails

## 🚀 Quick Start
1. Create a local environment file:
   ```bash
   cp internal/config/env.example .env
   ```
2. Start dependencies (Postgres + MinIO) and ensure the buckets exist.
3. Run the API:
   ```bash
   go run ./cmd/api
   ```
4. Open Swagger UI:
   ```
   http://localhost:8080/swagger/index.html
   ```

## 🐳 Docker (Local)
```bash
docker compose -f infra/compose.local.yml up --build
```
Notes:
- Compose references env files under `infra/env/` (not committed). Use `internal/config/env.example` as a starting point.
- API container ships with FFmpeg and serves Swagger from `/swagger`.

## ⚙️ Configuration
Environment variables are loaded from `.env` (via `godotenv`) and the process environment.

**Required**
- `DATABASE_URL` — Postgres connection string
- `JWT_SECRET` — HMAC key for JWT signing
- `MINIO_ENDPOINT` — `host:port` of MinIO/S3
- `MINIO_ACCESS_KEY` / `MINIO_SECRET_KEY`
- `MINIO_BUCKET_PROFILE`
- `MINIO_BUCKET_DESTINATIONS`

**Common / Optional**
- `MINIO_BUCKET_REVIEWS` (default `fitcity-reviews`)
- `MINIO_PUBLIC_URL` (base URL used to serve public media)
- `GOOGLE_AUDIENCE` (Google ID token audience)
- `ALLOW_ORIGINS` (CORS; `*` disables credentials)
- `SESSION_TTL`, `PASSWORD_RESET_TTL`, `PASSWORD_RESET_OTP_LENGTH`
- `FFMPEG_PATH`, `IMAGE_MAX_DIMENSION`, `PROFILE_IMAGE_MAX_DIMENSION`
- `DESTINATION_IMAGE_MAX_BYTES`, `DESTINATION_ALLOWED_CATEGORIES`
- `ENABLE_DESTINATION_*` feature flags, `DESTINATION_APPROVAL_REQUIRED`, `DESTINATION_HARD_DELETE_ALLOWED`
- `ENABLE_DESTINATION_BULK_IMPORT`, `DESTINATION_IMPORT_*`
- `ELASTICSEARCH_*`, `LOGSTASH_TCP_ADDR`, `DEST_VIEW_STATS_*`
- `SMTP_*` (enables password-reset email)

See `internal/config/env.example` for a full template.

## 📚 API Docs
- Swagger UI: `http://localhost:8080/swagger/index.html`
- JSON spec: `http://localhost:8080/swagger/doc.json`
- Source spec: `docs/swagger.yaml`
- Health check: `http://localhost:8080/health`

## 🧪 Testing
Run unit tests:
```bash
go test ./...
```

Swagger smoke tests (schemathesis):
```bash
ADMIN_TOKEN="bearer-token-with-admin-scope" \
API_HOST="https://fit-city.kaminjitt.com" \
BASE_PATH="/api/v1" \
scripts/testing/run_swagger_tests.sh
```

## 📁 Project Structure
- `cmd/api` — service entrypoint
- `internal/` — config, services, repositories, transports
- `migrations/` — SQL migrations
- `docs/` — architecture + API design docs
- `infra/` — Dockerfiles + compose stacks
- `scripts/` — utilities, curl scenarios, k6 load tests

## 🧰 Handy Scripts
- `scripts/setup_minio_buckets.sh` — create buckets and set public access
- `scripts/k6/*.js` — load testing scenarios
- `scripts/curl/destination_workflow_tests.sh` — admin workflow smoke tests
