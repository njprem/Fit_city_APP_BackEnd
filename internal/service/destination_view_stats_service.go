package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/elastic/go-elasticsearch/v8"
	"github.com/google/uuid"

	"github.com/njprem/Fit_city_APP_BackEnd/internal/domain"
	"github.com/njprem/Fit_city_APP_BackEnd/internal/repository/ports"
)

type DestinationViewStatsConfig struct {
	LogIndex       string
	CacheTTL       time.Duration
	RequestTimeout time.Duration
}

var ErrDestinationViewStatsUnavailable = errors.New("destination view stats unavailable")

type DestinationViewStatsService struct {
	repo           ports.DestinationViewStatsRepository
	es             *elasticsearch.Client
	logIndex       string
	cacheTTL       time.Duration
	requestTimeout time.Duration
}

func NewDestinationViewStatsService(repo ports.DestinationViewStatsRepository, es *elasticsearch.Client, cfg DestinationViewStatsConfig) *DestinationViewStatsService {
	return &DestinationViewStatsService{
		repo:           repo,
		es:             es,
		logIndex:       cfg.LogIndex,
		cacheTTL:       cfg.CacheTTL,
		requestTimeout: cfg.RequestTimeout,
	}
}

func (s *DestinationViewStatsService) GetViewStats(ctx context.Context, dest *domain.Destination, forceRefresh bool) (domain.DestinationViewStats, error) {
	stats, latest, err := s.repo.GetStats(ctx, dest.ID)
	if err != nil {
		return domain.DestinationViewStats{}, err
	}

	needsRefresh := forceRefresh || s.isStale(latest) || len(stats) == 0
	if needsRefresh {
		if err := s.refreshDestinations(ctx, []uuid.UUID{dest.ID}); err != nil {
			if len(stats) == 0 {
				return domain.DestinationViewStats{}, err
			}
			log.Printf("view stats: refresh fallback for %s failed: %v", dest.ID, err)
		} else {
			stats, latest, err = s.repo.GetStats(ctx, dest.ID)
			if err != nil {
				return domain.DestinationViewStats{}, err
			}
		}
	}

	result := domain.DestinationViewStats{
		DestinationID: dest.ID,
		Name:          dest.Name,
		City:          dest.City,
		Country:       dest.Country,
		Ranges:        make(map[domain.DestinationViewRange]domain.DestinationViewStatValue, len(domain.DestinationViewRangesOrdered)),
	}

	for _, key := range domain.DestinationViewRangesOrdered {
		if val, ok := stats[key]; ok {
			result.Ranges[key] = val
			continue
		}
		result.Ranges[key] = domain.DestinationViewStatValue{
			TotalViews:  0,
			UniqueUsers: 0,
			UniqueIPs:   0,
			BucketEnd:   latest,
		}
	}

	return result, nil
}

func (s *DestinationViewStatsService) Trending(ctx context.Context, rangeKey domain.DestinationViewRange, limit int) ([]domain.DestinationPopularityRecord, error) {
	if rangeKey == "" {
		rangeKey = domain.DestinationViewRange24h
	}
	return s.repo.ListTopByRange(ctx, rangeKey, limit)
}

