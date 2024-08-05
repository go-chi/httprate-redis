package httprateredis

import (
	"time"

	"github.com/redis/go-redis/v9"
)

type Config struct {
	Disabled bool `toml:"disabled"` // default: false

	WindowLength time.Duration `toml:"window_length"` // default: 1m
	ClientName   string        `toml:"client_name"`   // default: os.Args[0]
	PrefixKey    string        `toml:"prefix_key"`    // default: "httprate"

	// Disable the use of the local in-memory fallback mechanism. When enabled,
	// the system will return HTTP 428 for all requests when Redis is down.
	FallbackDisabled bool `toml:"fallback_disabled"` // default: false

	// Timeout for each Redis command after which we fall back to a local
	// in-memory counter. If Redis does not respond within this duration,
	// the system will use the local counter unless it is explicitly disabled.
	FallbackTimeout time.Duration `toml:"fallback_timeout"` // default: 50ms

	// Client if supplied will be used and the below fields will be ignored.
	//
	// NOTE: It's recommended to set short dial/read/write timeouts and disable
	// retries on the client, so the local in-memory fallback can activate quickly.
	Client    *redis.Client `toml:"-"`
	Host      string        `toml:"host"`
	Port      uint16        `toml:"port"`
	Password  string        `toml:"password"`   // optional
	DBIndex   int           `toml:"db_index"`   // default: 0
	MaxIdle   int           `toml:"max_idle"`   // default: 4
	MaxActive int           `toml:"max_active"` // default: 8
}
