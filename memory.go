package ratelimiter_grpc

import (
	"runtime"
	"sync"
	"time"
)

type cacheItem struct {
	creationTime time.Time
	child        *cacheItem
}

type cacheCleaner struct {
	interval *time.Timer
	stop     chan bool
}

type memoryCache struct {
	mu           sync.RWMutex
	items        map[string]*cacheItem
	latestExpiry map[string]time.Duration
	cleaner      *cacheCleaner
}

func newMemoryCache(cleaningTime time.Duration) cacheInterface {
	var cleaner *cacheCleaner
	if cleaningTime > 0 {
		cleaner = &cacheCleaner{
			interval: time.NewTimer(cleaningTime),
			stop:     make(chan bool),
		}
	}

	cache := &memoryCache{
		items:        make(map[string]*cacheItem),
		latestExpiry: make(map[string]time.Duration),
		cleaner:      cleaner,
	}

	cache.cleanExpiredMemoryCache(cleaningTime)

	return cache
}

func (m *memoryCache) appendEntry(key string, expiryDuration time.Duration) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.latestExpiry[key] = expiryDuration

	item := &cacheItem{creationTime: time.Now()}
	if v, ok := m.items[key]; ok {
		item.child = v
	}

	m.items[key] = item
	return nil
}

func (m *memoryCache) getCount(key string, expirationDuration time.Duration) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	m.latestExpiry[key] = expirationDuration

	item, found := m.items[key]
	if !found {
		return 0
	}

	counter := 0
	for {
		if time.Now().After(item.creationTime.Add(expirationDuration)) {
			break
		}
		counter++
		item = item.child

		if item == nil {
			break
		}
	}

	return counter
}

func (m *memoryCache) cleanExpiredMemoryCache(cleanerTime time.Duration) {
	if m.cleaner == nil {
		return
	}

	runtime.SetFinalizer(m.cleaner, stopCleaningRoutine)

	go func() {
		for {
			select {
			case <-m.cleaner.interval.C:
				m.unlinkExpiredCache()
				m.cleaner.interval.Reset(cleanerTime)
			case <-m.cleaner.stop:
				m.cleaner.interval.Stop()
			}
		}
	}()
}

// go routine is stopped when stop is set to true
func stopCleaningRoutine(cleaner *cacheCleaner) {
	cleaner.stop <- true
}

// delete all keys from the memory cache
func (m *memoryCache) unlinkExpiredCache() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for k, item := range m.items {
		expirationDuration := m.latestExpiry[k]
		count := 0
		for {
			if time.Now().After(item.creationTime.Add(expirationDuration)) {
				// If first item is expired, we delete all the entry of the items
				if count == 0 {
					delete(m.items, k)
					delete(m.latestExpiry, k)
					break
				}

				item.child = nil
				break
			}

			if item.child == nil {
				break
			}

			item = item.child
			count++
		}
	}
}
