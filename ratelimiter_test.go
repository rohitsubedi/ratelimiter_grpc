package ratelimiter_grpc

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

const (
	rateLimitHeaderKey = "ip-address"
	redisHost          = "0.0.0.0:6379"
	redisPassword      = "redis_password"
)

// CUSTOM LEVELED LOGGER
type logger struct{}

func newLogger() LeveledLogger {
	return &logger{}
}

func (lo *logger) WithContext(_ context.Context) LeveledLogger {
	return lo
}

func (lo *logger) Error(args ...interface{}) {

}

func (lo *logger) Info(args ...interface{}) {

}

// CUSTOM SERVER STREAM
type serverStream struct {
	ctx context.Context
}

func (s *serverStream) SetTrailer(md metadata.MD) {

}

func (s *serverStream) SetHeader(metadata.MD) error {
	return nil
}

func (s *serverStream) SendHeader(metadata.MD) error {
	return nil
}

func (s *serverStream) Context() context.Context {
	return s.ctx
}

func (s *serverStream) SendMsg(m interface{}) error {
	return nil
}

func (s *serverStream) RecvMsg(m interface{}) error {
	return nil
}

var (
	rateLimitInfo = &RateLimitingInfo{
		LimitKeyValue:                uuid.New().String(),
		TimeFrameDurationToCheck:     2 * time.Second,
		MaxRequestAllowedInTimeFrame: 10,
		LimitExceedError:             fmt.Errorf("rate limit exceeded"),
	}
)

type config struct{}

func (c *config) IsMethodRateLimited(_ context.Context, _ string) (bool, *RateLimitingInfo) {
	return true, rateLimitInfo
}

type notEnabledConfig struct{}

func (c *notEnabledConfig) IsMethodRateLimited(_ context.Context, _ string) (bool, *RateLimitingInfo) {
	return false, nil
}

// MEMORY UNARY
func TestNewRateLimiterUsingMemory_UnaryRateLimit(t *testing.T) {
	rateLimitInfo.LimitKeyValue = uuid.New().String()
	limiter := &limiter{cache: newMemoryCache(2 * time.Second), logger: log.Default()}
	fn := limiter.UnaryRateLimit(new(config))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(rateLimitHeaderKey, uuid.New().String()))

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		_, err := fn(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "test"}, func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})

		if i >= rateLimitInfo.MaxRequestAllowedInTimeFrame {
			assert.Error(t, err)
			assert.EqualError(t, err, rateLimitInfo.LimitExceedError.Error())
			continue
		}
		assert.NoError(t, err)
	}
}

func TestNewRateLimiterUsingMemory_UnaryRateLimit_NotEnabled(t *testing.T) {
	rateLimitInfo.LimitKeyValue = uuid.New().String()
	limiter := &limiter{cache: newMemoryCache(2 * time.Second)}
	fn := limiter.UnaryRateLimit(new(notEnabledConfig))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(rateLimitHeaderKey, uuid.New().String()))

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		_, err := fn(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "test"}, func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})
		// all request should pass as the path is not rate limited
		assert.NoError(t, err)
	}
}

func TestNewRateLimiterUsingMemory_UnaryRateLimit_LimitKeyIsEmpty(t *testing.T) {
	rateLimitInfo.LimitKeyValue = ""
	limiter := &limiter{cache: newMemoryCache(2 * time.Second), logger: log.Default()}
	conf := new(config)
	fn := limiter.UnaryRateLimit(conf)

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		_, err := fn(context.TODO(), nil, &grpc.UnaryServerInfo{FullMethod: "test"}, func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})
		// all request should pass as the limit key function is will return empty key
		assert.NoError(t, err)
	}
}

func TestNewRateLimiterUsingMemory_UnaryRateLimit_EmptyConf(t *testing.T) {
	limiter := &limiter{cache: newMemoryCache(2 * time.Second), logger: log.Default()}
	fn := limiter.UnaryRateLimit(nil)

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		_, err := fn(context.TODO(), nil, &grpc.UnaryServerInfo{FullMethod: "test"}, func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})
		// all request should pass as config is empty
		assert.NoError(t, err)
	}
}

