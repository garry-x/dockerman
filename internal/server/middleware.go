package server

import (
	"context"
	"net"
	"net/http"

	"dockerman/internal/docker"

	"github.com/gorilla/mux"
)

type contextKey string

const (
	ctxSourceType  contextKey = "source_type"
	ctxContainerID contextKey = "container_id"
)

type SourceType int

const (
	SourceLocalhost SourceType = iota
	SourceContainer
	SourceRemote
)

func ContainerAuthMiddleware(dockerCli *docker.Client) mux.MiddlewareFunc {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ip, _, _ := net.SplitHostPort(r.RemoteAddr)
			if ip == "" {
				ip = r.RemoteAddr
			}

			if ip == "127.0.0.1" || ip == "::1" {
				ctx := context.WithValue(r.Context(), ctxSourceType, SourceLocalhost)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			containerID, err := dockerCli.FindContainerByIP(r.Context(), ip)
			if err == nil && containerID != "" {
				ctx := context.WithValue(r.Context(), ctxSourceType, SourceContainer)
				ctx = context.WithValue(ctx, ctxContainerID, containerID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			ctx := context.WithValue(r.Context(), ctxSourceType, SourceRemote)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func GetSourceType(r *http.Request) SourceType {
	v, ok := r.Context().Value(ctxSourceType).(SourceType)
	if !ok {
		return SourceRemote
	}
	return v
}

func GetSourceContainerID(r *http.Request) string {
	v, _ := r.Context().Value(ctxContainerID).(string)
	return v
}

// RequireWriteAccess checks permissions at request time. Extracts {id} from mux.Vars(r).
func RequireWriteAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		src := GetSourceType(r)
		targetID := mux.Vars(r)["id"]

		switch src {
		case SourceLocalhost:
			next.ServeHTTP(w, r)
		case SourceContainer:
			myID := GetSourceContainerID(r)
			if targetID == myID || targetID == "" {
				next.ServeHTTP(w, r)
			} else {
				writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "container can only operate on itself"})
			}
		default:
			writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "write operations require localhost or container origin"})
		}
	})
}

// RequireWriteAllAccess blocks container and remote sources from all-container ops.
func RequireWriteAllAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		src := GetSourceType(r)
		switch src {
		case SourceLocalhost:
			next.ServeHTTP(w, r)
		case SourceContainer:
			writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "containers cannot operate on all containers"})
		default:
			writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "write operations require localhost or container origin"})
		}
	})
}

// RequireLocalhost blocks non-localhost sources.
func RequireLocalhost(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if GetSourceType(r) != SourceLocalhost {
			writeJSON(w, http.StatusForbidden, apiResponse{Success: false, Error: "this operation requires localhost"})
			return
		}
		next.ServeHTTP(w, r)
	})
}
