#!/usr/bin/env bash

set -euo pipefail

# Downloads the MinIO client if not present and ensures required buckets exist.
# Buckets are taken from MINIO_BUCKET_PROFILE and MINIO_BUCKET_DESTINATIONS env vars.
# Public read ("download") access is applied to each bucket.

MINIO_ALIAS=${MINIO_ALIAS:-fitcity}
MINIO_ENDPOINT=${MINIO_ENDPOINT:-}
MINIO_ACCESS_KEY=${MINIO_ACCESS_KEY:-}
MINIO_SECRET_KEY=${MINIO_SECRET_KEY:-}
MINIO_USE_SSL=${MINIO_USE_SSL:-false}
MINIO_BUCKET_PROFILE=${MINIO_BUCKET_PROFILE:-}
MINIO_BUCKET_DESTINATIONS=${MINIO_BUCKET_DESTINATIONS:-}
MC_BINARY=${MC_BINARY:-scripts/bin/mc}
MC_URL=${MC_URL:-https://dl.min.io/client/mc/release/linux-amd64/mc}

usage() {
  cat <<'EOF'
Usage: scripts/setup_minio_buckets.sh

Required environment variables:
  MINIO_ENDPOINT             MinIO endpoint in host:port form (e.g. localhost:9000)
  MINIO_ACCESS_KEY           MinIO access key
  MINIO_SECRET_KEY           MinIO secret key
  MINIO_BUCKET_PROFILE       Name of the profile image bucket to ensure
  MINIO_BUCKET_DESTINATIONS  Name of the destinations bucket to ensure

Optional environment variables:
  MINIO_ALIAS                Alias name used with the mc CLI (default: fitcity)
  MINIO_USE_SSL              Set to "true" to use HTTPS for the endpoint (default: false)
  MC_BINARY                  Path to the mc binary (default: scripts/bin/mc)
  MC_URL                     Download URL for mc if not already present

The script will create the buckets if they are missing and configure anonymous
download access so public URLs can retrieve objects.
EOF
}

if [[ "${1:-}" == "--help" || "${1:-}" == "-h" ]]; then
  usage
  exit 0
fi

abort() {
  echo "Error: $*" >&2
  exit 1
}

ensure_env() {
  [[ -n "$MINIO_ENDPOINT" ]] || abort "MINIO_ENDPOINT is required"
  [[ -n "$MINIO_ACCESS_KEY" ]] || abort "MINIO_ACCESS_KEY is required"
  [[ -n "$MINIO_SECRET_KEY" ]] || abort "MINIO_SECRET_KEY is required"
  [[ -n "$MINIO_BUCKET_PROFILE" ]] || abort "MINIO_BUCKET_PROFILE is required"
  [[ -n "$MINIO_BUCKET_DESTINATIONS" ]] || abort "MINIO_BUCKET_DESTINATIONS is required"
}

ensure_mc() {
  if command -v mc >/dev/null 2>&1; then
    MC=$(command -v mc)
    return
  fi
  MC=$MC_BINARY
  if [[ ! -x "$MC" ]]; then
    mkdir -p "$(dirname "$MC")"
    echo "Downloading MinIO client to $MC"
    curl -sSL "$MC_URL" -o "$MC"
    chmod +x "$MC"
  fi
}

ensure_alias() {
  local scheme="http://"
  if [[ "$MINIO_USE_SSL" == "true" ]]; then
    scheme="https://"
  fi
  local endpoint="${MINIO_ENDPOINT#http://}"
  endpoint="${endpoint#https://}"
  local url="${scheme}${endpoint}"
  "$MC" alias set "$MINIO_ALIAS" "$url" "$MINIO_ACCESS_KEY" "$MINIO_SECRET_KEY" >/dev/null
}

ensure_bucket() {
  local bucket=$1
  if ! "$MC" ls "${MINIO_ALIAS}/${bucket}" >/dev/null 2>&1; then
    echo "Creating bucket ${MINIO_ALIAS}/${bucket}"
    "$MC" mb "${MINIO_ALIAS}/${bucket}"
  else
    echo "Bucket ${bucket} already exists"
  fi
  echo "Setting anonymous download policy on ${bucket}"
  "$MC" anonymous set download "${MINIO_ALIAS}/${bucket}" >/dev/null
}

ensure_env
ensure_mc
ensure_alias

ensure_bucket "$MINIO_BUCKET_PROFILE"
ensure_bucket "$MINIO_BUCKET_DESTINATIONS"

echo "Buckets ready."
