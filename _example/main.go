package main

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/httprate"
	httprateredis "github.com/go-chi/httprate-redis"
)

func main() {
	r := chi.NewRouter()
	r.Use(middleware.Logger)

	r.Group(func(r chi.Router) {
		r.Use(httprate.Limit(
			10,
			10*time.Second,
			httprate.WithKeyByIP(),
			httprateredis.WithRedisLimitCounter(&httprateredis.Config{
				Host: "127.0.0.1", Port: 6379,
			}),
		))

		r.Get("/", func(w http.ResponseWriter, r *http.Request) {
			w.Write([]byte("."))
		})
	})

	http.ListenAndServe(":3333", r)
}
