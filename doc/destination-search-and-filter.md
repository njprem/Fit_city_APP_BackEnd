# Destination Listing Filters

Endpoint: `GET /api/v1/destinations`

## Query Parameters
- `query`: Full-text search across name, city, country, category, and description.
- `categories` or repeated `category`: Limit results to the provided categories.
- `min_rating` / `max_rating`: Restrict by average review rating (0–5 range).
- `sort`: Ordering strategy – `rating_desc` (default for "rating"), `rating_asc`, `alpha_asc` (`alphabetical`/`alpha`), `alpha_desc`, or `updated_at_desc`.
- `limit` and `offset`: Pagination controls (existing behaviour).

## Behaviour Notes
- Ratings are aggregated from published reviews; destinations without reviews have an average rating of `0` and appear in rating-desc sort after rated destinations.
- Sorting defaults to most recently updated when no `sort` value is supplied.
- Invalid rating ranges or sort values return `400 Bad Request`.

