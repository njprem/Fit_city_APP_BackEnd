#!/usr/bin/env bash

# Run schema-driven API tests against the HTTPS endpoint using the Swagger spec.
# The script installs schemathesis into a local venv (scripts/.venv/swagger-tests)
# and drives requests with data generated from docs/swagger.yaml.

set -euo pipefail

SCHEMA_PATH=${SCHEMA_PATH:-docs/swagger.yaml}
API_HOST=${API_HOST:-https://localhost:8080}
BASE_PATH=${BASE_PATH:-/api/v1}
BASE_URL=${BASE_URL:-${API_HOST%/}${BASE_PATH}}
VENV_DIR=${VENV_DIR:-scripts/.venv/swagger-tests}
WORKERS=${WORKERS:-4}
MAX_EXAMPLES=${MAX_EXAMPLES:-3}
REQUEST_TIMEOUT=${REQUEST_TIMEOUT:-8}
VERIFY_TLS=${VERIFY_TLS:-1}
ADMIN_TOKEN=${ADMIN_TOKEN:-}
USER_TOKEN=${USER_TOKEN:-}
RUN_UNAUTHORIZED=${RUN_UNAUTHORIZED:-0}
SCHEMATHESIS_ARGS=${SCHEMATHESIS_ARGS:-}
HYPOTHESIS_SEED=${HYPOTHESIS_SEED:-}

# Newer release to support Python 3.13 in this environment
SCHEMATHESIS_VERSION="4.6.1"

err() {
  echo "[swagger-tests] $*" >&2
}

require_cmd() {
  local name=$1
  if ! command -v "$name" >/dev/null 2>&1; then
    err "Missing required command: $name"
    exit 1
  fi
}

ensure_schema() {
  if [[ ! -f "$SCHEMA_PATH" ]]; then
    err "Swagger schema not found at $SCHEMA_PATH"
    exit 1
  fi
}

ensure_venv() {
  require_cmd python3
  if [[ ! -x "$VENV_DIR/bin/python" ]]; then
    err "Creating virtualenv at $VENV_DIR"
    python3 -m venv "$VENV_DIR"
  fi
  local py="$VENV_DIR/bin/python"
  "$py" -m pip -q install --upgrade pip
  if ! "$py" - <<PY >/dev/null 2>&1
import importlib, sys
try:
    importlib.import_module("schemathesis")
except ImportError:
    sys.exit(1)
PY
  then
    err "Installing schemathesis==$SCHEMATHESIS_VERSION"
    "$py" -m pip -q install "schemathesis==${SCHEMATHESIS_VERSION}"
  fi
}

build_common_args() {
  local -n out=$1
  out=(
    "$SCHEMA_PATH"
    --url "$BASE_URL"
    --checks all
    --phases examples,fuzzing,stateful
    --workers "$WORKERS"
    --request-timeout "$REQUEST_TIMEOUT"
    --max-examples "$MAX_EXAMPLES"
  )
  if [[ "$VERIFY_TLS" == "0" ]]; then
    out+=(--tls-verify false)
  fi
  if [[ -n "$HYPOTHESIS_SEED" ]]; then
    out+=(--seed "$HYPOTHESIS_SEED")
  fi
}

run_suite() {
  local label=$1
  shift
  local -a args=()
  build_common_args args
  if [[ -n "$SCHEMATHESIS_ARGS" ]]; then
    # shellcheck disable=SC2206
    extra_args=($SCHEMATHESIS_ARGS)
    args+=("${extra_args[@]}")
  fi
  args+=("$@")

  err "Running $label suite against $BASE_URL"
  "$VENV_DIR/bin/schemathesis" run "${args[@]}"
}

main() {
  ensure_schema
  ensure_venv

  local auth_header=""
  if [[ -n "$ADMIN_TOKEN" ]]; then
    auth_header="Authorization: Bearer $ADMIN_TOKEN"
  elif [[ -n "$USER_TOKEN" ]]; then
    auth_header="Authorization: Bearer $USER_TOKEN"
  fi

  if [[ -n "$auth_header" ]]; then
    run_suite "authorized" --header "$auth_header"
  else
    err "No ADMIN_TOKEN or USER_TOKEN set; running without Authorization header."
    run_suite "public"
  fi

  if [[ "$RUN_UNAUTHORIZED" -eq 1 && -n "$auth_header" ]]; then
    err "Running unauthorized pass without Authorization header (expects 401/403 where applicable)."
    run_suite "unauthorized"
  fi
}

main "$@"
