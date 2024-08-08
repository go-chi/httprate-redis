# httprate-redis

![CI workflow](https://github.com/go-chi/httprate-redis/actions/workflows/ci.yml/badge.svg)
[![GoDoc Widget]][GoDoc]

[GoDoc]: https://pkg.go.dev/github.com/go-chi/httprate-redis
[GoDoc Widget]: https://godoc.org/github.com/go-chi/httprate-redis?status.svg

Redis backend for [github.com/go-chi/httprate](https://github.com/go-chi/httprate), implementing `httprate.LimitCounter` interface.

See [_example/main.go](./_example/main.go) for usage.

## Example

```go
package main

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Use(httprate.Limit(
			5,
			time.Minute,
			httprate.WithKeyByIP(),
			httprateredis.WithRedisLimitCounter(&httprateredis.Config{
				Host: "127.0.0.1", Port: 6379,
			}),
		))

	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("This is IP rate-limited by 5 req/min"))
	})

	http.ListenAndServe(":3333", r)
}
```

## LICENSE

MIT
