package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github-release-notifier/internal/metrics"
)

type Cache interface {
	Get(ctx context.Context, key string) (value string, found bool, err error)
	Set(ctx context.Context, key string, value string, ttl time.Duration) error
	Close() error
}

type redisCache struct {
	client *redis.Client
}

func NewRedisCache(ctx context.Context, redisURL string) (Cache, error) {
	opt, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis URL: %w", err)
	}

	client := redis.NewClient(opt)

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, errors.Join(fmt.Errorf("ping redis: %w", err), client.Close())
	}

	return &redisCache{client: client}, nil
}

func (c *redisCache) Get(ctx context.Context, key string) (value string, found bool, err error) {
	start := time.Now()
	defer func() {
		result := "hit"
		if err != nil {
			result = "error"
		} else if !found {
			result = "miss"
		}
		metrics.CacheOperationsTotal.WithLabelValues("get", result).Inc()
		metrics.CacheOperationDuration.WithLabelValues("get").Observe(time.Since(start).Seconds())
	}()

	val, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", false, nil
	}

	if err != nil {
		return "", false, fmt.Errorf("redis get: %w", err)
	}

	return val, true, nil
}

func (c *redisCache) Set(ctx context.Context, key, value string, ttl time.Duration) (err error) {
	start := time.Now()
	defer func() {
		metrics.CacheOperationsTotal.WithLabelValues("set", metrics.ResultLabel(err)).Inc()
		metrics.CacheOperationDuration.WithLabelValues("set").Observe(time.Since(start).Seconds())
	}()

	if err = c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

func (c *redisCache) Close() error {
	if err := c.client.Close(); err != nil {
		return fmt.Errorf("close redis client: %w", err)
	}

	return nil
}
