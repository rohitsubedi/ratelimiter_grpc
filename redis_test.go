package ratelimiter_grpc

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestDefaultRedisCache(t *testing.T) {
	cacheTime := 1 * time.Second
	cacheKey := uuid.New().String()

	c, err := newRedisCache("0.0.0.0:6379", "redis_password")
	assert.NoError(t, err)

	err = c.appendEntry(cacheKey, cacheTime)
	assert.NoError(t, err)

	val := c.getCount(cacheKey, cacheTime)
	assert.Equal(t, val, 1)

	time.Sleep(cacheTime) // cache counter should be deleted
	val = c.getCount(cacheKey, cacheTime)
	assert.Equal(t, val, 0)
}
