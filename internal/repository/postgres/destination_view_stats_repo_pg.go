package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
)

type DestinationViewStatsRepository struct {
	db *sqlx.DB
}

func NewDestinationViewStatsRepo(db *sqlx.DB) *DestinationViewStatsRepository {
	return &DestinationViewStatsRepository{db: db}
}

func (r *DestinationViewStatsRepository) UpsertBuckets(ctx context.Context, buckets []domain.DestinationViewStatBucket) error {
	if len(buckets) == 0 {
		return nil
	}

	const query = `
		INSERT INTO destination_view_stats (
			destination_id, range_key, bucket_start, bucket_end,
			total_views, unique_users, unique_ips, updated_at
		) VALUES (
			:destination_id, :range_key, :bucket_start, :bucket_end,
			:total_views, :unique_users, :unique_ips, :updated_at
		)
		ON CONFLICT (destination_id, range_key)
		DO UPDATE SET
			bucket_start = EXCLUDED.bucket_start,
			bucket_end = EXCLUDED.bucket_end,
			total_views = EXCLUDED.total_views,
			unique_users = EXCLUDED.unique_users,
			unique_ips = EXCLUDED.unique_ips,
			updated_at = EXCLUDED.updated_at
	`

	rows := make([]map[string]any, 0, len(buckets))
	for _, bucket := range buckets {
		rows = append(rows, map[string]any{
			"destination_id": bucket.DestinationID,
			"range_key":      bucket.RangeKey,
			"bucket_start":   bucket.BucketStart,
			"bucket_end":     bucket.BucketEnd,
			"total_views":    bucket.TotalViews,
			"unique_users":   bucket.UniqueUsers,
			"unique_ips":     bucket.UniqueIPs,
			"updated_at":     bucket.UpdatedAt,
		})
	}
	_, err := r.db.NamedExecContext(ctx, query, rows)
	return err
}

func (r *DestinationViewStatsRepository) GetStats(ctx context.Context, destinationID uuid.UUID) (map[domain.DestinationViewRange]domain.DestinationViewStatValue, time.Time, error) {
	stats, latest, err := r.GetStatsBulk(ctx, []uuid.UUID{destinationID})
	if err != nil {
		return nil, time.Time{}, err
	}
	return stats[destinationID], latest[destinationID], nil
}

func (r *DestinationViewStatsRepository) GetStatsBulk(ctx context.Context, destinationIDs []uuid.UUID) (map[uuid.UUID]map[domain.DestinationViewRange]domain.DestinationViewStatValue, map[uuid.UUID]time.Time, error) {
	result := make(map[uuid.UUID]map[domain.DestinationViewRange]domain.DestinationViewStatValue, len(destinationIDs))
	latest := make(map[uuid.UUID]time.Time, len(destinationIDs))
	if len(destinationIDs) == 0 {
		return result, latest, nil
	}

	const query = `
		SELECT destination_id, range_key, bucket_start, bucket_end, total_views, unique_users, unique_ips
		FROM destination_view_stats
		WHERE destination_id = ANY($1)
	`

	rows := []struct {
		DestinationID uuid.UUID `db:"destination_id"`
		RangeKey      string    `db:"range_key"`
		BucketStart   time.Time `db:"bucket_start"`
		BucketEnd     time.Time `db:"bucket_end"`
		TotalViews    int64     `db:"total_views"`
		UniqueUsers   int       `db:"unique_users"`
		UniqueIPs     int       `db:"unique_ips"`
	}{}

	if err := r.db.SelectContext(ctx, &rows, query, pq.Array(destinationIDs)); err != nil {
		return nil, nil, err
	}

	for _, row := range rows {
		if _, ok := result[row.DestinationID]; !ok {
			result[row.DestinationID] = make(map[domain.DestinationViewRange]domain.DestinationViewStatValue)
		}
		rangeKey := domain.DestinationViewRange(row.RangeKey)
		value := domain.DestinationViewStatValue{
			TotalViews:  row.TotalViews,
			UniqueUsers: row.UniqueUsers,
			UniqueIPs:   row.UniqueIPs,
			BucketEnd:   row.BucketEnd,
		}
		result[row.DestinationID][rangeKey] = value
		if current, ok := latest[row.DestinationID]; !ok || row.BucketEnd.After(current) {
			latest[row.DestinationID] = row.BucketEnd
		}
	}

	return result, latest, nil
}

func (r *DestinationViewStatsRepository) ListTopByRange(ctx context.Context, rangeKey domain.DestinationViewRange, limit int) ([]domain.DestinationPopularityRecord, error) {
	if limit <= 0 {
		limit = 10
	}

	const query = `
		SELECT d.id, d.name, d.city, d.country,
		       dvs.range_key, dvs.bucket_start, dvs.bucket_end,
		       dvs.total_views, dvs.unique_users, dvs.unique_ips
		FROM destination_view_stats dvs
		JOIN travel_destination d ON d.id = dvs.destination_id
		WHERE d.deleted_at IS NULL
		  AND d.status = 'published'
		  AND dvs.range_key = $1
		ORDER BY dvs.total_views DESC, d.name ASC
		LIMIT $2
	`

	rows := []destinationPopularityRow{}
	if err := r.db.SelectContext(ctx, &rows, query, string(rangeKey), limit); err != nil {
		return nil, err
	}

	return buildPopularityRecords(rows, nil), nil
}

