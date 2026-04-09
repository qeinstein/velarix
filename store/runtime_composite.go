package store

import (
	"context"
	"time"

	"velarix/core"
)

type RuntimeCloser interface {
	Close() error
}

// CompositeRuntimeStore keeps the API-facing store surface on one primary
// backend while allowing specific coordination concerns to be delegated to
// shared services such as Redis.
type CompositeRuntimeStore struct {
	ServerStore

	idempotency IdempotencyStore
	rateLimits  RateLimitStore
	closers     []RuntimeCloser
}

func NewCompositeRuntimeStore(primary ServerStore, idempotency IdempotencyStore, rateLimits RateLimitStore, closers ...RuntimeCloser) *CompositeRuntimeStore {
	return &CompositeRuntimeStore{
		ServerStore: primary,
		idempotency: idempotency,
		rateLimits:  rateLimits,
		closers:     closers,
	}
}

func (s *CompositeRuntimeStore) GetIdempotency(orgID string, keyHash string, maxAge time.Duration) (*IdempotencyRecord, error) {
	if s.idempotency != nil {
		return s.idempotency.GetIdempotency(orgID, keyHash, maxAge)
	}
	return s.ServerStore.GetIdempotency(orgID, keyHash, maxAge)
}

func (s *CompositeRuntimeStore) SaveIdempotency(orgID string, keyHash string, rec *IdempotencyRecord) error {
	if s.idempotency != nil {
		return s.idempotency.SaveIdempotency(orgID, keyHash, rec)
	}
	return s.ServerStore.SaveIdempotency(orgID, keyHash, rec)
}

func (s *CompositeRuntimeStore) GetRateLimit(apiKey string) ([]time.Time, error) {
	if s.rateLimits != nil {
		return s.rateLimits.GetRateLimit(apiKey)
	}
	return s.ServerStore.GetRateLimit(apiKey)
}

func (s *CompositeRuntimeStore) SaveRateLimit(apiKey string, limits []time.Time) error {
	if s.rateLimits != nil {
		return s.rateLimits.SaveRateLimit(apiKey, limits)
	}
	return s.ServerStore.SaveRateLimit(apiKey, limits)
}

func (s *CompositeRuntimeStore) ReplayAll(engines map[string]*core.Engine, configs map[string][]byte) error {
	if runtimeStore, ok := s.ServerStore.(interface {
		ReplayAll(map[string]*core.Engine, map[string][]byte) error
	}); ok {
		return runtimeStore.ReplayAll(engines, configs)
	}
	return nil
}

func (s *CompositeRuntimeStore) StartGC() {
	if starter, ok := s.ServerStore.(interface{ StartGC() }); ok {
		starter.StartGC()
	}
}

func (s *CompositeRuntimeStore) BackendName() string {
	if reporter, ok := s.ServerStore.(HealthReporter); ok {
		return reporter.BackendName()
	}
	return "composite"
}

func (s *CompositeRuntimeStore) Ping(ctx context.Context) error {
	if reporter, ok := s.ServerStore.(HealthReporter); ok {
		return reporter.Ping(ctx)
	}
	return nil
}

func (s *CompositeRuntimeStore) Close() error {
	var firstErr error
	for _, closer := range s.closers {
		if closer == nil {
			continue
		}
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
