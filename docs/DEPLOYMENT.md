# Deployment Guide

This document provides instructions for deploying the Fit City application using Docker and Docker Compose.

## 1. Overview

The application is composed of three main services orchestrated by Docker Compose:

- `api`: The main Go application backend.
- `db`: A PostgreSQL database for data persistence.
- `minio`: An S3-compatible object storage server for file uploads.

## 2. Prerequisites

Before deploying, ensure you have the following software installed on your server:

- [Docker](https://docs.docker.com/engine/install/)
- [Docker Compose](https://docs.docker.com/compose/install/)

#### Optional Components:

**ELK Stack:** The Elasticsearch, Logstash, and Kibana (ELK) stack is optional. Without it, the API will not be able to log usage statistics, and the destination view statistics feature will not function.
**SMTP Mailer:** The SMTP mailer configuration is optional. Without it, features requiring email (e.g., password reset with OTP) will not work.

## 3. Configuration

The application uses environment files to configure the services. These files are not checked into version control and must be created manually on the deployment server.

1. Create the configuration directory:

   ```bash
   mkdir -p ~/envs/fit-city/
   ```
2. Create the environment files with the required variables:

   - `~/envs/fit-city/db.env`:

     ```env
     POSTGRES_DB=your_db_name
     POSTGRES_USER=your_db_user
     POSTGRES_PASSWORD=your_db_password
     ```
   - `~/envs/fit-city/minio.env`:

     ```env
     MINIO_ROOT_USER=your_minio_user
     MINIO_ROOT_PASSWORD=your_minio_password
     ```
   - `~/envs/fit-city/api.prod.env`: This file contains the configuration for the API service itself. You can find a template for the required variables in the `.env` file in the root of the repository. At a minimum, you will need to define:

# Core Application Settings

PORT=8181

# Database Configuration

```
DATABASE_URL=postgres://db:5432/fitcity?sslmode=disable # Ensure 'db' matches your docker-compose service name
DB_USER=your_db_user # Defined in ~/envs/fit-city/db.env
DB_PASSWORD=your_db_password # Defined in ~/envs/fit-city/db.env
DB_NAME=your_db_name # Defined in ~/envs/fit-city/db.env
DB_SSL_MODE=disable # Use 'require' or 'verify-full' for production with SSL
```

# JWT Configuration

```
JWT_SECRET=your_jwt_secret # IMPORTANT: Change this to a strong, random secret
JWT_TTL_MINUTES=1440 # Token time-to-live in minutes (24 hours)
```

# Session Management

```
SESSION_TTL=24h # Session duration in hours
```

# Frontend and CORS

```
FRONTEND_BASE_URL=https://fitcity.example.com # Update with your frontend URL
ALLOW_ORIGINS=* # Restrict to your frontend URL in production for security
```

# MinIO Object Storage Configuration

```
MINIO_ENDPOINT=minio:9000 # Ensure 'minio' matches your docker-compose service name
MINIO_ACCESS_KEY=your_minio_user # Defined in ~/envs/fit-city/minio.env
MINIO_SECRET_KEY=your_minio_password # Defined in ~/envs/fit-city/minio.env
MINIO_USE_SSL=false # Use 'true' for production with SSL/TLS
MINIO_BUCKET_PROFILE=fitcity-profiles
MINIO_BUCKET_DESTINATIONS=fitcity-destinations
MINIO_BUCKET_REVIEWS=fitcity-reviews
MINIO_PUBLIC_URL=http://your_server_ip:9000/fitcity-profiles # IMPORTANT: Update with your server's public IP or hostname and MinIO port
```

# Google Authentication

```
GOOGLE_AUDIENCE=your_google_client_id.apps.googleusercontent.com # Your Google OAuth client ID
```

# Elasticsearch / ELK Stack Configuration (Optional)

```
ELASTICSEARCH_BASE_URL=http://elasticsearch:9200 # Update with your Elasticsearch IP if not using Docker DNS
ELASTICSEARCH_LOG_INDEX=app-logs-*
ELASTICSEARCH_USERNAME=app_search # IMPORTANT: Update username for production
ELASTICSEARCH_PASSWORD=AppSearch!2025 # IMPORTANT: Change this password for production
```

# SMTP Email Configuration (Optional)

```
SMTP_HOST=smtp.example.com # Update with your SMTP host
SMTP_PORT=587 # Update with your SMTP port (commonly 587 for TLS)
SMTP_USERNAME=smtpuser # Update with your SMTP username
SMTP_PASSWORD=smtppass # IMPORTANT: Update with your SMTP password for production
SMTP_FROM="FitCity <no-reply@example.com>" # Update with your sender email address
SMTP_USE_TLS=true # Use 'true' for production SMTP with TLS
```

# Password Reset Feature

```
PASSWORD_RESET_TTL=15m # OTP expiration time
PASSWORD_RESET_OTP_LENGTH=6 # Numeric OTP length
```

# Image Processing

```
DESTINATION_IMAGE_MAX_BYTES=16777216 # Max destination image size in bytes (e.g., 16MB)
IMAGE_MAX_DIMENSION=960 # Max dimension for general images
PROFILE_IMAGE_MAX_DIMENSION=240 # Max dimension for profile images
FFMPEG_PATH=/usr/bin/ffmpeg # Path to FFmpeg executable (if used for media processing)
```

# Destination Management

```
DESTINATION_ALLOWED_CATEGORIES=Nature,Culture,Sport,Food # Comma-separated list of allowed categories
DESTINATION_HARD_DELETE_ALLOWED=false # Set to 'true' to allow hard deletion of destinations
```

# Feature Flags (set to 'true' or 'false')

```
ENABLE_DESTINATION_VIEW=true
ENABLE_DESTINATION_CREATE=true
ENABLE_DESTINATION_UPDATE=true
ENABLE_DESTINATION_DELETE=true
ENABLE_DESTINATION_BULK_IMPORT=true
```

# Destination Bulk Import Configuration

```
DESTINATION_IMPORT_MAX_ROWS=500
DESTINATION_IMPORT_MAX_FILE_BYTES=5242880 # Max CSV file size in bytes (e.g., 5MB)
DESTINATION_IMPORT_MAX_PENDING_IDS=25
```

# Destination View Stats Configuration

```
DEST_VIEW_STATS_TIMEOUT=5s
DEST_VIEW_STATS_CACHE_TTL=10m
DEST_VIEW_STATS_ROLLUP_INTERVAL=1h
DEST_VIEW_STATS_MAX_RANGE=720h # Max time range for stats (e.g., 30 days)
DEST_VIEW_STATS_ROLLUP_ENABLED=true # Set to 'true' to enable background stats rollup
```

    **Note:** Replace `your_*` placeholders with your actual secrets and configuration values.

## 4. Deployment

1. Clone the repository to your deployment server:

2. Build and start the services in detached mode using the production compose file:

   ```bash
   docker-compose -f infra/compose.prod.yml up --build -d
   ```

   - `--build`: This flag forces Docker Compose to build the `api` image from the Dockerfile.
   - `-d`: This flag runs the containers in the background.
3. Verify that the services are running:

   ```bash
   docker-compose -f infra/compose.prod.yml ps
   ```

   You should see the `api`, `db`, and `minio` services with a status of `Up`.

## 5. Services

### API Service (`api`)

- **Dockerfile:** `infra/docker/api.Dockerfile`
- **Host Port:** `8181`
- The Go application is compiled in a multi-stage build for a small final image size.
- It depends on the `db` and `minio` services, so they will be started first.

### Database Service (`db`)

- **Image:** `postgres:16-alpine`
- **Host Port:** `5432`
- **Volume:** `pgdata` is used to persist the PostgreSQL data, so your database will be preserved across container restarts.

### Object Storage (`minio`)

- **Image:** `minio/minio`
- **Host Ports:**
  - `9000`: The MinIO API.
  - `9001`: The MinIO web console.
- **Volume:** `minio` is used to persist uploaded files.

## 6. Stopping the Application

To stop the services, run:

```bash
docker-compose -f infra/compose.prod.yml down
```

If you also want to remove the data volumes (`pgdata` and `minio`), use the `-v` flag:
**Warning:** This will permanently delete your database and all uploaded files.

```bash
docker-compose -f infra/compose.prod.yml down -v
```
