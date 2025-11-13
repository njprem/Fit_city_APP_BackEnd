# Destination Bulk Import – System Integration Test Plan

## 1. Scope & Assumptions
- Covers end-to-end CSV ingestion for create-only destination imports from admin upload through change request creation and submission.
- Execution environment mirrors SIT: shared Postgres, MinIO/object storage, background worker, and API gateway behind the `RequireAdmin` guard.
- Hero/gallery assets referenced in CSV already exist on a reachable CDN; media uploads are out of scope.
- All tests use the `ENABLE_DESTINATION_BULK_IMPORT` feature flag enabled and approval workflow requiring dual admins.

## 2. Test Data Foundation
| Dataset | Description | Location |
| --- | --- | --- |
| `valid_create.csv` | 3 well-formed destinations with unique slugs, full metadata, <=3 gallery items. | `/tmp/bulk-import/valid_create.csv` |
| `dry_run_only.csv` | Same as `valid_create.csv` but used with `dry_run=true`. | `/tmp/bulk-import/dry_run_only.csv` |
| `validation_errors.csv` | Rows deliberately missing required fields (name, hero image, coordinates). | `/tmp/bulk-import/validation_errors.csv` |
| `duplicate_slug.csv` | Two rows sharing a slug already present in prod + duplicate within file. | `/tmp/bulk-import/duplicate_slug.csv` |
| `bad_headers.csv` | Header row missing `hero_image_url`. | `/tmp/bulk-import/bad_headers.csv` |

## 3. Test Cases

### SIT-001 – Successful Import Produces Pending Review Changes
- **Goal**: Verify a valid CSV creates change requests and auto-submits them.
- **Preconditions/Test Data**: `valid_create.csv`; Admin A (`author_admin`) authenticated; MinIO bucket writable.
- **Steps**:
  1. `POST /api/v1/admin/destination-imports` multipart upload with `file=valid_create.csv`.
  2. Poll `GET /api/v1/admin/destination-imports/{job_id}` until `status=completed`.
  3. Call `GET /api/v1/admin/destination-changes?submitted_by=author_admin`.
- **Expected Result**:
  - Job reports `total_rows=3`, `changes_created=3`, `rows_failed=0`.
  - Each new change request is in `pending_review`, references the CSV payload, and contains `submitted_at` matching job time.

### SIT-002 – Dry Run Does Not Create Change Requests
- **Goal**: Ensure `dry_run=true` validates without persistence.
- **Preconditions/Test Data**: `dry_run_only.csv`; Admin A authenticated.
- **Steps**:
  1. `POST /api/v1/admin/destination-imports?dry_run=true` with the dataset.
  2. After completion, list destination changes for Admin A.
  3. Query `destination_import_jobs` for the job counters.
- **Expected Result**:
  - Job shows `dry_run=true`, `changes_created=0`, `rows_failed=0`.
  - No new change requests exist in the repository.
  - Error CSV key is empty.

### SIT-003 – Validation Error Captured Per Row
- **Goal**: Confirm missing required fields fail row-level validation.
- **Preconditions/Test Data**: `validation_errors.csv` containing rows missing `name`, `hero_image_url`, and coordinates.
- **Steps**:
  1. Upload dataset via `POST /api/v1/admin/destination-imports`.
  2. Poll job status.
  3. Download `/api/v1/admin/destination-imports/{job_id}/errors`.
- **Expected Result**:
  - Job shows `rows_failed` matching bad rows and `changes_created` equals good rows (if any).
  - Error CSV lists each row number with concatenated messages (e.g., `name is required; hero image required`).
  - No change requests created for invalid rows.

### SIT-004 – Header Validation Blocks Missing Columns
- **Goal**: Ensure missing mandatory columns reject the entire file up front.
- **Preconditions/Test Data**: `bad_headers.csv` lacking `hero_image_url`.
- **Steps**:
  1. Upload dataset.
  2. Observe immediate `422` response.
- **Expected Result**:
  - API returns `422 invalid_headers` with details on missing columns.
  - No job record is created or job shows `status=failed` with `processed_rows=0`.

### SIT-005 – Duplicate Slug Detection
- **Goal**: Validate duplicates inside the file and vs. existing destinations.
- **Preconditions/Test Data**: `duplicate_slug.csv`; Destination `central-park` already published.
- **Steps**:
  1. Upload dataset.
  2. Inspect job rows and error CSV.
