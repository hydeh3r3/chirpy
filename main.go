package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"sort"
	"strings"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/hydeh3r3/chirpy/internal/auth"
	"github.com/hydeh3r3/chirpy/internal/database"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

// apiConfig holds server state and metrics
type apiConfig struct {
	fileserverHits atomic.Int32
	db             *database.Queries
	platform       string
	jwtSecret      string
	polkaKey       string
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
	Email    string `json:"email"`
	Password string `json:"password"`
}

// userResponse represents the user data response
type userResponse struct {
	ID          string    `json:"id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Email       string    `json:"email"`
	IsChirpyRed bool      `json:"is_chirpy_red"`
}

// chirpCreateRequest represents the incoming JSON payload
type chirpCreateRequest struct {
	Body string `json:"body"`
}

// loginRequest represents the login request body
type loginRequest struct {
	Email            string `json:"email"`
	Password         string `json:"password"`
	ExpiresInSeconds *int   `json:"expires_in_seconds,omitempty"`
}

// loginResponse represents the login response body
type loginResponse struct {
	ID           string `json:"id"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
	Email        string `json:"email"`
	Token        string `json:"token"`
	RefreshToken string `json:"refresh_token"`
	IsChirpyRed  bool   `json:"is_chirpy_red"`
}

// refreshResponse represents the refresh token response body
type refreshResponse struct {
	Token string `json:"token"`
}

// updateUserRequest represents the user update request body
type updateUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// webhookRequest represents the Polka webhook request body
type webhookRequest struct {
	Event string `json:"event"`
	Data  struct {
		UserID string `json:"user_id"`
	} `json:"data"`
}

// List of profane words to filter
var profaneWords = []string{
	"kerfuffle",
	"sharbert",
	"fornax",
}

// respondWithError sends an error response
func respondWithError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(errorResponse{Error: msg})
}

// respondWithJSON sends a JSON response
func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
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

// createUserHandler handles user creation requests
func (cfg *apiConfig) createUserHandler(w http.ResponseWriter, r *http.Request) {
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

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse{Error: "Failed to hash password"})
		return
	}

	// Create user in database
	now := time.Now().UTC()
	user, err := cfg.db.CreateUser(r.Context(), database.CreateUserParams{
		ID:             uuid.New(),
		CreatedAt:      now,
		UpdatedAt:      now,
		Email:          req.Email,
		HashedPassword: hashedPassword,
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
		ID:          user.ID.String(),
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	})
}

// loginHandler handles user login requests
func (cfg *apiConfig) loginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	user, err := cfg.db.GetUserByEmail(r.Context(), req.Email)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	if err := auth.CheckPasswordHash(req.Password, user.HashedPassword); err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	// Generate access token (1 hour expiration)
	accessToken, err := auth.MakeJWT(user.ID, cfg.jwtSecret, time.Hour)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating token")
		return
	}

	// Generate refresh token
	refreshToken, err := auth.MakeRefreshToken()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating refresh token")
		return
	}

	// Store refresh token in database (60 days expiration)
	now := time.Now()
	_, err = cfg.db.CreateRefreshToken(r.Context(), database.CreateRefreshTokenParams{
		Token:     refreshToken,
		CreatedAt: now,
		UserID:    user.ID,
		ExpiresAt: now.AddDate(0, 0, 60), // 60 days
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error storing refresh token")
		return
	}

	respondWithJSON(w, http.StatusOK, loginResponse{
		ID:           user.ID.String(),
		CreatedAt:    user.CreatedAt.Format(time.RFC3339),
		UpdatedAt:    user.UpdatedAt.Format(time.RFC3339),
		Email:        user.Email,
		Token:        accessToken,
		RefreshToken: refreshToken,
		IsChirpyRed:  user.IsChirpyRed,
	})
}

// createChirpHandler handles chirp creation requests
func (cfg *apiConfig) createChirpHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Get and validate JWT token
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errorResponse{Error: "Missing or invalid token"})
		return
	}

	// Validate token and get user ID
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(errorResponse{Error: "Invalid token"})
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
		UserID:    userID,
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

