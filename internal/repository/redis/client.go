package redis

import (
	"context"
	"fmt"
	"time"

	"github.com/ledger/transaction-engine/internal/config"
	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	Rdb *goredis.Client
}

func NewClient(ctx context.Context, cfg *config.Config) (*Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:         cfg.RedisAddr,
		Password:     cfg.RedisPassword,
		DB:           cfg.RedisDB,
		DialTimeout:  2 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
		PoolSize:     20,
	})

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to ping redis: %w", err)
	}

	return &Client{Rdb: rdb}, nil
}

func (c *Client) Close() error {
	if c.Rdb != nil {
		return c.Rdb.Close()
	}
	return nil
}
