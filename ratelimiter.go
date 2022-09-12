package ratelimiter_grpc

import (
	"context"
	"fmt"
	"log"
	"time"

	"google.golang.org/grpc"
)

type RateLimitKeyFunc func(ctx context.Context) string
type RateLimitingInfo struct {
	LimitKeyFunction             RateLimitKeyFunc
	TimeFrameDurationToCheck     time.Duration
	MaxRequestAllowedInTimeFrame int
	LimitExceedError             error
}

const (
	ErrMsgPossibleBruteForceAttack = "Possible Brute Force Attack"
	msgRateLimitKeyEmpty           = "Rate Limit key is empty"
	msgRateLimitKeyFuncIsNil       = "Rate Limit key function is nil"
	cachePathKeySeparator          = "||||"
)

type ConfigInterface interface {
	IsMethodRateLimited(fullMethodName string) (bool, *RateLimitingInfo)
}

type cacheInterface interface {
	appendEntry(key string, expirationTime time.Duration) error
	getCount(key string, expirationTime time.Duration) int
}

type LeveledLogger interface {
	WithContext(ctx context.Context) LeveledLogger
	Error(args ...interface{})
	Info(args ...interface{})
}

type Limiter interface {
	UnaryRateLimit(conf ConfigInterface) grpc.UnaryServerInterceptor
	StreamRateLimit(conf ConfigInterface) grpc.StreamServerInterceptor
}

type limiter struct {
	cache  cacheInterface
	logger interface{}
}

func NewRateLimiterUsingMemory(cacheCleaningInterval time.Duration) Limiter {
	if cacheCleaningInterval < 0 {
		cacheCleaningInterval = 0
	}

	return &limiter{
		cache:  newMemoryCache(cacheCleaningInterval),
		logger: log.Default(),
	}
}

func NewRateLimiterUsingRedis(host, password string) (Limiter, error) {
	redisCache, err := newRedisCache(host, password)
	if err != nil {
		return nil, err
	}

	rateLimiter := &limiter{
		cache:  redisCache,
		logger: log.Default(),
	}

	return rateLimiter, nil
}

// SetLogger sets your own logger. Pass nil if you don't want to log anything
func (l *limiter) SetLogger(logger LeveledLogger) {
	l.logger = logger
}

func (l *limiter) StreamRateLimit(conf ConfigInterface) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if conf == nil {
			return handler(srv, ss)
		}

		enabled, rateLimitInfo := conf.IsMethodRateLimited(info.FullMethod)
		if !enabled || rateLimitInfo == nil {
			return handler(srv, ss)
		}

		ctx := ss.Context()
		if rateLimitInfo.LimitKeyFunction == nil {
			logInfo(ctx, l.logger, msgRateLimitKeyFuncIsNil)
			return handler(srv, ss)
		}

		rateLimitKey := rateLimitInfo.LimitKeyFunction(ctx)
		if rateLimitKey == "" {
			logInfo(ctx, l.logger, msgRateLimitKeyEmpty)
			return handler(srv, ss)
		}

		cacheKey := fmt.Sprintf("%s%s%s", info.FullMethod, cachePathKeySeparator, rateLimitKey)
		limitCount := l.cache.getCount(cacheKey, rateLimitInfo.TimeFrameDurationToCheck)

		if limitCount >= rateLimitInfo.MaxRequestAllowedInTimeFrame {
			logError(ctx, l.logger, ErrMsgPossibleBruteForceAttack)
			return rateLimitInfo.LimitExceedError
		}

		_ = l.cache.appendEntry(cacheKey, rateLimitInfo.TimeFrameDurationToCheck)

		return handler(srv, ss)
	}
}

func (l *limiter) UnaryRateLimit(conf ConfigInterface) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		if conf == nil {
			return handler(ctx, req)
		}

		enabled, rateLimitInfo := conf.IsMethodRateLimited(info.FullMethod)
		if !enabled || rateLimitInfo == nil {
			return handler(ctx, req)
		}

		if rateLimitInfo.LimitKeyFunction == nil {
			logInfo(ctx, l.logger, msgRateLimitKeyFuncIsNil)
			return handler(ctx, req)
		}

		rateLimitKey := rateLimitInfo.LimitKeyFunction(ctx)
		if rateLimitKey == "" {
			logInfo(ctx, l.logger, msgRateLimitKeyEmpty)
			return handler(ctx, req)
		}

		cacheKey := fmt.Sprintf("%s%s%s", info.FullMethod, cachePathKeySeparator, rateLimitKey)
		limitCount := l.cache.getCount(cacheKey, rateLimitInfo.TimeFrameDurationToCheck)

		if limitCount >= rateLimitInfo.MaxRequestAllowedInTimeFrame {
			logError(ctx, l.logger, ErrMsgPossibleBruteForceAttack)
			return nil, rateLimitInfo.LimitExceedError
		}

		_ = l.cache.appendEntry(cacheKey, rateLimitInfo.TimeFrameDurationToCheck)

		return handler(ctx, req)
	}
}

func logInfo(ctx context.Context, logger interface{}, msg string) {
	if logger == nil {
		return
	}

	switch l := logger.(type) {
	case LeveledLogger:
		l.WithContext(ctx).Info(msg)
	default:
		log.Println(msg)
	}
}

func logError(ctx context.Context, logger interface{}, msg string) {
	if logger == nil {
		return
	}

	switch l := logger.(type) {
	case LeveledLogger:
		l.WithContext(ctx).Error(msg)
	default:
		log.Println(msg)
	}
}
