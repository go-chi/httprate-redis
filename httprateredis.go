package httprateredis

import (
	"fmt"
	"time"

	"github.com/go-chi/httprate"
	"github.com/gomodule/redigo/redis"
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

	dialFn := func() (redis.Conn, error) {
		address := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
		c, err := redis.Dial("tcp", address, redis.DialDatabase(cfg.DBIndex), redis.DialPassword(cfg.Password))
		if err != nil {
			return nil, fmt.Errorf("unable to dial redis host %v: %w", address, err)
		}
		return c, nil
	}

	return &redisCounter{
		pool: newPool(cfg, dialFn),
	}, nil
}

func newPool(cfg *Config, dial func() (redis.Conn, error)) *redis.Pool {
	var maxIdle, maxActive = cfg.MaxIdle, cfg.MaxActive
	if maxIdle <= 0 {
		maxIdle = 20
	}
	if maxActive <= 0 {
		maxActive = 50
	}

	return &redis.Pool{
		// Maximum number of idle connections in the pool.
		MaxIdle: maxIdle,
		// max number of connections
		MaxActive: maxActive,

		Dial: dial,
		TestOnBorrow: func(c redis.Conn, t time.Time) error {
			_, err := c.Do("PING")
			if err != nil {
				return fmt.Errorf("PING failed: %w", err)
			}
			return nil
		},
	}
}

type redisCounter struct {
	pool         *redis.Pool
	windowLength time.Duration
}

var _ httprate.LimitCounter = &redisCounter{}

func (c *redisCounter) Config(requestLimit int, windowLength time.Duration) {
	c.windowLength = windowLength
}

func (c *redisCounter) Increment(key string, currentWindow time.Time) error {
	conn := c.pool.Get()
	defer conn.Close()

	hkey := limitCounterKey(key, currentWindow)

	_, err := conn.Do("INCR", hkey)
	if err != nil {
		return err
	}
	_, err = conn.Do("EXPIRE", hkey, c.windowLength.Seconds()*3)
	if err != nil {
		return err
	}

	return nil
}

func (c *redisCounter) Get(key string, currentWindow, previousWindow time.Time) (int, int, error) {
	conn := c.pool.Get()
	defer conn.Close()

	currValue, err := conn.Do("GET", limitCounterKey(key, currentWindow))
	if err != nil && err != redis.ErrNil {
		return 0, 0, fmt.Errorf("redis get failed: %w", err)
	}

	var curr int
	if currValue != nil {
		curr, err = redis.Int(currValue, nil)
		if err != nil {
			return 0, 0, fmt.Errorf("redis int value: %w", err)
		}
	}

	prevValue, err := conn.Do("GET", limitCounterKey(key, previousWindow))
	if err != nil && err != redis.ErrNil {
		return 0, 0, fmt.Errorf("redis get failed: %w", err)
	}

	var prev int
	if prevValue != nil {
		prev, err = redis.Int(prevValue, nil)
		if err != nil {
			return 0, 0, fmt.Errorf("redis int value: %w", err)
		}
	}

	return curr, prev, nil
}

func limitCounterKey(key string, window time.Time) string {
	return fmt.Sprintf("httprate:%d", httprate.LimitCounterKey(key, window))
}
