package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/bamcmanus/Chirpy/internal/auth"
	"github.com/bamcmanus/Chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type errResponse struct {
    Error string `json:"error"`
}

type newChirpResponse struct {
    Body string `json:"body"`
    Id uuid.UUID `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    UserId uuid.UUID `json:"user_id"`
}

type newUserResponse struct {
    Id uuid.UUID `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Email string `json:"email"`
}


type apiConfig struct {
    fileserverHits atomic.Int32
    platform string
    jwtSecret string
}

func (c *apiConfig) middlewareMetricsInt(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        c.fileserverHits.Add(1)
        next.ServeHTTP(w, req)
    })
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) error {
    response, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    w.Write(response)
    return nil
}

func respondWithError(w http.ResponseWriter, code int, msg string) error {
    return respondWithJSON(w, code, errResponse{Error: msg})
}

func cleanseWords(body string) string {
    profaneWords := map[string]struct{}{ "kerfuffle": {}, "sharbert": {}, "fornax": {}}
    lowerBody := strings.ToLower(body)
    words := strings.Split(body, " ")
    lowerWords := strings.Split(lowerBody, " ")

    for i, word := range lowerWords {
        if _, ok := profaneWords[word]; ok {
            words[i] = "****"
        }
    }

    return strings.Join(words, " ")
}

