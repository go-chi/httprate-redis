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

	rc, _ := httprateredis.NewRedisLimitCounter(&httprateredis.Config{
		Host: "127.0.0.1", Port: 6379,
	})

	r.Group(func(r chi.Router) {
		// Set an extra header demonstrating which backend is currently
		// in use (redis vs. local in-memory fallback).
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if rc.IsRedisDown() {
					w.Header().Set("X-RateLimit-Backend", "in-memory")
				} else {
					w.Header().Set("X-RateLimit-Backend", "redis")
				}
				next.ServeHTTP(w, r)
			})
		})

		r.Use(httprate.Limit(
			100, time.Second,
			//5, time.Minute,
			httprate.WithKeyByIP(),
			// httprateredis.WithRedisLimitCounter(&httprateredis.Config{
			// 	Host: "127.0.0.1", Port: 6379,
			// }),
			httprate.WithLimitCounter(rc),
		))

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("5 req/min\n"))
		})
	})

	log.Printf("Serving at localhost:3333")
	log.Println()
	log.Printf("Try running:")
	log.Printf("curl -v http://localhost:3333")

	http.ListenAndServe(":3333", r)
}
