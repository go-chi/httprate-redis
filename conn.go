package httprateredis

import "github.com/redis/go-redis/v9"

type Config struct {
	// Client if supplied will be used and below fields will be ignored.
	Client    *redis.Client `toml:"-"`
	Host      string        `toml:"host"`
	Port      uint16        `toml:"port"`
	Password  string        `toml:"password"`   // optional
	DBIndex   int           `toml:"db_index"`   // default 0
	MaxIdle   int           `toml:"max_idle"`   // default 4
	MaxActive int           `toml:"max_active"` // default 8

	ClientName string `toml:"client_name"` // default os.Args[0]
	Disabled   bool   `toml:"disabled"`    // default false
	PrefixKey  string `toml:"prefix_key"`  // default "httprate"
}