- **Expected Result**:
  - First duplicate slug row referencing existing destination fails with `slug already exists`.
  - Second duplicate within file fails with `duplicate slug in import`.
  - Other unique rows succeed.

### SIT-006 – Gallery Limit Enforcement
- **Goal**: Reject rows with >3 gallery URLs.
- **Preconditions/Test Data**: CSV row containing four gallery URLs.
- **Steps**:
  1. Upload dataset.
  2. Download error CSV.
- **Expected Result**:
  - Row flagged with `gallery limit exceeded` error and omitted from `changes_created`.

### SIT-007 – Coordinate Validation
- **Goal**: Verify latitude/longitude bounds enforced.
- **Preconditions/Test Data**: Row with latitude `-95` or longitude `181`.
- **Steps**:
  1. Upload dataset.
  2. Inspect error CSV.
- **Expected Result**:
  - Error message `latitude must be between -90 and 90` (and/or longitude).
  - No change request for invalid row.

### SIT-008 – Missing Hero Image
- **Goal**: Ensure hero image requirement for create rows persists.
- **Preconditions/Test Data**: Row where `hero_image_url` blank.
- **Steps**:
  1. Upload dataset.
  2. Inspect job + errors.
- **Expected Result**:
  - Row fails with `hero image required`.
  - Job `changes_created` unaffected for valid rows.

### SIT-009 – Authorization & Feature Flag
- **Goal**: Validate only admins with feature flag enabled can access endpoints.
- **Preconditions/Test Data**: Admin flag toggled off; non-admin token.
- **Steps**:
  1. Call `POST /api/v1/admin/destination-imports` with non-admin token → expect `403`.
  2. Disable `ENABLE_DESTINATION_BULK_IMPORT` and retry as admin → expect `403 feature disabled`.
- **Expected Result**:
  - Access denied in both cases; no job created.

### SIT-010 – Error CSV Download & Security
- **Goal**: Confirm error files stored privately and only job owner/admins can download.
- **Preconditions/Test Data**: Job with failed rows owned by Admin A; Admin B also admin.
- **Steps**:
  1. Admin B downloads errors → expect success because admin role allowed.
  2. Non-admin attempts download → expect `403`.
- **Expected Result**:
  - Error CSV contents match recorded errors.
  - RBAC enforced per role.

### SIT-011 – Observability & Metrics
- **Goal**: Ensure metrics/logs emit expected values.
- **Preconditions/Test Data**: Prometheus scraping enabled; log aggregation reachable.
- **Steps**:
  1. Run a successful job.
  2. Query metrics endpoint for `destination_import_jobs_total{status="completed"}` increment.
  3. Inspect logs for job summary line (contains job id, rows processed).
- **Expected Result**:
  - Metric counters increment.
  - Structured log entry includes job stats and change IDs.

### SIT-012 – Worker Retry & Failure Handling
- **Goal**: Verify job transitions to `failed` when worker crashes mid-run and can be retried.
- **Preconditions/Test Data**: Long CSV (>200 rows); worker intentionally killed mid-processing.
- **Steps**:
  1. Start import.
  2. Kill worker pod while job `processing`.
  3. Restart worker and trigger retry command.
- **Expected Result**:
  - Job moves to `failed` with partial counters.
  - Retried run either resumes from scratch or enforces single-run invariant (documented behavior), with audit log capturing both attempts.

## 4. Exit Criteria
- All SIT cases above executed and pass.
- No P1/P2 defects open against bulk import.
- Metrics and error CSV downloads verifiably functioning in SIT.

## 5. Execution Evidence (Local)
- **SIT-002 Dry Run**: `POST /api/v1/admin/destination-imports?dry_run=true` with `dry_run.csv` (job `421b3e7d-cb3c-4596-aa91-5b1ee51374a1`) → `changes_created=0`, row status `skipped`.
- **SIT-003 Validation Failure**: `POST /api/v1/admin/destination-imports` with `valid_import.csv` (job `35190d3f-1c71-4875-aeaf-d3f8ba0cc905`) → museum row failed with `category not allowed`, error CSV download confirmed.
- **SIT-001 Successful Import**: `POST /api/v1/admin/destination-imports` with `valid_import2.csv` (job `a3931b6d-359f-4a56-a426-6fc8bd769c6d`) → two change requests created and auto-submitted (`pending_review`), job counters reflect success.
