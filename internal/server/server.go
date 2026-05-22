package server

import (
	"fmt"
	"net/http"

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

func (s *Server) Start() error {
	dockerCli, err := docker.NewClient()
	if err != nil {
		return fmt.Errorf("create docker client: %w", err)
	}
	s.dockerCli = dockerCli
	s.store = store.NewJSONStore(s.dbPath)

	r := mux.NewRouter()

	// Auth middleware identifies request source (localhost/container/remote)
	r.Use(ContainerAuthMiddleware(s.dockerCli))

	handler := NewHandler(s.dockerCli, s.store)
	handler.RegisterRoutes(r)

	addr := fmt.Sprintf("%s:%d", s.host, s.port)
	fmt.Printf("Docker Manager HTTP server listening on %s\n", addr)
	fmt.Printf("Database: %s\n", s.dbPath)
	return http.ListenAndServe(addr, r)
}
