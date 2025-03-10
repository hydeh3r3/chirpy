package main

import (
	"fmt"
	"net/http"
	"sync/atomic"
)

// apiConfig holds server state and metrics
type apiConfig struct {
	fileserverHits atomic.Int32
}

// middlewareMetricsInc increments the hit counter for each request
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// metricsHandler returns the current hit count
func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, "Hits: %d", cfg.fileserverHits.Load())
}

// resetHandler resets the hit counter to zero
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
}

// healthzHandler handles health check requests
func healthzHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func main() {
	// Create API config
	apiCfg := &apiConfig{}

	// Create a new ServeMux instance
	mux := http.NewServeMux()

	// Add health check endpoint
	mux.HandleFunc("/healthz", healthzHandler)

	// Add metrics endpoints
	mux.HandleFunc("/metrics", apiCfg.metricsHandler)
	mux.HandleFunc("/reset", apiCfg.resetHandler)

	// Add fileserver handler with /app prefix and metrics middleware
	fileServer := http.FileServer(http.Dir("."))
	handler := http.StripPrefix("/app/", fileServer)
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(handler))

	// Create a new http.Server with the mux as handler
	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	// Start the server
	err := server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
