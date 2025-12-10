# Detection Rules – Auth Abuse & Endpoint Abuse

The API already emits structured logs (`time`, `user_uuid`, `ip`, `request.method`, `request.uri`, `response.status`, `latency_ms`, request/response body summaries). The rules below assume those fields land in Elasticsearch under the pattern `app-logs-*` (default `ELASTICSEARCH_LOG_INDEX`).

## Field reference
- `@timestamp`: ingestion time (use `time` if mapped directly)
- `request.uri.keyword`: full URI
- `request.method.keyword`: HTTP verb
- `response.status`: integer HTTP status
- `user_uuid.keyword`: authenticated user or `"anonymous"`
- `ip.keyword`: client IP

## Auth abuse
1) **Brute-force login from single IP**
- Type: Threshold rule (KQL)
- Index: `app-logs-*`
- Interval/from: `5m` / `now-5m`
- Query: `request.uri.keyword: "/api/v1/auth/login" and response.status >= 400 and response.status < 500`
- Group by: `ip.keyword`
- Threshold: `>= 5` hits in 5m
- Severity/Risk: high / 73
- Rationale: flag repeated failed login attempts from one source.

2) **Success after multiple failures (same user + IP)**
- Type: EQL sequence or two-stage threshold. EQL example:
```
sequence by user_uuid.keyword, ip.keyword
  [ authentication where request.uri.keyword == "/api/v1/auth/login" and response.status >= 400 and response.status < 500 ]
    with maxspan=10m
  [ authentication where request.uri.keyword == "/api/v1/auth/login" and response.status == 200 ]
```
- Interval/from: `10m` / `now-10m`
- Severity/Risk: medium / 55
- Rationale: successful login immediately after multiple failures can indicate credential stuffing; follow up with MFA reset.

3) **Password reset spray**
- Type: Threshold rule (KQL)
- Query: `request.uri.keyword: "/api/v1/auth/reset-password" and response.status >= 400 and response.status < 500`
- Group by: `ip.keyword`
- Threshold: `>= 10` in 10m
- Severity/Risk: medium / 47
- Rationale: detects reset token guessing or user enumeration.

## Endpoint abuse
1) **High-rate access to sensitive endpoints**
- Type: Threshold rule (KQL)
- Query: `(request.uri.keyword: "/api/v1/admin/*" or request.uri.keyword: "/api/v1/auth/*" or request.uri.keyword: "/api/v1/destinations/*/export" or request.uri.keyword: "/api/v1/destination-stats/*")`
- Group by: `ip.keyword`
- Threshold: `>= 100` requests in 1m
- Interval/from: `1m` / `now-1m`
- Severity/Risk: high / 73
- Rationale: catch scraping or DoS targeting auth/admin/export surfaces.

2) **Enumeration via 404s on admin paths**
- Type: Threshold rule (KQL)
- Query: `request.uri.keyword: "/api/v1/admin/*" and response.status == 404`
- Group by: `ip.keyword`
- Threshold: `>= 15` in 5m
- Severity/Risk: medium / 55
- Rationale: repeated misses on admin endpoints suggest probing.

3) **Server error burst (5xx)**
- Type: Threshold rule (KQL)
- Query: `response.status >= 500 and response.status < 600`
- Group by: `request.uri.keyword`
- Threshold: `>= 20` in 1m
- Severity/Risk: high / 82
- Rationale: flags instability/attacks causing backend failures; annotate with current release if available.

4) **Internal traffic hitting public endpoints**
- Type: Threshold rule (KQL)
- Query: `ip.keyword: "10.*" and request.uri.keyword: "/api/v1/*"`
- Group by: `ip.keyword`
- Threshold: `>= 50` in 5m
- Severity/Risk: low / 21
- Rationale: detects unexpected internal callers (CI load tests, misrouted services).

## Response playbook (recommendation)
- Triage alert → confirm URI, IP, user, status mix, and volume.
- For auth abuse: rate-limit offending IPs, invalidate session tokens for the user, and require MFA reset.
- For endpoint abuse or 5xx spikes: capture current deploy/version, check upstream health, and apply WAF/rate limits if scraping/DoS is confirmed.

## How to import in Kibana Detection Engine
1. Go to Security → Rules → Create.
2. Pick rule type:
   - Threshold for the KQL rules above (set index pattern `app-logs-*`, interval/from as listed, group-by and threshold).
   - EQL for the success-after-failures sequence (Event category `authentication` if mapped; otherwise map `event.category` or leave empty and keep the KQL query).
3. Set severity/risk scores as suggested and attach the `Security` or `Ops` rule tag.
4. Add actions (Slack/email/webhook) with a 1m throttle to avoid noise.

If you prefer exportable NDJSON, these definitions can be translated to Kibana Detection rules (same queries/thresholds) and imported via the “Import rules” button.