// MEMORY STREAM
func TestNewRateLimiterUsingMemory_StreamRateLimit(t *testing.T) {
	rateLimitInfo.LimitKeyValue = uuid.New().String()
	limiter := &limiter{cache: newMemoryCache(2 * time.Second), logger: log.Default()}
	limiter.SetLogger(nil)
	fn := limiter.StreamRateLimit(new(config))

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		err := fn(nil, &serverStream{ctx: context.TODO()}, &grpc.StreamServerInfo{FullMethod: "test"}, func(srv interface{}, stream grpc.ServerStream) error {
			return nil
		})

		if i >= rateLimitInfo.MaxRequestAllowedInTimeFrame {
			assert.Error(t, err)
			assert.EqualError(t, err, rateLimitInfo.LimitExceedError.Error())
			continue
		}
		assert.NoError(t, err)
	}
}

func TestNewRateLimiterUsingMemory_StreamRateLimit_NotEnabled(t *testing.T) {
	rateLimitInfo.LimitKeyValue = uuid.New().String()
	limiter := &limiter{cache: newMemoryCache(2 * time.Second), logger: log.Default()}
	fn := limiter.StreamRateLimit(new(notEnabledConfig))

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		err := fn(nil, &serverStream{ctx: context.TODO()}, &grpc.StreamServerInfo{FullMethod: "test"}, func(srv interface{}, stream grpc.ServerStream) error {
			return nil
		})
		// all request should pass as the path is not rate limited
		assert.NoError(t, err)
	}
}

func TestNewRateLimiterUsingMemory_StreamRateLimit_LimitKeyIsEmpty(t *testing.T) {
	rateLimitInfo.LimitKeyValue = ""
	limiter := &limiter{cache: newMemoryCache(2 * time.Second), logger: log.Default()}
	conf := new(config)
	fn := limiter.StreamRateLimit(conf)

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		err := fn(nil, &serverStream{ctx: context.TODO()}, &grpc.StreamServerInfo{FullMethod: "test"}, func(srv interface{}, stream grpc.ServerStream) error {
			return nil
		})
		// all request should pass as the limit key function is will return empty key
		assert.NoError(t, err)
	}
}

func TestNewRateLimiterUsingMemory_StreamRateLimit_EmptyConfig(t *testing.T) {
	limiter := &limiter{cache: newMemoryCache(2 * time.Second), logger: log.Default()}
	fn := limiter.StreamRateLimit(nil)

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		err := fn(nil, &serverStream{ctx: context.TODO()}, &grpc.StreamServerInfo{FullMethod: "test"}, func(srv interface{}, stream grpc.ServerStream) error {
			return nil
		})
		// all request should pass as the config is empty
		assert.NoError(t, err)
	}
}

// REDIS UNARY
func TestNewRateLimiterUsingRedis_UnaryRateLimit(t *testing.T) {
	rateLimitInfo.LimitKeyValue = uuid.New().String()
	cache, err := newRedisCache(redisHost, redisPassword)
	assert.NoError(t, err)
	limiter := &limiter{cache: cache, logger: newLogger()}
	fn := limiter.UnaryRateLimit(new(config))

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		_, err := fn(context.TODO(), nil, &grpc.UnaryServerInfo{FullMethod: "test"}, func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})

		if i >= rateLimitInfo.MaxRequestAllowedInTimeFrame {
			assert.Error(t, err)
			assert.EqualError(t, err, rateLimitInfo.LimitExceedError.Error())
			continue
		}
		assert.NoError(t, err)
	}
}

func TestNewRateLimiterUsingRedis_UnaryRateLimit_NotEnabled(t *testing.T) {
	cache, err := newRedisCache(redisHost, redisPassword)
	assert.NoError(t, err)
	limiter := &limiter{cache: cache}
	fn := limiter.UnaryRateLimit(new(notEnabledConfig))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(rateLimitHeaderKey, uuid.New().String()))

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		_, err := fn(ctx, nil, &grpc.UnaryServerInfo{FullMethod: "test"}, func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})
		// all request should pass as the path is not rate limited
		assert.NoError(t, err)
	}
}

func TestNewRateLimiterUsingRedis_UnaryRateLimit_LimitKeyIsEmpty(t *testing.T) {
	rateLimitInfo.LimitKeyValue = ""
	cache, err := newRedisCache(redisHost, redisPassword)
	assert.NoError(t, err)
	limiter := &limiter{cache: cache, logger: newLogger()}
	conf := new(config)
	fn := limiter.UnaryRateLimit(conf)

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		_, err := fn(context.TODO(), nil, &grpc.UnaryServerInfo{FullMethod: "test"}, func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})
		// all request should pass as the limit key function is will return empty key
		assert.NoError(t, err)
	}
}