func (s *DestinationViewStatsService) ExportPopularity(ctx context.Context, destinationIDs []uuid.UUID, forceRefresh bool) ([]domain.DestinationPopularityRecord, error) {
	var ids []uuid.UUID
	var err error

	if len(destinationIDs) == 0 {
		ids, err = s.repo.ListPublishedDestinationIDs(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		ids = destinationIDs
	}

	if forceRefresh && len(ids) > 0 && s.es != nil {
		if err := s.refreshDestinations(ctx, ids); err != nil {
			// Log and continue with existing data to avoid blocking exports when ES is unavailable.
			// The view stats endpoints already fallback when refresh fails.
			log.Printf("refresh destinations for export failed: %v", err)
		}
	}

	if len(destinationIDs) == 0 {
		return s.repo.ListAllWithMetadata(ctx)
	}
	return s.repo.ListWithMetadata(ctx, destinationIDs)
}

func (s *DestinationViewStatsService) refreshDestinations(ctx context.Context, destinationIDs []uuid.UUID) error {
	if len(destinationIDs) == 0 {
		return nil
	}
	if s.es == nil {
		return fmt.Errorf("%w: elasticsearch client not configured", ErrDestinationViewStatsUnavailable)
	}

	const chunkSize = 200
	now := time.Now().UTC()
	for start := 0; start < len(destinationIDs); start += chunkSize {
		end := start + chunkSize
		if end > len(destinationIDs) {
			end = len(destinationIDs)
		}
		chunk := destinationIDs[start:end]

		buckets := make([]domain.DestinationViewStatBucket, 0, len(chunk)*len(domain.DestinationViewRangesOrdered))
		for _, rangeKey := range domain.DestinationViewRangesOrdered {
			rangeStats, err := s.fetchRangeStats(ctx, chunk, rangeKey, now)
			if err != nil {
				return err
			}
			for _, id := range chunk {
				value := rangeStats[id]
				if value.BucketEnd.IsZero() {
					value.BucketEnd = now
				}
				duration, _ := rangeKey.Duration()
				var bucketStart time.Time
				if duration > 0 {
					bucketStart = value.BucketEnd.Add(-duration)
				} else {
					bucketStart = time.Time{}
				}
				buckets = append(buckets, domain.DestinationViewStatBucket{
					DestinationID: id,
					RangeKey:      rangeKey,
					BucketStart:   bucketStart,
					BucketEnd:     value.BucketEnd,
					TotalViews:    value.TotalViews,
					UniqueUsers:   value.UniqueUsers,
					UniqueIPs:     value.UniqueIPs,
					UpdatedAt:     now,
				})
			}
		}

		if err := s.repo.UpsertBuckets(ctx, buckets); err != nil {
			return err
		}
	}
	return nil
}

func (s *DestinationViewStatsService) fetchRangeStats(ctx context.Context, destinationIDs []uuid.UUID, rangeKey domain.DestinationViewRange, now time.Time) (map[uuid.UUID]domain.DestinationViewStatValue, error) {
	result := make(map[uuid.UUID]domain.DestinationViewStatValue, len(destinationIDs))
	if len(destinationIDs) == 0 {
		return result, nil
	}
	if s.es == nil {
		return result, fmt.Errorf("%w: elasticsearch client not configured", ErrDestinationViewStatsUnavailable)
	}

	ids := make([]string, 0, len(destinationIDs))
	for _, id := range destinationIDs {
		ids = append(ids, id.String())
	}

	mustFilters := []map[string]any{
		{"term": map[string]any{"request.method.keyword": "GET"}},
		{"term": map[string]any{"response.status": 200}},
		{"prefix": map[string]any{"request.uri.keyword": "/api/v1/destinations/"}},
		{"terms": map[string]any{"response.body.destination.id.keyword": ids}},
	}

	mustNotFilters := []map[string]any{
		{"term": map[string]any{"request.uri.keyword": "/api/v1/destinations"}},
		{"prefix": map[string]any{"ip.keyword": "10."}},
	}

	if duration, ok := rangeKey.Duration(); ok && duration > 0 {
		gte := now.Add(-duration).UTC().Format(time.RFC3339)
		mustFilters = append(mustFilters, map[string]any{
			"range": map[string]any{
				"@timestamp": map[string]any{"gte": gte},
			},
		})
	}

	body := map[string]any{
		"size": 0,
		"query": map[string]any{
			"bool": map[string]any{
				"must":     mustFilters,
				"must_not": mustNotFilters,
			},
		},
		"aggs": map[string]any{
			"destinations": map[string]any{
				"terms": map[string]any{
					"field": "response.body.destination.id.keyword",
					"size":  len(destinationIDs),
				},
				"aggs": map[string]any{
					"unique_users": map[string]any{"cardinality": map[string]any{"field": "user_uuid.keyword"}},
					"unique_ips":   map[string]any{"cardinality": map[string]any{"field": "ip.keyword"}},
				},
			},
		},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}

	reqCtx := ctx
	var cancel context.CancelFunc
	if s.requestTimeout > 0 {
		reqCtx, cancel = context.WithTimeout(ctx, s.requestTimeout)
		defer cancel()
	}

	resp, err := s.es.Search(
		s.es.Search.WithContext(reqCtx),
		s.es.Search.WithIndex(s.logIndex),
		s.es.Search.WithBody(bytes.NewReader(payload)),
	)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDestinationViewStatsUnavailable, err)
	}
	defer resp.Body.Close()

	if resp.IsError() {
		return nil, fmt.Errorf("%w: elasticsearch search error: %s", ErrDestinationViewStatsUnavailable, resp.String())
	}

	var parsed esSearchResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("%w: decode response: %v", ErrDestinationViewStatsUnavailable, err)
	}

	for _, bucket := range parsed.Aggregations.Destinations.Buckets {
		id, err := uuid.Parse(bucket.Key)
		if err != nil {
			continue
		}
		result[id] = domain.DestinationViewStatValue{
			TotalViews:  bucket.DocCount,
			UniqueUsers: int(bucket.UniqueUsers.Value),
			UniqueIPs:   int(bucket.UniqueIPs.Value),
			BucketEnd:   now,
		}
	}

	for _, id := range destinationIDs {
		if _, ok := result[id]; !ok {
			result[id] = domain.DestinationViewStatValue{
				TotalViews:  0,
				UniqueUsers: 0,
				UniqueIPs:   0,
				BucketEnd:   now,
			}
		}
	}

	return result, nil
}

func (s *DestinationViewStatsService) isStale(latest time.Time) bool {
	if s.cacheTTL <= 0 {
		return false
	}
	if latest.IsZero() {
		return true
	}
	return time.Since(latest) > s.cacheTTL
}

func (s *DestinationViewStatsService) RunRollup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			ids, err := s.repo.ListPublishedDestinationIDs(ctx)
			if err != nil {
				log.Printf("view stats rollup: list destinations: %v", err)
				continue
			}
			if err := s.refreshDestinations(ctx, ids); err != nil {
				log.Printf("view stats rollup: refresh failed: %v", err)
			}
		}
	}
}

type esSearchResponse struct {
	Aggregations struct {
		Destinations struct {
			Buckets []struct {
				Key         string `json:"key"`
				DocCount    int64  `json:"doc_count"`
				UniqueUsers struct {
					Value float64 `json:"value"`
				} `json:"unique_users"`
				UniqueIPs struct {
					Value float64 `json:"value"`
				} `json:"unique_ips"`
			} `json:"buckets"`
		} `json:"destinations"`
	} `json:"aggregations"`
}
