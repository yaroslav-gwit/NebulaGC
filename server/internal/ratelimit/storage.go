package ratelimit

import (
	"sync"
	"time"
)

// Bucket represents a token bucket for rate limiting.
type Bucket struct {
	// Tokens is the current number of available tokens.
	Tokens float64

	// LastRefill is the timestamp of the last token refill.
	LastRefill time.Time

	// Capacity is the maximum number of tokens the bucket can hold.
	Capacity float64

	// RefillRate is the number of tokens added per second.
	RefillRate float64
}

// Storage provides thread-safe in-memory storage for rate limit buckets.
// It uses sync.Map for concurrent access and performs periodic cleanup of expired entries.
type Storage struct {
	buckets   sync.Map
	cleanupMu sync.Mutex
	stopCh    chan struct{}
	wg        sync.WaitGroup
}

// NewStorage creates a new rate limit storage and starts the cleanup goroutine.
func NewStorage() *Storage {
	s := &Storage{
		stopCh: make(chan struct{}),
	}
	s.startCleanup()
	return s
}

// Get retrieves a bucket by key. Returns nil if not found.
func (s *Storage) Get(key string) *Bucket {
	value, ok := s.buckets.Load(key)
	if !ok {
		return nil
	}
	bucket, ok := value.(*Bucket)
	if !ok {
		return nil
	}
	return bucket
}

// Set stores or updates a bucket by key.
func (s *Storage) Set(key string, bucket *Bucket) {
	s.buckets.Store(key, bucket)
}

// Delete removes a bucket by key.
func (s *Storage) Delete(key string) {
	s.buckets.Delete(key)
}

// startCleanup starts a background goroutine that periodically cleans up expired buckets.
func (s *Storage) startCleanup() {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				s.cleanup()
			case <-s.stopCh:
				return
			}
		}
	}()
}

// cleanup removes buckets that haven't been accessed in over 1 hour.
func (s *Storage) cleanup() {
	s.cleanupMu.Lock()
	defer s.cleanupMu.Unlock()

	now := time.Now()
	expireThreshold := now.Add(-1 * time.Hour)

	s.buckets.Range(func(key, value interface{}) bool {
		bucket, ok := value.(*Bucket)
		if !ok {
			return true
		}

		// Remove buckets that haven't been refilled in over an hour
		if bucket.LastRefill.Before(expireThreshold) {
			s.buckets.Delete(key)
		}

		return true
	})
}

// Stop gracefully stops the storage cleanup goroutine.
func (s *Storage) Stop() {
	close(s.stopCh)
	s.wg.Wait()
}

// Count returns the number of buckets currently stored (for testing/monitoring).
func (s *Storage) Count() int {
	count := 0
	s.buckets.Range(func(_, _ interface{}) bool {
		count++
		return true
	})
	return count
}
