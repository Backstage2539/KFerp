package main

import (
	"context"
	"log"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	cfg, err := loadAppConfig()
	if err != nil {
		log.Fatal(err)
	}

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	if err := ensureAppSchema(ctx, pool, cfg.Schema); err != nil {
		log.Fatal(err)
	}

	e := newHTTPServer(cfg, pool)
	registerAppRoutes(e, pool, cfg.Schema, cfg.AssetDir)

	log.Printf("orderapp listening on %s", cfg.ListenAddr)
	if err := e.Start(cfg.ListenAddr); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
