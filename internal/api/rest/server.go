package rest

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/ledger/transaction-engine/internal/config"
	"github.com/ledger/transaction-engine/internal/repository/postgres"
	"github.com/ledger/transaction-engine/internal/repository/redis"
	"github.com/ledger/transaction-engine/internal/service"
)

type Server struct {
	httpServer *http.Server
	router     *chi.Mux
}

func NewServer(
	cfg *config.Config,
	ledgerService *service.LedgerService,
	db *postgres.DB,
	redisClient *redis.Client,
) *Server {
	r := chi.NewRouter()

	// Global Middlewares
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(LoggerMiddleware)
	r.Use(middleware.Recoverer)
	r.Use(SecurityHeadersMiddleware)
	r.Use(middleware.Timeout(30 * time.Second))

	accountHandler := NewAccountHandler(ledgerService)
	transactionHandler := NewTransactionHandler(ledgerService)
	auditHandler := NewAuditHandler(ledgerService, db, redisClient)

	// Health and Readiness
	r.Get("/healthz", auditHandler.Healthz)
	r.Get("/readyz", auditHandler.Readyz)

	// API Routes
	r.Route("/api/v1", func(api chi.Router) {
		// Accounts
		api.Route("/accounts", func(acc chi.Router) {
			acc.Post("/", accountHandler.CreateAccount)
			acc.Get("/", accountHandler.ListAccounts)
			acc.Get("/{id}", accountHandler.GetAccount)
			acc.Get("/{id}/entries", accountHandler.GetAccountEntries)
		})

		// Transactions & Transfers
		api.Route("/transactions", func(tx chi.Router) {
			tx.Post("/", transactionHandler.CreateTransaction)
			tx.Get("/{id}", transactionHandler.GetTransaction)
		})
		api.Post("/transfers", transactionHandler.CreateTransfer)

		// Ledger Integrity Audit
		api.Get("/ledger/audit", auditHandler.AuditLedger)
	})

	httpServer := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	return &Server{
		httpServer: httpServer,
		router:     r,
	}
}

func (s *Server) Start() error {
	return s.httpServer.ListenAndServe()
}

func (s *Server) Shutdown(ctx context.Context) error {
	return s.httpServer.Shutdown(ctx)
}

func (s *Server) Router() *chi.Mux {
	return s.router
}
