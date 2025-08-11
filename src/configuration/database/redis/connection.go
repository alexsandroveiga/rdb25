package redis

import (
	"context"
	"os"

	"github.com/redis/go-redis/v9"
)

const (
	REDIS_URL = "REDIS_URL"
)

func NewRedisConnection(ctx context.Context) (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr: os.Getenv(REDIS_URL),
	})
	_, err := client.Ping(ctx).Result()
	if err != nil {
		return nil, err
	}
	return client, nil
}
