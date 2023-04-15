package httprateredis

import (
	"context"
	"fmt"
	"time"

	"github.com/go-chi/httprate"
	"github.com/redis/go-redis/v9"
)

func WithRedisLimitCounter(cfg *Config) httprate.Option {
	rc, _ := NewRedisLimitCounter(cfg)
	return httprate.WithLimitCounter(rc)
}

func NewRedisLimitCounter(cfg *Config) (httprate.LimitCounter, error) {
	if cfg == nil {
		cfg = &Config{}
	}
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port < 1 {
		cfg.Port = 6379
	}

	c, err := newClient(cfg)
	if err != nil {
		return nil, err
	}
	return &redisCounter{
		client: c,
	}, nil
}

func newClient(cfg *Config) (*redis.Client, error) {
	var maxIdle, maxActive = cfg.MaxIdle, cfg.MaxActive
	if maxIdle <= 0 {
		maxIdle = 20
	}
	if maxActive <= 0 {
		maxActive = 50
	}

	address := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	c := redis.NewClient(&redis.Options{
		Addr:         address,
		Password:     cfg.Password,
		DB:           cfg.DBIndex,
		PoolSize:     maxActive,
		MaxIdleConns: maxIdle,
	})

	status := c.Ping(context.Background())

	if status == nil || status.Err() != nil {
		return nil, fmt.Errorf("unable to dial redis host %v", address)
	}

	return c, nil
}

type redisCounter struct {
	client       *redis.Client
	windowLength time.Duration
}

var _ httprate.LimitCounter = &redisCounter{}

func (c *redisCounter) Config(requestLimit int, windowLength time.Duration) {
	c.windowLength = windowLength
}

func (c *redisCounter) Increment(key string, currentWindow time.Time) error {
	conn := c.client

	hkey := limitCounterKey(key, currentWindow)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := conn.Do(ctx, "INCR", hkey)
	if cmd == nil {
		return fmt.Errorf("redis incr failed")
	}

	if err := cmd.Err(); err != nil {
		return err
	}
	cmd = conn.Do(ctx, "EXPIRE", hkey, c.windowLength.Seconds()*3)
	if cmd == nil {
		return fmt.Errorf("redis incr failed")
	}

	if err := cmd.Err(); err != nil {
		return err
	}

	return nil
}

func (c *redisCounter) Get(key string, currentWindow, previousWindow time.Time) (int, int, error) {
	conn := c.client

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	cmd := conn.Do(ctx, "GET", limitCounterKey(key, currentWindow))
	if cmd == nil {
		return 0, 0, fmt.Errorf("redis get failed")
	}

	if err := cmd.Err(); err != nil && err != redis.Nil {
		return 0, 0, fmt.Errorf("redis get failed: %w", err)
	}

	curr, err := cmd.Int()
	if err != nil && err != redis.Nil {
		return 0, 0, fmt.Errorf("redis int value: %w", err)
	}

	cmd = conn.Do(ctx, "GET", limitCounterKey(key, previousWindow))
	if cmd == nil {
		return 0, 0, fmt.Errorf("redis get failed")
	}

	if err := cmd.Err(); err != nil && err != redis.Nil {
		return 0, 0, fmt.Errorf("redis get failed: %w", err)
	}

	var prev int
	prev, err = cmd.Int()

	if err != nil && err != redis.Nil {
		return 0, 0, fmt.Errorf("redis int value: %w", err)
	}

	return curr, prev, nil
}

func limitCounterKey(key string, window time.Time) string {
	return fmt.Sprintf("httprate:%d", httprate.LimitCounterKey(key, window))
}
