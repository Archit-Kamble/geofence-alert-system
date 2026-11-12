// The server starts here: read settings, connect to the database,
// start the alert hub, and serve HTTP until the process is asked to stop.
package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/Archit-Kamble/geofence-alert-system/backend/internal/api"
	"github.com/Archit-Kamble/geofence-alert-system/backend/internal/store"
	"github.com/Archit-Kamble/geofence-alert-system/backend/internal/ws"
)

func main() {
	// Settings come from environment variables, so the same Docker image
	// runs on a laptop and on the hosting platform without changes.
	databaseURL := env("DATABASE_URL", "postgres://geofence:geofence@localhost:5432/geofence?sslmode=disable")
	port := env("PORT", "8080")
	allowedOrigins := strings.Split(env("ALLOWED_ORIGINS", "*"), ",")

	// ctx is cancelled when the process receives Ctrl+C or a stop signal.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.New(ctx, databaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer st.Close()

	hub := ws.NewHub()
	go hub.Run(ctx) // broadcasts alerts in the background

	server := &http.Server{
		Addr:              ":" + port,
		Handler:           api.NewServer(st, hub, allowedOrigins).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Serve in a goroutine so that main can wait for the stop signal.
	go func() {
		log.Printf("listening on :%s", port)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	<-ctx.Done()
	log.Print("shutting down")

	// Give requests that are in progress up to 10 seconds to finish.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("shutdown: %v", err)
	}
}

// env returns the environment variable, or fallback when it is not set.
func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
