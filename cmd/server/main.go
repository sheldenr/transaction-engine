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

	"github.com/ledger/transaction-engine/internal/api/rest"
	"github.com/ledger/transaction-engine/internal/config"
	"github.com/ledger/transaction-engine/internal/repository/postgres"
	"github.com/ledger/transaction-engine/internal/repository/redis"
	"github.com/ledger/transaction-engine/internal/service"
)

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-healthcheck" {
		cfg := config.Load()
		resp, err := http.Get("http://127.0.0.1:" + cfg.Port + "/healthz")
		if err != nil || resp.StatusCode != http.StatusOK {
			os.Exit(1)
		}
		os.Exit(0)
	}

	cfg := config.Load()
	log.Printf("Starting Ledger & Transaction Engine on port :%s (env: %s)...", cfg.Port, cfg.Environment)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Initialize Postgres
	db, err := postgres.NewDB(ctx, cfg)
	if err != nil {
		log.Fatalf("Fatal: Unable to connect to PostgreSQL: %v", err)
	}
	defer db.Close()
	log.Println("Connected to PostgreSQL successfully.")

	// Run embedded migrations
	if err := db.RunMigrations(ctx, postgres.SchemaMigrationSQL); err != nil {
		log.Fatalf("Fatal: Database migration failed: %v", err)
	}
	log.Println("Database migrations applied successfully.")

	// Initialize Redis
	redisClient, err := redis.NewClient(ctx, cfg)
	if err != nil {
		log.Printf("Warning: Failed to connect to Redis (%v). Distributed locks will operate in fallback mode.", err)
		redisClient = nil
	} else {
		defer redisClient.Close()
		log.Println("Connected to Redis successfully.")
	}

	// Wire Repositories
	accRepo := postgres.NewAccountRepository(db)
	txnRepo := postgres.NewTransactionRepository(db)
	idempRepo := postgres.NewIdempotencyRepository(db)
	redisLock := redis.NewLockService(redisClient)

	// Wire Services
	idempService := service.NewIdempotencyService(idempRepo, redisLock, cfg.IdempotencyTTL)
	ledgerService := service.NewLedgerService(accRepo, txnRepo, idempService, cfg.IdempotencyTTL)

	// Wire HTTP Server
	srv := rest.NewServer(cfg, ledgerService, db, redisClient)

	// Graceful shutdown handling
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		if err := srv.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Server crashed: %v", err)
		}
	}()

	<-stopChan
	log.Println("Shutting down gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited cleanly.")
}
