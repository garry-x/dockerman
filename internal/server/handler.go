package server

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"dockerman/internal/docker"
	"dockerman/internal/model"
	"dockerman/internal/store"

	"github.com/gorilla/mux"
)

type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

type Handler struct {
	dockerCli *docker.Client
	store     *store.JSONStore
}

func NewHandler(dockerCli *docker.Client, s *store.JSONStore) *Handler {
	return &Handler{dockerCli: dockerCli, store: s}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("writeJSON encode error: %v", err)
	}
}

func (h *Handler) RegisterRoutes(r *mux.Router) {
	// Health — open to all
	r.HandleFunc("/api/v1/health", h.Health).Methods("GET")

	// Read endpoints — open to all sources
	r.HandleFunc("/api/v1/containers", h.ListContainers).Methods("GET")
	r.HandleFunc("/api/v1/containers/{id}", h.GetContainer).Methods("GET")
	r.HandleFunc("/api/v1/containers/{id}/info", h.InfoContainer).Methods("GET")

	// Scan — localhost only
	r.Handle("/api/v1/containers/scan", RequireLocalhost(http.HandlerFunc(h.Scan))).Methods("POST")

	// Single-container write ops
	r.Handle("/api/v1/containers/{id}/start", RequireWriteAccess(http.HandlerFunc(h.StartContainer))).Methods("POST")
	r.Handle("/api/v1/containers/{id}/stop", RequireWriteAccess(http.HandlerFunc(h.StopContainer))).Methods("POST")
	r.Handle("/api/v1/containers/{id}/restart", RequireWriteAccess(http.HandlerFunc(h.RestartContainer))).Methods("POST")
	r.Handle("/api/v1/containers/{id}", RequireWriteAccess(http.HandlerFunc(h.RemoveContainer))).Methods("DELETE")

	// All-container ops — localhost only
	r.Handle("/api/v1/containers/start-all", RequireWriteAllAccess(http.HandlerFunc(h.StartAll))).Methods("POST")
	r.Handle("/api/v1/containers/stop-all", RequireWriteAllAccess(http.HandlerFunc(h.StopAll))).Methods("POST")
	r.Handle("/api/v1/containers/all", RequireWriteAllAccess(http.HandlerFunc(h.RemoveAll))).Methods("DELETE")

	// Exec
	r.Handle("/api/v1/exec/{id}", RequireWriteAccess(http.HandlerFunc(h.ExecContainer))).Methods("POST")
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	if err := h.dockerCli.Ping(r.Context()); err != nil {
		writeJSON(w, http.StatusServiceUnavailable, apiResponse{Success: false, Error: "docker daemon unreachable"})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: "ok"})
}

func (h *Handler) Scan(w http.ResponseWriter, r *http.Request) {
	containers, err := h.dockerCli.ScanAll(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	if err := h.store.Save(containers); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: containers})
}

func (h *Handler) ListContainers(w http.ResponseWriter, r *http.Request) {
	containers, err := h.store.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}

	src := GetSourceType(r)
	if src == SourceContainer {
		myID := GetSourceContainerID(r)
		myName := GetSourceContainerName(r)
		filtered := make([]model.ContainerInfo, 0)
		for _, c := range containers {
			if c.ID == myID || c.Name == myID || c.Name == myName {
				filtered = append(filtered, c)
			}
		}
		writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: filtered})
		return
	}

	// Unauthenticated localhost (host-network containers without token)
	if src == SourceRemote && isLocalhostIP(r) {
		writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "access denied"})
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: containers})
}

func (h *Handler) GetContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	if !h.authorizeContainerRead(r, id) {
		writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "access denied"})
		return
	}

	ctr, err := h.store.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: ctr})
}

func (h *Handler) InfoContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	if !h.authorizeContainerRead(r, id) {
		writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "access denied"})
		return
	}

	ctr, err := h.store.GetByID(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: ctr})
}

func (h *Handler) authorizeContainerRead(r *http.Request, id string) bool {
	src := GetSourceType(r)
	switch src {
	case SourceLocalhost:
		return true
	case SourceContainer:
		return isSelfContainer(r, id)
	default:
		return !isLocalhostIP(r)
	}
}

func isLocalhostIP(r *http.Request) bool {
	ip := GetRemoteIP(r)
	return ip == "127.0.0.1" || ip == "::1"
}

func (h *Handler) StartContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.dockerCli.Start(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: "started"})
}

func (h *Handler) StopContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.dockerCli.Stop(r.Context(), id); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: "stopped"})
}

func (h *Handler) RestartContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	// Use background context so Docker operations survive client disconnect
	// (e.g. container stopping itself breaks the HTTP connection)
	bgCtx := context.Background()
	if err := h.dockerCli.Stop(bgCtx, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	if err := h.dockerCli.Start(bgCtx, id); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: "restarted"})
}

func (h *Handler) RemoveContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := h.dockerCli.Remove(r.Context(), id, false); err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: "removed"})
}

func (h *Handler) StartAll(w http.ResponseWriter, r *http.Request) {
	containers, err := h.store.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	results := make([]string, 0)
	for _, c := range containers {
		if err := h.dockerCli.Start(r.Context(), c.ID); err != nil {
			results = append(results, "failed: "+c.ID+" - "+err.Error())
		} else {
			results = append(results, "started: "+c.ID)
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: results})
}

func (h *Handler) StopAll(w http.ResponseWriter, r *http.Request) {
	containers, err := h.store.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	results := make([]string, 0)
	for _, c := range containers {
		if err := h.dockerCli.Stop(r.Context(), c.ID); err != nil {
			results = append(results, "failed: "+c.ID+" - "+err.Error())
		} else {
			results = append(results, "stopped: "+c.ID)
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: results})
}

func (h *Handler) RemoveAll(w http.ResponseWriter, r *http.Request) {
	containers, err := h.store.Load()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	results := make([]string, 0)
	for _, c := range containers {
		if err := h.dockerCli.Remove(r.Context(), c.ID, false); err != nil {
			results = append(results, "failed: "+c.ID+" - "+err.Error())
		} else {
			results = append(results, "removed: "+c.ID)
		}
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: results})
}

func (h *Handler) ExecContainer(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	var body struct {
		Command []string `json:"command"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || len(body.Command) == 0 {
		writeJSON(w, http.StatusBadRequest, apiResponse{Success: false, Error: "missing command in request body"})
		return
	}
	output, err := h.dockerCli.Exec(r.Context(), id, body.Command)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Success: false, Error: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, apiResponse{Success: true, Data: output})
}
