package ratelimiter_grpc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestDefaultCache(t *testing.T) {
	cacheTime := 1 * time.Second
	cacheKey := uuid.New().String()

	c := newMemoryCache(cacheTime)
	err := c.appendEntry(cacheKey, cacheTime)
	assert.NoError(t, err)
	val := c.getCount(cacheKey, cacheTime)
	assert.Equal(t, val, 1)

	time.Sleep(cacheTime) // cache counter should be deleted
	val = c.getCount(cacheKey, cacheTime)
	assert.Equal(t, val, 0)
}

func TestUnlinkExpiredCache(t *testing.T) {
	pathName := "testing-path-with-rate-limit-key"
	c := &memoryCache{
		items:        make(map[string]*cacheItem),
		latestExpiry: make(map[string]time.Duration),
	}

	c.latestExpiry[pathName] = 60 * time.Second
	c.items[pathName] = &cacheItem{
		creationTime: time.Now(),
		child: &cacheItem{
			creationTime: time.Now(),
			child: &cacheItem{
				creationTime: time.Now().Add(-2 * time.Minute),
				child: &cacheItem{
					creationTime: time.Now().Add(-2 * time.Minute),
				},
			},
		},
	}

	c.unlinkExpiredCache()
	assert.NotEmpty(t, c.items[pathName])
	assert.Equal(t, c.getCount(pathName, c.latestExpiry[pathName]), 2) // Only third child is expired
	// Also the third item should be deleted
	count := 0
	items := c.items[pathName]
	for {
		count++
		items = items.child
		assert.True(t, count <= 2)
		if items.child == nil {
			break
		}
	}
}

func TestUnlinkExpiredCache_AllExpired(t *testing.T) {
	pathName := "testing-path-with-rate-limit-key"
	c := &memoryCache{
		items:        make(map[string]*cacheItem),
		latestExpiry: make(map[string]time.Duration),
	}

	c.latestExpiry[pathName] = 60 * time.Second
	c.items[pathName] = &cacheItem{
		creationTime: time.Now().Add(-2 * time.Minute),
		child: &cacheItem{
			creationTime: time.Now().Add(-2 * time.Minute),
			child: &cacheItem{
				creationTime: time.Now().Add(-2 * time.Minute),
			},
		},
	}

	c.unlinkExpiredCache()
	item, ok := c.items[pathName]
	assert.Empty(t, item)
	assert.False(t, ok)
}