// getAllChirpsHandler handles requests to get all chirps
func (cfg *apiConfig) getAllChirpsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Get author_id from query parameters if it exists
	authorIDStr := r.URL.Query().Get("author_id")
	var chirps []database.Chirp
	var err error

	if authorIDStr != "" {
		// Parse author ID
		authorID, err := uuid.Parse(authorIDStr)
		if err != nil {
			respondWithError(w, http.StatusBadRequest, "Invalid author ID")
			return
		}

		// Get chirps by author
		chirps, err = cfg.db.GetChirpsByAuthorID(r.Context(), authorID)
	} else {
		// Get all chirps
		chirps, err = cfg.db.GetAllChirps(r.Context())
	}

	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to get chirps")
		return
	}

	// Sort chirps by created_at based on sort parameter
	sortParam := r.URL.Query().Get("sort")
	if sortParam == "desc" {
		sort.Slice(chirps, func(i, j int) bool {
			return chirps[i].CreatedAt.After(chirps[j].CreatedAt)
		})
	} else {
		// Default to ascending order
		sort.Slice(chirps, func(i, j int) bool {
			return chirps[i].CreatedAt.Before(chirps[j].CreatedAt)
		})
	}

	// Convert to response format
	response := make([]chirpResponse, len(chirps))
	for i, chirp := range chirps {
		response[i] = chirpResponse{
			ID:        chirp.ID.String(),
			CreatedAt: chirp.CreatedAt,
			UpdatedAt: chirp.UpdatedAt,
			Body:      chirp.Body,
			UserID:    chirp.UserID.String(),
		}
	}

	respondWithJSON(w, http.StatusOK, response)
}

// getChirpByIDHandler handles requests to get a single chirp
func (cfg *apiConfig) getChirpByIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Get chirp ID from path parameter
	chirpIDStr := r.PathValue("chirpID")
	if chirpIDStr == "" {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Convert string ID to UUID
	chirpID, err := uuid.Parse(chirpIDStr)
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	// Get chirp from database
	chirp, err := cfg.db.GetChirpByID(r.Context(), chirpID)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(errorResponse{Error: "Failed to get chirp"})
		return
	}

	// Return response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(chirpResponse{
		ID:        chirp.ID.String(),
		CreatedAt: chirp.CreatedAt,
		UpdatedAt: chirp.UpdatedAt,
		Body:      chirp.Body,
		UserID:    chirp.UserID.String(),
	})
}

// deleteChirpHandler handles chirp deletion requests
func (cfg *apiConfig) deleteChirpHandler(w http.ResponseWriter, r *http.Request) {
	// Get and validate JWT token
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Missing or invalid token")
		return
	}

	// Validate token and get user ID
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	// Get chirp ID from path parameter
	chirpIDStr := r.PathValue("chirpID")
	if chirpIDStr == "" {
		respondWithError(w, http.StatusNotFound, "Chirp ID not provided")
		return
	}

	// Convert string ID to UUID
	chirpID, err := uuid.Parse(chirpIDStr)
	if err != nil {
		respondWithError(w, http.StatusNotFound, "Invalid chirp ID")
		return
	}

	// Get chirp from database to check ownership
	chirp, err := cfg.db.GetChirpByID(r.Context(), chirpID)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusNotFound, "Chirp not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to get chirp")
		return
	}

	// Check if user owns the chirp
	if chirp.UserID != userID {
		respondWithError(w, http.StatusForbidden, "Not authorized to delete this chirp")
		return
	}

	// Delete the chirp
	err = cfg.db.DeleteChirp(r.Context(), chirpID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to delete chirp")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// chirpsHandler handles GET, POST, and DELETE requests for chirps
func (cfg *apiConfig) chirpsHandler(w http.ResponseWriter, r *http.Request) {
	// Check if this is a request for a specific chirp
	if chirpID := r.PathValue("chirpID"); chirpID != "" {
		switch r.Method {
		case http.MethodGet:
			cfg.getChirpByIDHandler(w, r)
		case http.MethodDelete:
			cfg.deleteChirpHandler(w, r)
		default:
			respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		}
		return
	}

	// Handle collection endpoints
	switch r.Method {
	case http.MethodGet:
		cfg.getAllChirpsHandler(w, r)
	case http.MethodPost:
		cfg.createChirpHandler(w, r)
	default:
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (cfg *apiConfig) refreshHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	user, err := cfg.db.GetUserFromRefreshToken(r.Context(), token)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid or expired refresh token")
		return
	}

	// Generate new access token (1 hour expiration)
	accessToken, err := auth.MakeJWT(user.ID, cfg.jwtSecret, time.Hour)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error generating token")
		return
	}

	respondWithJSON(w, http.StatusOK, refreshResponse{
		Token: accessToken,
	})
}

