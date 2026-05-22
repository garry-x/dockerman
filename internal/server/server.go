package server

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"dockerman/internal/docker"
	"dockerman/internal/store"

	"github.com/gorilla/mux"
)

type Server struct {
	host      string
	port      int
	dbPath    string
	dockerCli *docker.Client
	store     *store.JSONStore
}

func NewServer(host string, port int, dbPath string) *Server {
	return &Server{
		host:   host,
		port:   port,
		dbPath: dbPath,
	}
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: "internal server error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) Start() error {
	dockerCli, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	s.dockerCli = dockerCli
	defer dockerCli.Close()

	s.store = store.NewJSONStore(s.dbPath)

	containers, err := s.store.Load()
	if err != nil {
		return fmt.Errorf("load store: %w", err)
	}
	if len(containers) == 0 {
		fmt.Println("Database is empty, running initial scan...")
		containers, err = s.dockerCli.ScanAll(context.Background())
		if err != nil {
			return fmt.Errorf("initial scan: %w", err)
		}
		if err := s.store.Save(containers); err != nil {
			return fmt.Errorf("initial scan save: %w", err)
		}
		fmt.Printf("Initial scan complete: %d container(s) found\n", len(containers))
	}

	r := mux.NewRouter()

	r.Use(recoveryMiddleware)
	r.Use(ContainerAuthMiddleware(s.dockerCli, os.Getenv("DOCKERMAN_AUTH_TOKEN")))

	handler := NewHandler(s.dockerCli, s.store)
	handler.RegisterRoutes(r)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		fmt.Printf("Docker Manager HTTP server listening on %s\n", addr)
		fmt.Printf("Database: %s\n", s.dbPath)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Printf("server error: %v\n", err)
		}
	}()

	<-ctx.Done()
	fmt.Println("\nShutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	fmt.Println("Server stopped.")
	return nil
}
