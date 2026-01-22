package storage

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisClient interface {
	SetState(ctx context.Context, state, sessionID string, ttl time.Duration) error
	GetState(ctx context.Context, state string) (string, error)
	DeleteState(ctx context.Context, state string) error
	Close() error
}

type redisClientImpl struct {
	client *redis.Client
}

func NewRedisClient(addr string) (RedisClient, error) {
	client := redis.NewClient(&redis.Options{
		Addr: addr,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &redisClientImpl{client: client}, nil
}

func (c *redisClientImpl) SetState(ctx context.Context, state, sessionID string, ttl time.Duration) error {
	key := "oauth:state:" + state
	return c.client.Set(ctx, key, sessionID, ttl).Err()
}

func (c *redisClientImpl) GetState(ctx context.Context, state string) (string, error) {
	key := "oauth:state:" + state
	return c.client.Get(ctx, key).Result()
}

func (c *redisClientImpl) DeleteState(ctx context.Context, state string) error {
	key := "oauth:state:" + state
	return c.client.Del(ctx, key).Err()
}

func (c *redisClientImpl) Close() error {
	return c.client.Close()
}