func main() {
    godotenv.Load()
    dbUrl := os.Getenv("DB_URL")
    db, err := sql.Open("postgres", dbUrl)
    if err != nil {
        log.Fatalf("failed to connect to database; err: %s", err)
    }

    dbQueries := database.New(db)
    
    var cfg apiConfig
    cfg.platform = os.Getenv("PLATFORM")
    if cfg.platform == "" {
        log.Fatal("PLATFORM not set")
    }
    
    cfg.jwtSecret = os.Getenv("JWT_SECRET")
    if cfg.jwtSecret == "" {
        log.Fatal("JWT_SECRET was not set")
    }

    mux := http.NewServeMux()

    mux.HandleFunc("POST /api/login", func(w http.ResponseWriter, req *http.Request) {
        type login struct {
            Password string `json:"password"`
            Email string `json:"email"`
        }

        var loginRequest login
        decoder := json.NewDecoder(req.Body)
        if err := decoder.Decode(&loginRequest); err != nil {
            log .Printf("failed to decode request; error: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "failed to decode request")
            return
        }

        user, err := dbQueries.GetUserByEmail(context.Background(), loginRequest.Email)
        if err != nil {
            log.Printf("failed to get user; error: %s", err)
            _ = respondWithError(w, http.StatusUnauthorized, "Incorrect email or Password")
            return
        }

        err = auth.CheckPasswordHash(user.HashedPassword, loginRequest.Password)
        if err != nil {
            log.Printf("passowrds do not match; error: %s", err)
            _ = respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
            return
        }

        token, err := auth.MakeJWT(user.ID, cfg.jwtSecret, time.Duration(3600) * time.Second)
        if err != nil {
            log.Printf("could not create JWT; error: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "failed to create JWT")
            return
        }

        refreshToken, err := auth.MakeRefreshToken()
        if err != nil {
            log.Printf("could not create refresh token; error: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "failed to create refresh token")
            return
        }

        params := database.CreateRefreshTokenParams{
            Token: refreshToken,
            UserID: user.ID,
        }
        _, err = dbQueries.CreateRefreshToken(context.Background(), params)
        if err != nil {
            log.Printf("could not perisste refresh token; error: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "refresh token creation failed")
            return
        }

        userReponse := struct{
            Id uuid.UUID `json:"id"`
            CreatedAt time.Time `json:"created_at"`
            UpdatedAt time.Time `json:"updated_at"`
            Email string `json:"email"`
            Token string `json:"token"`
            RefreshToken string `json:"refresh_token"`
        }{
            Id: user.ID,
            CreatedAt: user.CreatedAt,
            UpdatedAt: user.UpdatedAt,
            Email: user.Email,
            Token: token,
            RefreshToken: refreshToken,
        }
        if err := respondWithJSON(w, http.StatusOK, userReponse); err != nil {
            log.Printf("failed to respond; error: %s", err)
        }
    })

    mux.HandleFunc("POST /api/refresh", func(w http.ResponseWriter, req *http.Request) {
        refreshToken, err := auth.GetBearerToken(req.Header)
        if err != nil {
            log.Printf("failed to fetch Bearer token; error: %s", err)
            _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
            return
        }

        token, err :=dbQueries.GetRefreshToken(req.Context(), refreshToken)
        if err != nil {
            log.Printf("error fetching refresh token; error: %s", err)
            _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
            return
        }

        if token.RevokedAt.Valid {
            log.Printf("token is expired; expiration time: %v", token.RevokedAt)
            _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
            return
        }

        jwt, err := auth.MakeJWT(token.UserID, cfg.jwtSecret, time.Duration(3600) * time.Second)
        if err != nil {
            log.Printf("could not create JWT; error: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "failed to create JWT")
            return
        }

        res := struct{
            Token string `json:"token"`
        }{
            Token: jwt,
        }
        if err := respondWithJSON(w, http.StatusOK, res); err != nil {
            log.Printf("failed to respond; error: %s", err)
        }
    })

    mux.HandleFunc("POST /api/revoke", func(w http.ResponseWriter, req *http.Request) {
        refreshToken, err := auth.GetBearerToken(req.Header)
        if err != nil {
            log.Printf("failed to fetch Bearer token; error: %s", err)
            _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
            return
        }

        _, err = dbQueries.RevokeRefreshToken(req.Context(), refreshToken)
        if err != nil {
            log.Printf("failed to revoke token; error: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "")
        }

        w.WriteHeader(http.StatusNoContent) 
    })

    mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, req *http.Request) {
        type newUserRequest struct {
            Email string `json:"email"`
            Password string `json:"password"`
        }

        var newUserReq newUserRequest
        decoder := json.NewDecoder(req.Body)
        if err := decoder.Decode(&newUserReq); err != nil {
            log.Printf("failed to decode request body; error: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "could not decode new user request")
            return
        }

        if newUserReq.Password == "" {
            _ = respondWithError(w, http.StatusBadRequest, "password required")
            return
        }

        hashedPassword, err := auth.HashPassword(newUserReq.Password)
        if err != nil {
            log.Printf("failed to hash password; error: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "failed hashing password")
            return
        }

        params := database.CreateUserParams{Email: newUserReq.Email, HashedPassword: hashedPassword}
        user, err := dbQueries.CreateUser(context.Background(), params)
        if err != nil {
            _ = respondWithError(w, http.StatusInternalServerError, "failed to create user")
            return
        }

        res := newUserResponse {
            Email: user.Email,
            CreatedAt: user.CreatedAt,
            UpdatedAt: user.UpdatedAt,
            Id: user.ID,
        }

        _ = respondWithJSON(w, http.StatusCreated, res)
    })

    mux.Handle("/app/", cfg.middlewareMetricsInt(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))

    mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
        w.Header().Add("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusOK)
        fmt.Fprintf(w, "OK")
    })

    mux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, req *http.Request) {
        type newChirpRequest struct {
            Body string `json:"body"`
        }

        token, err := auth.GetBearerToken(req.Header)
        if err != nil {
            log.Printf("failed to fetch Bearer token; error: %s", err)
            _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
            return
        }

        userId, err := auth.ValidateJWT(token, cfg.jwtSecret)
        if err != nil  {
            log.Printf("JWT validation failed; error: %s", err)
            _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
            return
        }
        
        decoder := json.NewDecoder(req.Body)
        var params newChirpRequest
        if err := decoder.Decode(&params); err != nil {
            log.Printf("error decoding prameters: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "Something went wrong")
            return
        }

        if len(params.Body) > 140 {
            _ = respondWithError(w, http.StatusBadRequest, "Chirp is too long")
            return
        }

        body := cleanseWords(params.Body)

        cParams := database.CreateChirpParams {
            Body: body,
            UserID: userId,
        }

        chirp, err := dbQueries.CreateChirp(context.Background(), cParams)
        if err != nil {
            log.Printf("error creating chirp; err: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "failed to create chirp")
            return
        }

        resp := newChirpResponse{
            Id: chirp.ID,
            UserId: chirp.UserID,
            CreatedAt: chirp.CreatedAt,
            UpdatedAt: chirp.UpdatedAt,
            Body: chirp.Body,
        }
        err = respondWithJSON(w, http.StatusCreated, resp)
        if err != nil {
            log.Fatal("could not marshal response")
        }
    })

    mux.HandleFunc("GET /api/chirps", func(w http.ResponseWriter, req *http.Request) {
        chirps, err := dbQueries.ListChirps(context.Background())        
        if err != nil {
            log.Printf("error fetching chirps: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "failed to fetch chirps")
            return
        }

        var chirpResponses []newChirpResponse
        for _, chirp := range chirps {
            chirpResponse := newChirpResponse{
                Id: chirp.ID,
                CreatedAt: chirp.CreatedAt,
                UpdatedAt: chirp.UpdatedAt,
                Body: chirp.Body,
                UserId: chirp.UserID,
            }
            chirpResponses = append(chirpResponses, chirpResponse)
        }
        _ = respondWithJSON(w, http.StatusOK, chirpResponses)
    })

    mux.HandleFunc("GET /admin/metrics", func(w http.ResponseWriter, req *http.Request) {
        w.Header().Add("Content-Type", "text/html")
        template := `<html>
  <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
  </body>
</html>`
        htmlContent := fmt.Sprintf(template, cfg.fileserverHits.Load())
        fmt.Fprint(w, htmlContent)
    })

    mux.HandleFunc("GET /api/chirps/{chirpID}", func(w http.ResponseWriter, req *http.Request) {
        chirpId := req.PathValue("chirpID")
        log.Printf("received chirp ID: %s", chirpId)
        id := uuid.MustParse(chirpId)
        chirp, err := dbQueries.GetChirp(context.Background(), id)
        if err != nil {
            log.Printf("error fetching chirp; err: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "failed to get chirps")
            return
        }
        log.Printf("chirp: %+v", chirp)

        if chirp.ID.String() == "" {
            _ = respondWithError(w, http.StatusNotFound, "chirp not found")
            return
        }

        resp := newChirpResponse{
            Id: chirp.ID,
            CreatedAt: chirp.CreatedAt,
            UpdatedAt: chirp.UpdatedAt,
            Body: chirp.Body,
            UserId: chirp.UserID,
        }
        _ = respondWithJSON(w, http.StatusOK, resp)
    })

    mux.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, req *http.Request) {
        if cfg.platform != "dev" {
            w.WriteHeader(http.StatusForbidden)
            return
        }

        if err := dbQueries.DeleteUsers(context.Background()); err != nil {
            log.Printf("error deleting users; err: %s", err)
            _ = respondWithError(w, http.StatusInternalServerError, "error deleting users")
            return
        }

        cfg.fileserverHits.Store(0)
        w.Header().Add("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusOK)
    })

    server := http.Server{
        Addr: ":8080",
        Handler: mux,
    }
    defer server.Close()

    log.Println("Starting server ...")
    if err := server.ListenAndServe(); err != nil {
        log.Fatalf("failed to start server: %s", err)
    }
}
