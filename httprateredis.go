package httprateredis

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/go-chi/httprate"
	"github.com/redis/go-redis/v9"
)

func WithRedisLimitCounter(cfg *Config) httprate.Option {
	if cfg.Disabled {
		return httprate.WithNoop()
	}
	rc, _ := NewRedisLimitCounter(cfg)
	return httprate.WithLimitCounter(rc)
}

func NewRedisLimitCounter(cfg *Config) (*redisCounter, error) {
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
	if cfg.FallbackTimeout == 0 {
		if cfg.FallbackDisabled {
			cfg.FallbackTimeout = time.Second
		} else {
			// Activate local in-memory fallback fairly quickly,
			// so we don't slow down incoming requests too much.
			cfg.FallbackTimeout = 100 * time.Millisecond
		}
	}

	rc := &redisCounter{
		prefixKey: cfg.PrefixKey,
	}
	if !cfg.FallbackDisabled {
		rc.fallbackCounter = httprate.NewLocalLimitCounter(cfg.WindowLength)
	}

	if cfg.Client == nil {
		maxIdle, maxActive := cfg.MaxIdle, cfg.MaxActive
		if maxIdle < 1 {
			maxIdle = 5
		}
		if maxActive < 1 {
			maxActive = 10
		}

		rc.client = redis.NewClient(&redis.Options{
			Addr:             fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			Password:         cfg.Password,
			DB:               cfg.DBIndex,
			ClientName:       cfg.ClientName,
			DisableIndentity: true,

			DialTimeout:  2 * cfg.FallbackTimeout,
			ReadTimeout:  cfg.FallbackTimeout,
			WriteTimeout: cfg.FallbackTimeout,
			PoolSize:     maxActive,
			MinIdleConns: 1,
			MaxIdleConns: maxIdle,
			MaxRetries:   -1, // -1 disables retries
		})
	}

	return rc, nil
}

type redisCounter struct {
	client            *redis.Client
	windowLength      time.Duration
	prefixKey         string
	fallbackActivated atomic.Bool
	fallbackCounter   httprate.LimitCounter
}

var _ httprate.LimitCounter = (*redisCounter)(nil)

func (c *redisCounter) Config(requestLimit int, windowLength time.Duration) {
	c.windowLength = windowLength
	if c.fallbackCounter != nil {
		c.fallbackCounter.Config(requestLimit, windowLength)
	}
}

func (c *redisCounter) Increment(key string, currentWindow time.Time) error {
	return c.IncrementBy(key, currentWindow, 1)
}

func (c *redisCounter) IncrementBy(key string, currentWindow time.Time, amount int) (err error) {
	if c.fallbackCounter != nil {
		if c.fallbackActivated.Load() {
			return c.fallbackCounter.IncrementBy(key, currentWindow, amount)
		}
		defer func() {
			if c.shouldFallback(err) {
				err = c.fallbackCounter.IncrementBy(key, currentWindow, amount)
			}
		}()
	}

	// Note: Timeouts are set up directly on the Redis client.
	ctx := context.Background()

	hkey := c.limitCounterKey(key, currentWindow)

	pipe := c.client.TxPipeline()
	incrCmd := pipe.IncrBy(ctx, hkey, int64(amount))
	expireCmd := pipe.Expire(ctx, hkey, c.windowLength*3)

	_, err = pipe.Exec(ctx)
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

func (c *redisCounter) Get(key string, currentWindow, previousWindow time.Time) (curr int, prev int, err error) {
	if c.fallbackCounter != nil {
		if c.fallbackActivated.Load() {
			return c.fallbackCounter.Get(key, currentWindow, previousWindow)
		}
		defer func() {
			if c.shouldFallback(err) {
				curr, prev, err = c.fallbackCounter.Get(key, currentWindow, previousWindow)
			}
		}()
	}

	// Note: Timeouts are set up directly on the Redis client.
	ctx := context.Background()

	currKey := c.limitCounterKey(key, currentWindow)
	prevKey := c.limitCounterKey(key, previousWindow)

	values, err := c.client.MGet(ctx, currKey, prevKey).Result()
	if err != nil {
		return 0, 0, fmt.Errorf("httprateredis: redis mget failed: %w", err)
	} else if len(values) != 2 {
		return 0, 0, fmt.Errorf("httprateredis: redis mget returned wrong number of keys: %v, expected 2", len(values))
	}

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

func (c *redisCounter) IsFallbackActivated() bool {
	return c.fallbackActivated.Load()
}

func (c *redisCounter) shouldFallback(err error) bool {
	if err == nil {
		return false
	}

	// Activate the local in-memory counter fallback, unless activated by some other goroutine.
	alreadyActivated := c.fallbackActivated.Swap(true)
	if !alreadyActivated {
		go c.reconnect()
	}

	return true
}

func (c *redisCounter) reconnect() {
	// Try to re-connect to redis every 200ms.
	for {
		time.Sleep(200 * time.Millisecond)

		err := c.client.Ping(context.Background()).Err()
		if err == nil {
			c.fallbackActivated.Store(false)
			return
		}
	}
}

func (c *redisCounter) limitCounterKey(key string, window time.Time) string {
	return fmt.Sprintf("%s:%d", c.prefixKey, httprate.LimitCounterKey(key, window))
}
