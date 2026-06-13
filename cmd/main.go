package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/onix-fun/push-service/internal/api"
	"github.com/onix-fun/push-service/internal/config"
	"github.com/onix-fun/push-service/internal/provider"
	"github.com/onix-fun/push-service/internal/store"
	"github.com/onix-fun/push-service/internal/worker"
)

// @title Push Service API
// @version 1.0
// @description Service for managing device tokens and delivering push notifications via FCM, APNS, and Web Push.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@swagger.io

// @license.name MIT
// @license.url https://opensource.org/licenses/MIT

// @host localhost:8080
// @BasePath /

// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if len(os.Args) < 2 {
		log.Error("usage: push-service <serve|config>")
		os.Exit(2)
	}
	if os.Args[1] == "config" && (len(os.Args) < 3 || os.Args[2] != "validate") {
		log.Error("usage: push-service config validate --config=<path>")
		os.Exit(2)
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	path := fs.String("config", "config/config.example.yaml", "YAML config path")
	role := fs.String("role", "all", "api, worker or all")
	flagArgs := os.Args[2:]
	if os.Args[1] == "config" {
		flagArgs = os.Args[3:]
	}
	_ = fs.Parse(flagArgs)
	cfg, err := config.Load(*path)
	if err != nil {
		log.Error("invalid config", "error", err)
		os.Exit(1)
	}
	if os.Args[1] == "config" {
		log.Info("config is valid")
		return
	}
	if os.Args[1] != "serve" {
		log.Error("unknown command")
		os.Exit(2)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.Database.AutoMigrate {
		m, err := migrate.New(cfg.Database.MigrationPath, cfg.Database.URL)
		if err != nil {
			log.Error("migrator failed", "error", err)
			os.Exit(1)
		}
		if err = m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Error("migration failed", "error", err)
			os.Exit(1)
		}
	}

	s, err := store.Open(ctx, cfg.Database.URL)
	if err != nil {
		log.Error("database unavailable", "error", err)
		os.Exit(1)
	}
	defer s.Pool.Close()
	errs := make(chan error, 2)
	if *role == "all" || *role == "worker" {
		gateway, err := provider.New(ctx, cfg.Providers)
		if err != nil {
			log.Error("provider configuration failed", "error", err)
			os.Exit(1)
		}
		go func() { errs <- worker.New(cfg.RabbitMQ, s, gateway, log).Run(ctx) }()
	}
	var server *http.Server
	if *role == "all" || *role == "api" {
		mux := http.NewServeMux()
		mux.Handle("/", api.Handler(s, cfg.APIKeys))
		mux.HandleFunc("/livez", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(200) })
		mux.HandleFunc("/readyz", func(w http.ResponseWriter, r *http.Request) {
			if s.Pool.Ping(r.Context()) != nil {
				http.Error(w, "not ready", 503)
				return
			}
			w.WriteHeader(200)
		})
		mux.HandleFunc("/metrics", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("push_service_up 1\n")) })
		server = &http.Server{Addr: cfg.Service.HTTPAddr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
		go func() {
			err := server.ListenAndServe()
			if !errors.Is(err, http.ErrServerClosed) {
				errs <- err
			}
		}()
	}
	select {
	case <-ctx.Done():
	case err := <-errs:
		log.Error("service stopped", "error", err)
	}
	if server != nil {
		shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdown)
	}
}
