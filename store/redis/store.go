package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	redislib "github.com/redis/go-redis/v9"

	"velarix/store"
)

type Store struct {
	client          redislib.UniversalClient
	idempotencyTTL  time.Duration
	rateLimitWindow time.Duration
}

func Open(addrOrURL string) (*Store, error) {
	addrOrURL = strings.TrimSpace(addrOrURL)
	if addrOrURL == "" {
		return nil, fmt.Errorf("redis address is required")
	}

	var client redislib.UniversalClient
	if strings.HasPrefix(addrOrURL, "redis://") || strings.HasPrefix(addrOrURL, "rediss://") {
		opts, err := redislib.ParseURL(addrOrURL)
		if err != nil {
			return nil, err
		}
		client = redislib.NewClient(opts)
	} else {
		client = redislib.NewClient(&redislib.Options{Addr: addrOrURL})
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := client.Ping(ctx).Err(); err != nil {
		_ = client.Close()
		return nil, err
	}

	return &Store{
		client:          client,
		idempotencyTTL:  24 * time.Hour,
		rateLimitWindow: 2 * time.Minute,
	}, nil
}

func (s *Store) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	return s.client.Close()
}

func (s *Store) idempotencyKey(orgID string, keyHash string) string {
	return fmt.Sprintf("idempotency:%s:%s", orgID, keyHash)
}

func (s *Store) rateLimitKey(apiKey string) string {
	return "ratelimit:" + apiKey
}

func (s *Store) GetIdempotency(orgID string, keyHash string, maxAge time.Duration) (*store.IdempotencyRecord, error) {
	raw, err := s.client.Get(context.Background(), s.idempotencyKey(orgID, keyHash)).Bytes()
	if err == redislib.Nil {
		return nil, err
	}
	if err != nil {
		return nil, err
	}
	var rec store.IdempotencyRecord
	if err := json.Unmarshal(raw, &rec); err != nil {
		return nil, err
	}
	if maxAge > 0 && rec.CreatedAt > 0 && time.Since(time.UnixMilli(rec.CreatedAt)) > maxAge {
		_ = s.client.Del(context.Background(), s.idempotencyKey(orgID, keyHash)).Err()
		return nil, redislib.Nil
	}
	return &rec, nil
}

func (s *Store) SaveIdempotency(orgID string, keyHash string, rec *store.IdempotencyRecord) error {
	if rec == nil {
		return fmt.Errorf("idempotency record is required")
	}
	if rec.CreatedAt == 0 {
		rec.CreatedAt = time.Now().UnixMilli()
	}
	raw, err := json.Marshal(rec)
	if err != nil {
		return err
	}
	return s.client.Set(context.Background(), s.idempotencyKey(orgID, keyHash), raw, s.idempotencyTTL).Err()
}

func (s *Store) GetRateLimit(apiKey string) ([]time.Time, error) {
	raw, err := s.client.Get(context.Background(), s.rateLimitKey(apiKey)).Bytes()
	if err == redislib.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []time.Time
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *Store) SaveRateLimit(apiKey string, limits []time.Time) error {
	raw, err := json.Marshal(limits)
	if err != nil {
		return err
	}
	return s.client.Set(context.Background(), s.rateLimitKey(apiKey), raw, s.rateLimitWindow).Err()
}

var _ store.IdempotencyStore = (*Store)(nil)
var _ store.RateLimitStore = (*Store)(nil)