func (r *DestinationViewStatsRepository) ListWithMetadata(ctx context.Context, destinationIDs []uuid.UUID) ([]domain.DestinationPopularityRecord, error) {
	if len(destinationIDs) == 0 {
		return []domain.DestinationPopularityRecord{}, nil
	}

	const query = `
		SELECT d.id, d.name, d.city, d.country,
		       dvs.range_key, dvs.bucket_start, dvs.bucket_end,
		       dvs.total_views, dvs.unique_users, dvs.unique_ips
		FROM travel_destination d
		LEFT JOIN destination_view_stats dvs ON d.id = dvs.destination_id
		WHERE d.id = ANY($1)
		ORDER BY d.name ASC
	`

	rows := []destinationPopularityRow{}
	if err := r.db.SelectContext(ctx, &rows, query, pq.Array(destinationIDs)); err != nil {
		return nil, err
	}

	return buildPopularityRecords(rows, destinationIDs), nil
}

func (r *DestinationViewStatsRepository) ListAllWithMetadata(ctx context.Context) ([]domain.DestinationPopularityRecord, error) {
	const query = `
		SELECT d.id, d.name, d.city, d.country,
		       dvs.range_key, dvs.bucket_start, dvs.bucket_end,
		       dvs.total_views, dvs.unique_users, dvs.unique_ips
		FROM travel_destination d
		LEFT JOIN destination_view_stats dvs ON d.id = dvs.destination_id
		WHERE d.deleted_at IS NULL
		  AND d.status = 'published'
		ORDER BY d.name ASC
	`

	rows := []destinationPopularityRow{}
	if err := r.db.SelectContext(ctx, &rows, query); err != nil {
		return nil, err
	}
	return buildPopularityRecords(rows, nil), nil
}

func (r *DestinationViewStatsRepository) GetCheckpoint(ctx context.Context) (time.Time, error) {
	const query = `SELECT last_bucket_end FROM destination_view_rollup_checkpoint WHERE id = 1`
	var ts time.Time
	err := r.db.GetContext(ctx, &ts, query)
	if err == sql.ErrNoRows {
		return time.Time{}, nil
	}
	return ts, err
}

func (r *DestinationViewStatsRepository) UpdateCheckpoint(ctx context.Context, ts time.Time) error {
	const query = `
		INSERT INTO destination_view_rollup_checkpoint (id, last_bucket_end)
		VALUES (1, $1)
		ON CONFLICT (id) DO UPDATE SET last_bucket_end = EXCLUDED.last_bucket_end
	`
	_, err := r.db.ExecContext(ctx, query, ts)
	return err
}

func (r *DestinationViewStatsRepository) ListPublishedDestinationIDs(ctx context.Context) ([]uuid.UUID, error) {
	const query = `
		SELECT id
		FROM travel_destination
		WHERE deleted_at IS NULL
		  AND status = 'published'
	`
	var ids []uuid.UUID
	if err := r.db.SelectContext(ctx, &ids, query); err != nil {
		return nil, err
	}
	return ids, nil
}

type destinationPopularityRow struct {
	DestinationID uuid.UUID      `db:"id"`
	Name          string         `db:"name"`
	City          sql.NullString `db:"city"`
	Country       sql.NullString `db:"country"`
	RangeKey      sql.NullString `db:"range_key"`
	BucketStart   sql.NullTime   `db:"bucket_start"`
	BucketEnd     sql.NullTime   `db:"bucket_end"`
	TotalViews    sql.NullInt64  `db:"total_views"`
	UniqueUsers   sql.NullInt64  `db:"unique_users"`
	UniqueIPs     sql.NullInt64  `db:"unique_ips"`
}

func buildPopularityRecords(rows []destinationPopularityRow, ids []uuid.UUID) []domain.DestinationPopularityRecord {
	records := make(map[uuid.UUID]domain.DestinationPopularityRecord)
	order := make([]uuid.UUID, 0)

	for _, row := range rows {
		rec, ok := records[row.DestinationID]
		if !ok {
			rec = domain.DestinationPopularityRecord{
				DestinationID: row.DestinationID,
				Name:          row.Name,
				Stats:         make(map[domain.DestinationViewRange]domain.DestinationViewStatValue),
			}
			if row.City.Valid {
				rec.City = &row.City.String
			}
			if row.Country.Valid {
				rec.Country = &row.Country.String
			}
			records[row.DestinationID] = rec
			order = append(order, row.DestinationID)
		}

		if row.RangeKey.Valid {
			key := domain.DestinationViewRange(row.RangeKey.String)
			rec := records[row.DestinationID]
			rec.Stats[key] = domain.DestinationViewStatValue{
				TotalViews:  row.TotalViews.Int64,
				UniqueUsers: int(row.UniqueUsers.Int64),
				UniqueIPs:   int(row.UniqueIPs.Int64),
				BucketEnd:   row.BucketEnd.Time,
			}
			records[row.DestinationID] = rec
		}
	}

	if len(ids) > 0 {
		// Preserve the request order if a list was provided.
		order = ids
	}

	result := make([]domain.DestinationPopularityRecord, 0, len(records))
	for _, id := range order {
		if rec, ok := records[id]; ok {
			result = append(result, rec)
		}
	}
	return result
}
