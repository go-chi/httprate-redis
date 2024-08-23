module github.com/ddo/httprate-redis/_example

go 1.22.5

replace github.com/ddo/httprate-redis => ../

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/go-chi/httprate v0.12.0
	github.com/ddo/httprate-redis v0.3.0
	github.com/go-chi/telemetry v0.3.4
)

require (
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang/mock v1.6.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.19.0 // indirect
	github.com/prometheus/client_model v0.6.1 // indirect
	github.com/prometheus/common v0.52.3 // indirect
	github.com/prometheus/procfs v0.13.0 // indirect
	github.com/redis/go-redis/v9 v9.6.1 // indirect
	github.com/twmb/murmur3 v1.1.8 // indirect
	github.com/uber-go/tally/v4 v4.1.16 // indirect
	go.uber.org/atomic v1.11.0 // indirect
	golang.org/x/sys v0.19.0 // indirect
	google.golang.org/protobuf v1.33.0 // indirect
)
