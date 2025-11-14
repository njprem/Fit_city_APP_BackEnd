# Destination View Stats – System Integration Test Plan

## 1. Scope & Assumptions
- Validates end-to-end view-stat retrieval for both public users and admins, including cache logic, Elasticsearch aggregations, and hourly rollup persistence.
- Environment mirrors SIT: shared Postgres, Elasticsearch reachable via the configured `ELASTICSEARCH_BASE_URL`, hourly rollup worker enabled, and API gateway reachable over `/api/v1`.
- `DEST_VIEW_STATS_CACHE_TTL` defaulted to `10m` and `DEST_VIEW_STATS_ROLLUP_INTERVAL` to `1h` unless the test case explicitly overrides via env.
- Structured access logs already flow into `app-logs-*`; log generators can replay synthetic hits for deterministic counts.

## 2. Test Data & Utilities
- **Synthetic traffic script**: `scripts/k6/view_published_destinations.js` generates GET calls to `/api/v1/destinations/:id` for known destination IDs to seed Elasticsearch.
- **Database helpers**:
  - `destination_view_stats` – inspect/upsert rows via SQL client.
  - `destination_view_rollup_checkpoint` – track latest processed `bucket_end`.
- **Elasticsearch queries**: use `docs/api-design/Destination-View-Stats-Design.md` aggregation as reference for validating counts.

## 3. Test Cases

### SIT-VW-001 – Public View Uses Cache When Fresh
- **Goal**: Ensure `/api/v1/destinations/:id/views` serves from Postgres when data is < cache TTL.
- **Steps**:
  1. Seed stats via ES query + manual upsert (or wait for rollup) so `bucket_end` is current.
  2. Call endpoint twice within `DEST_VIEW_STATS_CACHE_TTL`.
  3. Capture Postgres query logs / metrics.
- **Expected Result**:
  - First call may trigger ES (if initial cache empty), subsequent call hits Postgres only.
  - Response latency <50 ms; no ES call logged during second request.

### SIT-VW-002 – Public View Refreshes When Cache Stale
- **Goal**: Non-admin request older than TTL should reaggregate from Elasticsearch.
- **Steps**:
  1. Set `DEST_VIEW_STATS_CACHE_TTL=2m`; wait >2 m without updates.
  2. Generate new GET traffic to destination directly (to ensure ES has fresh hits).
  3. Call `/api/v1/destinations/:id/views`.
- **Expected Result**:
  - Handler invokes ES query (verify via logs).
  - `destination_view_stats` table receives new upsert row with current `bucket_end`.
  - Response reflects new view totals.

### SIT-VW-003 – Admin Always Forces ES Refresh
- **Goal**: `/api/v1/admin/destination-stats/views` should query ES even when cache fresh.
- **Steps**:
  1. Warm cache with recent data (<TTL).
  2. Call admin endpoint as privileged user.
- **Expected Result**:
  - Logs show ES request constructed.
  - Postgres table updated/confirmed even though cache was fresh.
  - Response includes latest counts and optional histogram per `range`/`interval`.

### SIT-VW-004 – 1 Request Equals 1 View (Count Validation)
- **Goal**: Verify counting rule uses total hits, not deduped IP/user.
- **Steps**:
  1. Replay exactly 25 GET requests for destination `A` from same IP and account.
  2. Run `/api/v1/destinations/:id/views` post-refresh.
- **Expected Result**:
  - `total_views` increments by 25.
  - `unique_users` remains 1; `unique_ips` equals 1.

### SIT-VW-005 – Internal Traffic (10.* IP) Excluded
- **Goal**: Confirm IP filter drops internal load tests.
- **Steps**:
  1. Send GET requests via a host with IP `10.x.x.x` (simulate using proxy or rewrite logs).
  2. Ensure equivalent hits from public IP exist for comparison.
- **Expected Result**:
  - Only public IP hits impact counts.
  - ES query with `must_not prefix ip.keyword: "10."` visible in logs.

### SIT-VW-006 – Trending Endpoint Returns Leaderboard
- **Goal**: Validate `/api/v1/destinations/trending` sorts by view counts.
- **Steps**:
  1. Generate traffic for three destinations with different volumes.
  2. Call trending endpoint with `range=24h&limit=2`.
- **Expected Result**:
  - Response list contains top two destinations ordered by total views.
  - Each entry includes same stats structure as single view endpoint.

### SIT-VW-007 – Hourly Rollup Updates Table Without Traffic
- **Goal**: Ensure rollup worker updates Postgres even when endpoints unused.
- **Steps**:
  1. Stop manual traffic after seeding.
  2. Wait for `DEST_VIEW_STATS_ROLLUP_INTERVAL` (default 1h) with worker running.
  3. Inspect `destination_view_stats` and checkpoint table.
- **Expected Result**:
  - New bucket rows inserted with timestamps aligning to rollup window.
  - Checkpoint `last_bucket_end` matches latest bucket.

### SIT-VW-008 – Configurable TTL & Interval via Env
- **Goal**: Changing env vars should alter behavior.
- **Steps**:
  1. Set `DEST_VIEW_STATS_CACHE_TTL=30s` and `DEST_VIEW_STATS_ROLLUP_INTERVAL=15m`.
  2. Redeploy API + worker.
  3. Measure time between automatic PG refreshes and cache expiry.
- **Expected Result**:
  - Cache flips to ES refresh roughly every 30 s.
  - Rollup job logs every ~15 m.

### SIT-VW-009 – Authorization Matrix
- **Goal**: Verify public vs admin route protections.
- **Steps**:
  1. Anonymous user hits `/destinations/:id/views` → expect `200`.
  2. Anonymous user hits `/admin/destination-stats/views` → expect `401/403`.
  3. Authenticated non-admin hits admin route → expect `403`.
- **Expected Result**:
  - Only admin token succeeds on admin endpoint.

### SIT-VW-010 – Failure Handling & Fallback
- **Goal**: When ES unavailable, API should degrade gracefully (serve cache or error).
- **Steps**:
  1. Bring down ES service temporarily.
  2. With cached stats fresh, call public endpoint.
  3. With cache stale, call public endpoint again.
- **Expected Result**:
  - Fresh-cache request still succeeds from Postgres.
  - Stale request returns `503` or documented error indicating stats unavailable.
  - Error logged and metrics emitted.

-### SIT-VW-011 – Admin CSV Export
- **Goal**: `POST /api/v1/admin/destination-stats/export` returns CSV with requested ranges.
- **Steps**:
  1. Seed view stats for three destinations (ensuring caches are fresh).
  2. Admin calls export endpoint with payload `{"destination_ids":["idA","idB"]}`.
  3. Save CSV response and inspect rows.
- **Expected Result**:
  - Response headers include `Content-Type: text/csv` and `Content-Disposition` attachment.
  - CSV columns appear as: `destination_name,city,country,views_1h,views_6h,views_12h,views_24h,views_7d,views_30d,views_all`.
  - Each value matches the latest stats; order mirrors the request body.
  - When `destination_ids` omitted, export includes all published destinations sorted alphabetically.

## 4. Exit Criteria
- All SIT-VW cases executed with evidence.
- No P1/P2 defects open for view stats feature.
- Rollup job observed running at configured cadence with checkpoints advancing.

## 5. Evidence Capture
- Store Kibana screenshots and `destination_view_stats` snapshots under `docs/testing/data/view-stats/`.
- Attach API response payloads (JSON) per case for traceability.
