package ratelimit

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestStorage_GetSet(t *testing.T) {
	storage := NewStorage()
	defer storage.Stop()

	// Test Get on empty storage
	bucket := storage.Get("test-key")
	if bucket != nil {
		t.Error("Get() should return nil for non-existent key")
	}

	// Test Set and Get
	testBucket := &Bucket{
		Tokens:     10.0,
		LastRefill: time.Now(),
		Capacity:   10.0,
		RefillRate: 1.0,
	}
	storage.Set("test-key", testBucket)

	retrieved := storage.Get("test-key")
	if retrieved == nil {
		t.Fatal("Get() should return bucket after Set()")
	}

	if retrieved.Tokens != testBucket.Tokens {
		t.Errorf("Tokens = %f, want %f", retrieved.Tokens, testBucket.Tokens)
	}

	if retrieved.Capacity != testBucket.Capacity {
		t.Errorf("Capacity = %f, want %f", retrieved.Capacity, testBucket.Capacity)
	}
}

func TestStorage_Delete(t *testing.T) {
	storage := NewStorage()
	defer storage.Stop()

	// Set a bucket
	testBucket := &Bucket{
		Tokens:     5.0,
		LastRefill: time.Now(),
		Capacity:   10.0,
		RefillRate: 1.0,
	}
	storage.Set("test-key", testBucket)

	// Verify it exists
	if storage.Get("test-key") == nil {
		t.Fatal("Bucket should exist before Delete()")
	}

	// Delete it
	storage.Delete("test-key")

	// Verify it's gone
	if storage.Get("test-key") != nil {
		t.Error("Bucket should not exist after Delete()")
	}
}

func TestStorage_ConcurrentAccess(t *testing.T) {
	storage := NewStorage()
	defer storage.Stop()

	var wg sync.WaitGroup
	numGoroutines := 100
	numOperations := 100

	// Concurrent writes
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "key-" + string(rune(id%10))
				bucket := &Bucket{
					Tokens:     float64(j),
					LastRefill: time.Now(),
					Capacity:   100.0,
					RefillRate: 1.0,
				}
				storage.Set(key, bucket)
			}
		}(i)
	}

	// Concurrent reads
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < numOperations; j++ {
				key := "key-" + string(rune(id%10))
				storage.Get(key)
			}
		}(i)
	}

	wg.Wait()

	// Should have some buckets stored
	if storage.Count() == 0 {
		t.Error("Storage should have buckets after concurrent operations")
	}
}

func TestStorage_Cleanup(t *testing.T) {
	storage := NewStorage()
	defer storage.Stop()

	// Create buckets with different ages
	now := time.Now()

	// Fresh bucket (should not be cleaned)
	freshBucket := &Bucket{
		Tokens:     10.0,
		LastRefill: now,
		Capacity:   10.0,
		RefillRate: 1.0,
	}
	storage.Set("fresh", freshBucket)

	// Old bucket (should be cleaned)
	oldBucket := &Bucket{
		Tokens:     10.0,
		LastRefill: now.Add(-2 * time.Hour),
		Capacity:   10.0,
		RefillRate: 1.0,
	}
	storage.Set("old", oldBucket)

	// Verify both exist
	if storage.Count() != 2 {
		t.Errorf("Count = %d, want 2", storage.Count())
	}

	// Run cleanup
	storage.cleanup()

	// Fresh bucket should still exist
	if storage.Get("fresh") == nil {
		t.Error("Fresh bucket should not be cleaned up")
	}

	// Old bucket should be removed
	if storage.Get("old") != nil {
		t.Error("Old bucket should be cleaned up")
	}

	// Should have 1 bucket left
	if storage.Count() != 1 {
		t.Errorf("Count = %d, want 1", storage.Count())
	}
}

func TestStorage_Count(t *testing.T) {
	storage := NewStorage()
	defer storage.Stop()

	// Empty storage
	if storage.Count() != 0 {
		t.Errorf("Count = %d, want 0", storage.Count())
	}

	// Add buckets
	for i := 0; i < 10; i++ {
		bucket := &Bucket{
			Tokens:     float64(i),
			LastRefill: time.Now(),
			Capacity:   10.0,
			RefillRate: 1.0,
		}
		storage.Set(fmt.Sprintf("key-%d", i), bucket)
	}

	if storage.Count() != 10 {
		t.Errorf("Count = %d, want 10", storage.Count())
	}

	// Delete some
	storage.Delete("key-0")
	storage.Delete("key-1")

	if storage.Count() != 8 {
		t.Errorf("Count = %d, want 8", storage.Count())
	}
}

func TestStorage_Stop(t *testing.T) {
	storage := NewStorage()

	// Add some buckets
	for i := 0; i < 5; i++ {
		bucket := &Bucket{
			Tokens:     float64(i),
			LastRefill: time.Now(),
			Capacity:   10.0,
			RefillRate: 1.0,
		}
		storage.Set(fmt.Sprintf("key-%d", i), bucket)
	}

	// Stop should not panic and should cleanup goroutine
	storage.Stop()

	// Buckets should still be accessible after stop
	if storage.Get("key-0") == nil {
		t.Error("Buckets should still exist after Stop()")
	}
}