func TestNewRateLimiterUsingRedis_UnaryRateLimit_EmptyConfig(t *testing.T) {
	cache, err := newRedisCache(redisHost, redisPassword)
	assert.NoError(t, err)
	limiter := &limiter{cache: cache, logger: newLogger()}
	fn := limiter.UnaryRateLimit(nil)

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		_, err := fn(context.TODO(), nil, &grpc.UnaryServerInfo{FullMethod: "test"}, func(ctx context.Context, req interface{}) (interface{}, error) {
			return nil, nil
		})
		// all request should pass as the config is empty
		assert.NoError(t, err)
	}
}

// REDIS STREAM
func TestNewRateLimiterUsingRedis_StreamRateLimit(t *testing.T) {
	rateLimitInfo.LimitKeyValue = uuid.New().String()
	cache, err := newRedisCache(redisHost, redisPassword)
	assert.NoError(t, err)
	limiter := &limiter{cache: cache, logger: log.Default()}
	fn := limiter.StreamRateLimit(new(config))

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		err := fn(nil, &serverStream{ctx: context.TODO()}, &grpc.StreamServerInfo{FullMethod: "test"}, func(srv interface{}, stream grpc.ServerStream) error {
			return nil
		})

		if i >= rateLimitInfo.MaxRequestAllowedInTimeFrame {
			assert.Error(t, err)
			assert.EqualError(t, err, rateLimitInfo.LimitExceedError.Error())
			continue
		}
		assert.NoError(t, err)
	}
}

func TestNewRateLimiterUsingRedis_StreamRateLimit_NotEnabled(t *testing.T) {
	cache, err := newRedisCache(redisHost, redisPassword)
	assert.NoError(t, err)
	limiter := &limiter{cache: cache}
	fn := limiter.StreamRateLimit(new(notEnabledConfig))
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(rateLimitHeaderKey, uuid.New().String()))

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		err := fn(nil, &serverStream{ctx: ctx}, &grpc.StreamServerInfo{FullMethod: "test"}, func(srv interface{}, stream grpc.ServerStream) error {
			return nil
		})
		// all request should pass as the path is not rate limited
		assert.NoError(t, err)
	}
}

func TestNewRateLimiterUsingRedis_StreamRateLimit_LimitKeyIsEmpty(t *testing.T) {
	rateLimitInfo.LimitKeyValue = ""
	cache, err := newRedisCache(redisHost, redisPassword)
	assert.NoError(t, err)
	limiter := &limiter{cache: cache}
	conf := new(config)
	fn := limiter.StreamRateLimit(conf)

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		err := fn(nil, &serverStream{ctx: context.TODO()}, &grpc.StreamServerInfo{FullMethod: "test"}, func(srv interface{}, stream grpc.ServerStream) error {
			return nil
		})
		// all request should pass as the limit key function is will return empty key
		assert.NoError(t, err)
	}
}

func TestNewRateLimiterUsingRedis_StreamRateLimit_EmptyConfig(t *testing.T) {
	cache, err := newRedisCache(redisHost, redisPassword)
	assert.NoError(t, err)
	limiter := &limiter{cache: cache, logger: newLogger()}
	fn := limiter.StreamRateLimit(nil)

	for i := 0; i < 2*rateLimitInfo.MaxRequestAllowedInTimeFrame; i++ {
		err := fn(nil, &serverStream{ctx: context.TODO()}, &grpc.StreamServerInfo{FullMethod: "test"}, func(srv interface{}, stream grpc.ServerStream) error {
			return nil
		})
		// all request should pass as the limit key function is will return empty key
		assert.NoError(t, err)
	}
}

// PUBLIC METHODS
func TestNewRateLimiterUsingMemory(t *testing.T) {
	NewRateLimiterUsingMemory(-1 * time.Second)
}

func TestNewRateLimiterUsingRedis(t *testing.T) {
	_, err := NewRateLimiterUsingRedis(redisHost, redisPassword)
	assert.NoError(t, err)
}

func TestNewRateLimiterUsingRedis_Error(t *testing.T) {
	_, err := NewRateLimiterUsingRedis(redisHost+"invalid", redisPassword)
	assert.Error(t, err)
}
