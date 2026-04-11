package main

import (
	"context"
	"github-release-notifier/internal/config"
	"github-release-notifier/internal/db"
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf("could not load .env file: %v", err)
	}

	cfg := config.Load()

	if err := db.RunMigrations(cfg.MigrateDSN); err != nil {
		log.Fatalf("failed to run migrations: %v", err)
	}

	pool, err := db.NewPool(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	port := cfg.Port

	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"status":"ok"}`))
		if err != nil {
			log.Printf("failed to write health check response: %v", err)
			return
		}
	})

	log.Printf("Server started on port %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}
