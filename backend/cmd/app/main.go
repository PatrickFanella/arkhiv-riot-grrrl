package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/catalog"
	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/storage"

	"git.subcult.tv/subculture-collective/arkhiv-riot-grrrl/internal/app"
)

func main() {
	addr := os.Getenv("API_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	handler := app.Handler()
	if os.Getenv("ARKHIV_PUBLIC") == "1" && os.Getenv("DATABASE_URL") == "" {
		log.Fatal("public mode requires catalog")
	}
	if url := os.Getenv("DATABASE_URL"); url != "" {
		db, err := catalog.Open(context.Background(), url)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Pool.Close()
		migrations := os.Getenv("MIGRATIONS_DIR")
		if migrations == "" {
			migrations = "../migrations"
		}
		db.PublicOnly = os.Getenv("ARKHIV_PUBLIC") == "1"
		if !db.PublicOnly {
			if err = db.Migrate(context.Background(), migrations); err != nil {
				log.Fatal(err)
			}
		}
		store, err := storage.Open(os.Getenv("ARKHIV_ARCHIVE"))
		if err != nil {
			log.Fatal(err)
		}
		defer store.Close()
		handler = app.ArchiveHandler(db, store)
	}
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan error, 1)
	go func() { log.Printf("arkhiv listening on %s", addr); done <- server.ListenAndServe() }()
	select {
	case err := <-done:
		if !errors.Is(err, http.ErrServerClosed) {
			log.Fatal(err)
		}
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdown); err != nil {
			log.Printf("shutdown: %v", err)
			_ = server.Close()
		}
	}
}
