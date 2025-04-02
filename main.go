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

type apiConfig struct {
    fileserverHits atomic.Int32
    platform string
}

func (c *apiConfig) middlewareMetricsInt(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        c.fileserverHits.Add(1)
        next.ServeHTTP(w, req)
    })
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) error {
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
    platform := os.Getenv("PLATFORM")
    cfg.platform = platform
    mux := http.NewServeMux()

    mux.HandleFunc("POST /api/users", func(w http.ResponseWriter, req *http.Request) {
        type newUserRequest struct {
            Email string `json:"email"`
            Password string `json:"password"`
        }

        type newUserResponse struct {
            Id uuid.UUID `json:"id"`
            CreatedAt time.Time `json:"created_at"`
            UpdatedAt time.Time `json:"updated_at"`
            Email string `json:"email"`
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
            UserId uuid.UUID `json:"user_id"`
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
            UserID: params.UserId,
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
