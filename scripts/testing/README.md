# Swagger HTTPS Tests

Smoke-test every documented API route over HTTPS using the Swagger spec and [schemathesis](https://schemathesis.readthedocs.io/). The runner generates requests from the schema examples and validates responses.

## Usage

```bash
# Default host is https://localhost:8080 with base path /api/v1
ADMIN_TOKEN="bearer-token-with-admin-scope" \
USER_TOKEN="optional-user-token" \
API_HOST="https://fit-city.kaminjitt.com" \
BASE_PATH="/api/v1" \
scripts/testing/run_swagger_tests.sh
```

Key options (env vars):
- `SCHEMA_PATH` (default `docs/swagger.yaml`)
- `API_HOST` / `BASE_PATH` (builds `BASE_URL`)
- `ADMIN_TOKEN` or `USER_TOKEN` (Authorization header; admin is preferred)
- `RUN_UNAUTHORIZED=1` also runs a pass without the Authorization header to confirm 401/403 handling
- `MAX_EXAMPLES` (per endpoint examples, default 3)
- `WORKERS` (concurrent workers, default 4)
- `VERIFY_TLS=0` to ignore self-signed certs
- `SCHEMATHESIS_ARGS` to forward extra flags (e.g. `--endpoint-regex 'destinations.*'`)

The script installs schemathesis into `scripts/.venv/swagger-tests` on first run and reuses it afterward.
