package httprateredis

type Config struct {
	Host      string `toml:"host"`
	Port      uint16 `toml:"port"`
	DBIndex   int    `toml:"db_index"`   // default 0
	MaxIdle   int    `toml:"max_idle"`   // default 4
	MaxActive int    `toml:"max_active"` // default 8
}
