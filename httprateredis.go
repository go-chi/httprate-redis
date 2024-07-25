package httprateredis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/go-chi/httprate"
	"github.com/redis/go-redis/v9"
)

func WithRedisLimitCounter(cfg *Config) httprate.Option {
	if cfg.Disabled {
		return httprate.WithNoop()
	}
	rc, err := NewRedisLimitCounter(cfg)
	if err != nil {
		panic(err)
	}
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
	if cfg.ClientName == "" {
		cfg.ClientName = filepath.Base(os.Args[0])
	}
	if cfg.PrefixKey == "" {
		cfg.PrefixKey = "httprate"
	}

	c, err := newClient(cfg)
	if err != nil {
		return nil, err
	}
	return &redisCounter{
		client:    c,
		prefixKey: cfg.PrefixKey,
	}, nil
}

func newClient(cfg *Config) (*redis.Client, error) {
	if cfg.Client != nil {
		return cfg.Client, nil
	}

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
		ClientName:   cfg.ClientName,
	})

	status := c.Ping(context.Background())
	if status == nil || status.Err() != nil {
		return nil, fmt.Errorf("httprateredis: unable to dial redis host %v", address)
	}

	return c, nil
}

type redisCounter struct {
	client       *redis.Client
	windowLength time.Duration
	prefixKey    string
}

var _ httprate.LimitCounter = &redisCounter{}

func (c *redisCounter) Config(requestLimit int, windowLength time.Duration) {
	c.windowLength = windowLength
}

func (c *redisCounter) Increment(key string, currentWindow time.Time) error {
	return c.IncrementBy(key, currentWindow, 1)
}

func (c *redisCounter) IncrementBy(key string, currentWindow time.Time, amount int) error {
	ctx := context.Background()
	conn := c.client

	hkey := c.limitCounterKey(key, currentWindow)

	cmd := conn.Do(ctx, "INCRBY", hkey, amount)
	if cmd == nil {
		return fmt.Errorf("httprateredis: redis incr failed")
	}
	if err := cmd.Err(); err != nil {
		return err
	}

	cmd = conn.Do(ctx, "EXPIRE", hkey, c.windowLength.Seconds()*3)
	if cmd == nil {
		return fmt.Errorf("httprateredis: redis expire failed")
	}
	if err := cmd.Err(); err != nil {
		return err
	}

	return nil
}

func (c *redisCounter) Get(key string, currentWindow, previousWindow time.Time) (int, int, error) {
	ctx := context.Background()
	conn := c.client

	cmd := conn.Do(ctx, "GET", c.limitCounterKey(key, currentWindow))
	if cmd == nil {
		return 0, 0, fmt.Errorf("httprateredis: redis get curr failed")
	}
	if err := cmd.Err(); err != nil && err != redis.Nil {
		return 0, 0, fmt.Errorf("httprateredis: redis get curr failed: %w", err)
	}

	curr, err := cmd.Int()
	if err != nil && err != redis.Nil {
		return 0, 0, fmt.Errorf("httprateredis: redis int curr value: %w", err)
	}

	cmd = conn.Do(ctx, "GET", c.limitCounterKey(key, previousWindow))
	if cmd == nil {
		return 0, 0, fmt.Errorf("httprateredis: redis get prev failed")
	}

	if err := cmd.Err(); err != nil && err != redis.Nil {
		return 0, 0, fmt.Errorf("httprateredis: redis get prev failed: %w", err)
	}

	var prev int
	prev, err = cmd.Int()
	if err != nil && err != redis.Nil {
		return 0, 0, fmt.Errorf("httprateredis: redis int prev value: %w", err)
	}

	return curr, prev, nil
}

func (c *redisCounter) limitCounterKey(key string, window time.Time) string {
	return fmt.Sprintf("%s:%d", c.prefixKey, httprate.LimitCounterKey(key, window))
}
