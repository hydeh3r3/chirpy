package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/hydeh3r3/chirpy/internal/database"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// apiConfig holds server state and metrics
type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
}

// chirpRequest represents the incoming JSON payload
type chirpRequest struct {
	Body string `json:"body"`
}

// chirpResponse represents the chirp data response
type chirpResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Body      string    `json:"body"`
	UserID    string    `json:"user_id"`
}

// errorResponse represents an error message response
type errorResponse struct {
	Error string `json:"error"`
}

// userRequest represents the incoming JSON payload
type userRequest struct {
	Email string `json:"email"`
}

// userResponse represents the user data response
type userResponse struct {
	ID        string    `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
}

// chirpCreateRequest represents the incoming JSON payload
type chirpCreateRequest struct {
	Body   string    `json:"body"`
	UserID uuid.UUID `json:"user_id"`
}

// List of profane words to filter
var profaneWords = []string{
	"kerfuffle",
	"sharbert",
	"fornax",
}

// middlewareMetricsInc increments the hit counter for each request
func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

// metricsHandler returns HTML with the current hit count
func (cfg *apiConfig) metricsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	html := `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`
	fmt.Fprintf(w, html, cfg.fileserverHits.Load())
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

// validateChirpHandler handles chirp validation and cleaning
func validateChirpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse{Error: "Failed to read request"})
		return
	}

	// Parse the JSON request
	var chirp chirpRequest
	err = json.Unmarshal(body, &chirp)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "Invalid JSON"})
		return
	}

	// Validate chirp length
	if len(chirp.Body) > 140 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "Chirp is too long"})
		return
	}

	// Clean the chirp text
	words := strings.Split(chirp.Body, " ")
	for i, word := range words {
		wordLower := strings.ToLower(word)
		for _, profane := range profaneWords {
			if wordLower == profane {
				words[i] = "****"
				break
			}
		}
	}
	cleanedChirp := strings.Join(words, " ")

	// Return cleaned chirp
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(chirpResponse{
		Body: cleanedChirp,
	})
}

// createUserHandler handles user creation requests
func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Read and parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse{Error: "Failed to read request"})
		return
	}

	var req userRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "Invalid JSON"})
		return
	}

	// Create user in database
	now := time.Now().UTC()
	user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Email:     req.Email,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse{Error: "Failed to create user"})
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(userResponse{
		ID:        user.ID.String(),
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	})
}

// createChirpHandler handles chirp creation requests
func (cfg *apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Read and parse request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse{Error: "Failed to read request"})
		return
	}

	var req chirpCreateRequest
	err = json.Unmarshal(body, &req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "Invalid JSON"})
		return
	}

	// Validate chirp length
	if len(req.Body) > 140 {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(errorResponse{Error: "Chirp is too long"})
		return
	}

	// Clean the chirp text
	words := strings.Split(req.Body, " ")
	for i, word := range words {
		wordLower := strings.ToLower(word)
		for _, profane := range profaneWords {
			if wordLower == profane {
				words[i] = "****"
				break
			}
		}
	}
	cleanedChirp := strings.Join(words, " ")

	// Create chirp in database
	now := time.Now().UTC()
	chirp, err := cfg.db.CreateChirp(r.Context(), database.CreateChirpParams{
		ID:        uuid.New(),
		CreatedAt: now,
		UpdatedAt: now,
		Body:      cleanedChirp,
		UserID:    req.UserID,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse{Error: "Failed to create chirp"})
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(chirpResponse{
		ID:        chirp.ID.String(),
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID.String(),
	})
}

// resetHandler resets the hit counter and deletes all users
func (cfg *apiConfig) resetHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Check if we're in dev mode
	if cfg.platform != "dev" {
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(errorResponse{Error: "Reset endpoint only available in dev mode"})
		return
	}

	// Reset hit counter
	cfg.fileserverHits.Store(0)

	// Delete all users
	err := cfg.db.DeleteAllUsers(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse{Error: "Failed to delete users"})
		return
	}

	w.WriteHeader(http.StatusOK)
}

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		panic(err)
	}

	// Get environment variables
	dbURL := os.Getenv("DB_URL")
	if dbURL == "" {
		panic("DB_URL environment variable is not set")
	}
	platform := os.Getenv("PLATFORM")
	if platform == "" {
		platform = "prod" // Default to prod for safety
	}

	// Open database connection
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(err)
	}
	defer db.Close()

	// Create database queries
	dbQueries := database.New(db)

	// Create API config
	apiCfg := &apiConfig{
		db:       dbQueries,
		platform: platform,
	}

	// Create a new ServeMux instance
	mux := http.NewServeMux()

	// Add API endpoints
	mux.HandleFunc("/api/healthz", healthzHandler)
	mux.HandleFunc("/api/users", apiCfg.createUserHandler)
	mux.HandleFunc("/api/chirps", apiCfg.createChirpHandler)

	// Add admin endpoints
	mux.HandleFunc("/admin/metrics", apiCfg.metricsHandler)
	mux.HandleFunc("/admin/reset", apiCfg.resetHandler)

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
	err = server.ListenAndServe()
	if err != nil {
		panic(err)
	}
}
