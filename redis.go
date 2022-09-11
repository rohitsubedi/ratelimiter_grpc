package ratelimiter_grpc

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
)

var (
	errConnectingRedis = fmt.Errorf("cache lib: cannot connect to redis server")
	timeFormatForRedis = time.RFC3339Nano
)

type redisCache struct {
	redisClient *redis.Client
}

func newRedisCache(host, password string) (cacheInterface, error) {
	client := redis.NewClient(&redis.Options{
		Addr:     host,
		Password: password,
	})

	if _, err := client.Ping(context.Background()).Result(); err != nil {
		return nil, fmt.Errorf("%v: %w", errConnectingRedis, err)
	}

	return &redisCache{
		redisClient: client,
	}, nil
}

func (r *redisCache) appendEntry(key string, expirationDuration time.Duration) error {
	previousValue := r.redisClient.Get(context.Background(), key).Val()

	currentValue := time.Now().Format(timeFormatForRedis)
	if previousValue != "" {
		currentValue = fmt.Sprintf("%s,%s", currentValue, previousValue)
	}

	if err := r.redisClient.Set(context.Background(), key, currentValue, expirationDuration).Err(); err != nil {
		return err
	}

	return nil
}

func (r *redisCache) getCount(key string, expirationDuration time.Duration) (count int) {
	currentValue := r.redisClient.Get(context.Background(), key).Val()
	if currentValue == "" {
		return 0
	}

	for _, v := range strings.Split(currentValue, ",") {
		itemCreatedTime, err := time.Parse(timeFormatForRedis, v)
		if err != nil {
			continue
		}

		if time.Now().After(itemCreatedTime.Add(expirationDuration)) {
			break
		}

		count++
	}

	return count
}
