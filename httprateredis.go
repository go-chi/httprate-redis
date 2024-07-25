package httprateredis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

	pipe := conn.TxPipeline()
	incrCmd := pipe.IncrBy(ctx, hkey, int64(amount))
	expireCmd := pipe.Expire(ctx, hkey, c.windowLength*2+time.Second)
	_, err := pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("httprateredis: redis transaction failed: %w", err)
	}

	if err := incrCmd.Err(); err != nil {
		return fmt.Errorf("httprateredis: redis incr failed: %w", err)
	}

	if err := expireCmd.Err(); err != nil {
		return fmt.Errorf("httprateredis: redis expire failed: %w", err)
	}

	return nil
}

func (c *redisCounter) Get(key string, currentWindow, previousWindow time.Time) (int, int, error) {
	ctx := context.Background()
	conn := c.client

	currKey := c.limitCounterKey(key, currentWindow)
	prevKey := c.limitCounterKey(key, previousWindow)

	values, err := conn.MGet(ctx, currKey, prevKey).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("httprateredis: redis mget failed: %w", err)
	} else if len(values) != 2 {
		return 0, 0, fmt.Errorf("httprateredis: redis mget returned wrong number of keys: %v, expected 2", len(values))
	}

	var curr, prev int

	// MGET always returns slice with nil or "string" values, even if the values
	// were created with the INCR command. Ignore error if we can't parse the number.
	if values[0] != nil {
		v, _ := values[0].(string)
		curr, _ = strconv.Atoi(v)
	}
	if values[1] != nil {
		v, _ := values[1].(string)
		prev, _ = strconv.Atoi(v)
	}

	return curr, prev, nil
}

func (c *redisCounter) limitCounterKey(key string, window time.Time) string {
	return fmt.Sprintf("%s:%d", c.prefixKey, httprate.LimitCounterKey(key, window))
}
