package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"io"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"sort"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

//go:embed web
var assets embed.FS

//go:embed go.mod
var moduleFile string

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "5003"
	}
	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	server := &http.Server{
		Addr: net.JoinHostPort(host, port), Handler: newHandler(newBank()),
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 15 * time.Second, IdleTimeout: 60 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	done := make(chan struct{})
	go func() {
		defer close(done)
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("Shutdown: %v", err)
		}
	}()
	log.Printf("Chainguard Go Bank running at http://%s (in-memory demo)", server.Addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
	<-done
}

func newHandler(bank *bankStore) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; img-src 'self'; font-src 'self'; base-uri 'none'; frame-ancestors 'none'; form-action 'self'")
			next.ServeHTTP(w, r)
		})
	})
	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "healthy"})
	})
	r.Route("/api", func(r chi.Router) {
		r.Use(middleware.NoCache)
		r.Get("/dashboard", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, bank.snapshot())
		})
		r.Post("/transfers", func(w http.ResponseWriter, r *http.Request) {
			mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || mediaType != "application/json" {
				writeError(w, http.StatusUnsupportedMediaType, "Use application/json.")
				return
			}
			body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 4096))
			if err != nil {
				writeError(w, http.StatusBadRequest, "Enter a valid transfer.")
				return
			}
			// Record a redacted audit copy of the raw request. Audit writes are
			// fire-and-forget so logging never blocks or fails a transfer.
			go recordTransferAudit(body)
			var input transferInput
			decoder := json.NewDecoder(bytes.NewReader(body))
			// transferInput.UnmarshalJSON reads fields and rejects unknown fields.
			if err := decoder.Decode(&input); err != nil {
				writeError(w, http.StatusBadRequest, "Enter a valid transfer.")
				return
			}
			if err := decoder.Decode(&struct{}{}); err != io.EOF {
				writeError(w, http.StatusBadRequest, "Send a single transfer.")
				return
			}
			transaction, err := bank.transfer(input)
			if err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeJSON(w, http.StatusCreated, transaction)
		})
		r.Get("/dependencies", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, buildDependencies())
		})
		r.Get("/go-mod", func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, http.StatusOK, map[string]string{"content": moduleFile})
		})
	})
	public, err := fs.Sub(assets, "web")
	if err != nil {
		panic(err)
	}
	r.Handle("/*", http.FileServer(http.FS(public)))
	return r
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		log.Printf("Encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

type dependency struct {
	Path    string        `json:"path"`
	Version string        `json:"version"`
	Sum     string        `json:"sum"`
	Purpose string        `json:"purpose"`
	Replace *debug.Module `json:"replace,omitempty"`
}

func buildDependencies() any {
	info, ok := debug.ReadBuildInfo()
	result := struct {
		GoVersion string       `json:"goVersion"`
		Modules   []dependency `json:"modules"`
		Note      string       `json:"note"`
	}{Modules: []dependency{},
		Note: "Module information comes from this running binary. It does not verify the download source or prove who built the modules."}
	if !ok {
		return result
	}
	result.GoVersion = info.GoVersion
	purposes := map[string]string{
		"github.com/buger/jsonparser":   "Reads transfer request fields and redacts credentials from the audit log.",
		"github.com/go-chi/chi/v5":      "Routes the banking API and serves this application.",
		"github.com/google/uuid":        "Generates a unique ID for each account transfer.",
		"github.com/shopspring/decimal": "Calculates balances and transfers with exact decimal arithmetic.",
	}
	for _, module := range info.Deps {
		result.Modules = append(result.Modules, dependency{module.Path, module.Version, module.Sum, purposes[module.Path], module.Replace})
	}
	sort.Slice(result.Modules, func(i, j int) bool { return result.Modules[i].Path < result.Modules[j].Path })
	return result
}