func (cfg *apiConfig) revokeHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	err = cfg.db.RevokeRefreshToken(r.Context(), database.RevokeRefreshTokenParams{
		RevokedAt: sql.NullTime{Time: time.Now(), Valid: true},
		Token:     token,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Error revoking token")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// usersHandler handles both POST and PUT requests for users
func (cfg *apiConfig) usersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodPost:
		cfg.createUserHandler(w, r)
	case http.MethodPut:
		cfg.updateUserHandler(w, r)
	default:
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

func (cfg *apiConfig) updateUserHandler(w http.ResponseWriter, r *http.Request) {
	// Get and validate JWT token
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Missing or invalid token")
		return
	}

	// Validate token and get user ID
	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid token")
		return
	}

	var req updateUserRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Hash password
	hashedPassword, err := auth.HashPassword(req.Password)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	// Update user in database
	now := time.Now()
	user, err := cfg.db.UpdateUser(r.Context(), database.UpdateUserParams{
		ID:             userID,
		Email:          req.Email,
		HashedPassword: hashedPassword,
		UpdatedAt:      now,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to update user")
		return
	}

	respondWithJSON(w, http.StatusOK, userResponse{
		ID:          user.ID.String(),
		CreatedAt:   user.CreatedAt,
		UpdatedAt:   user.UpdatedAt,
		Email:       user.Email,
		IsChirpyRed: user.IsChirpyRed,
	})
}

// webhookHandler handles Polka webhook requests
func (cfg *apiConfig) webhookHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondWithError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Validate API key
	apiKey, err := auth.GetAPIKey(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Invalid API key")
		return
	}

	if apiKey != cfg.polkaKey {
		respondWithError(w, http.StatusUnauthorized, "Invalid API key")
		return
	}

	var req webhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Only handle user.upgraded events
	if req.Event != "user.upgraded" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	// Parse user ID
	userID, err := uuid.Parse(req.Data.UserID)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid user ID")
		return
	}

	// Check if user exists
	_, err = cfg.db.GetUserByID(r.Context(), userID)
	if err != nil {
		if err == sql.ErrNoRows {
			respondWithError(w, http.StatusNotFound, "User not found")
			return
		}
		respondWithError(w, http.StatusInternalServerError, "Failed to get user")
		return
	}

	// Upgrade user to Chirpy Red
	err = cfg.db.UpgradeUserToChirpyRed(r.Context(), userID)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Failed to upgrade user")
		return
	}

	w.WriteHeader(http.StatusNoContent)
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
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		panic("JWT_SECRET environment variable is not set")
	}
	polkaKey := os.Getenv("POLKA_KEY")
	if polkaKey == "" {
		panic("POLKA_KEY environment variable is not set")
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
		db:        dbQueries,
		platform:  platform,
		jwtSecret: jwtSecret,
		polkaKey:  polkaKey,
	}

	// Create a new ServeMux instance
	mux := http.NewServeMux()

	// Add API endpoints
	mux.HandleFunc("/api/healthz", healthzHandler)
	mux.HandleFunc("/api/users", apiCfg.usersHandler)
	mux.HandleFunc("/api/login", apiCfg.loginHandler)
	mux.HandleFunc("/api/chirps", apiCfg.chirpsHandler)
	mux.HandleFunc("/api/chirps/{chirpID}", apiCfg.chirpsHandler)
	mux.HandleFunc("/api/refresh", apiCfg.refreshHandler)
	mux.HandleFunc("/api/revoke", apiCfg.revokeHandler)
	mux.HandleFunc("/api/polka/webhooks", apiCfg.webhookHandler)

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
