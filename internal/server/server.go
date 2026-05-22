package server

import (
	"context"
	"fmt"
	"net/http"
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

	// Recovery middleware must be first to catch panics in all handlers
	r.Use(recoveryMiddleware)
	r.Use(ContainerAuthMiddleware(s.dockerCli))

	handler := NewHandler(s.dockerCli, s.store)
	handler.RegisterRoutes(r)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	fmt.Printf("Docker Manager HTTP server listening on %s\n", addr)
	fmt.Printf("Database: %s\n", s.dbPath)

	srv := &http.Server{
		Addr:              addr,
		Handler:           r,
		ReadTimeout:       10 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       30 * time.Second,
	}
	return srv.ListenAndServe()
}
