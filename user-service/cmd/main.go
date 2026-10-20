package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"user-service/internal/config"
	"user-service/internal/database"
	"user-service/internal/database/repository"
	"user-service/internal/handlers"

	"github.com/gorilla/mux"
	"golang.org/x/sync/errgroup"
)

func main() {
	if err := run(); err != nil {
		slog.Error("service stopped with error", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()

	if err != nil {
		return fmt.Errorf("config: %w", err)
	}

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: cfg.LogLevel}))
	slog.SetDefault(logger)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	startCtx, cancelStart := context.WithTimeout(ctx, 15*time.Second)
	db, err := database.NewCluster(startCtx, cfg.Database, logger)
	cancelStart()

	if err != nil {
		return fmt.Errorf("database: %w", err)
	}

	defer db.Close()

	repo := repository.NewUserRepository(db)

	router := mux.NewRouter()
	handlers.RegisterUserHandlers(router, repo)

	httpSrv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelWarn),
	}

	g, gctx := errgroup.WithContext(ctx)

	g.Go(func() error {
		db.Monitor(gctx)
		return nil
	})

	g.Go(func() error {
		logger.Info("http listening", "addr", cfg.HTTPAddr)
		if err := httpSrv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("http: %w", err)
		}
		return nil
	})

	g.Go(func() error {
		<-gctx.Done()
		logger.Info("shutting down")

		sctx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
		defer cancel()

		var errs []error
		if err := httpSrv.Shutdown(sctx); err != nil {
			errs = append(errs, fmt.Errorf("http shutdown: %w", err))
			_ = httpSrv.Close()
		}

		return errors.Join(errs...)
	})

	err = g.Wait()
	logger.Info("stopped")
	return err
}
