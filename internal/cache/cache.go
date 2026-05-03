package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github-release-notifier/internal/domain"

	"github.com/redis/go-redis/v9"
)

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
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
		_ = client.Close()
		return nil, fmt.Errorf("ping redis: %w", err)
	}

	return &redisCache{client: client}, nil
}

func (c *redisCache) Get(ctx context.Context, key string) (string, error) {
	val, err := c.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", domain.ErrMiss
	}

	if err != nil {
		return "", fmt.Errorf("redis get: %w", err)
	}

	return val, nil
}

func (c *redisCache) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	if err := c.client.Set(ctx, key, value, ttl).Err(); err != nil {
		return fmt.Errorf("redis set: %w", err)
	}

	return nil
}

func (c *redisCache) Close() error {
	return c.client.Close()
}
