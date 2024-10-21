package main

import (
	"log"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
	"github.com/go-chi/telemetry"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	// Expose Prometheus endpoint at /metrics path.
	r.Use(telemetry.Collector(telemetry.Config{AllowAny: true}))

	rc := httprateredis.NewRedisLimitCounter(&httprateredis.Config{
		Host: "127.0.0.1", Port: 6379,
	})

	r.Group(func(r chi.Router) {
		// Set an extra header demonstrating which backend is currently
		// in use (redis vs. local in-memory fallback).
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if rc.IsFallbackActivated() {
					w.Header().Set("X-RateLimit-Backend", "in-memory")
				} else {
					w.Header().Set("X-RateLimit-Backend", "redis")
				}
				next.ServeHTTP(w, r)
			})
		})

		// Rate-limit at 50 req/s per IP address.
		r.Use(httprate.Limit(
			50, time.Second,
			httprate.WithKeyByIP(),
			httprateredis.WithRedisLimitCounter(&httprateredis.Config{
				Host: "127.0.0.1", Port: 6379,
			}),
			httprate.WithLimitCounter(rc),
		))

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("ok\n"))
		})
	})

	log.Printf("Serving at http://localhost:3333, rate-limited at 50 req/s per IP address")
	log.Println()
	log.Printf("Try making 55 requests:")
	log.Println(`curl -s -o /dev/null -w "Request #%{xfer_id} => Response HTTP %{http_code} (backend: %header{X-Ratelimit-Backend}, limit: %header{X-Ratelimit-Limit}, remaining: %header{X-Ratelimit-Remaining})\n" "http://localhost:3333?req=[0-54]"`)

	http.ListenAndServe(":3333", r)
}
