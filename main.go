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

	"github.com/bamcmanus/Chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)


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
            log.Println("could not decode new user request")
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        user, err := dbQueries.CreateUser(context.Background(), newUserReq.Email)
        if err != nil {
            log.Printf("failed to create user; err: %s", err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        res := newUserResponse {
            Email: user.Email,
            CreatedAt: user.CreatedAt,
            UpdatedAt: user.UpdatedAt,
            Id: user.ID,
        }

        jsonData, err := json.Marshal(res)
        if err != nil {
            log.Printf("failed to marshal created user; err: %s", err)
            w.WriteHeader(http.StatusInternalServerError)
            return
        }

        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusCreated)
        w.Write(jsonData)
    })

    mux.Handle("/app/", cfg.middlewareMetricsInt(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))

    mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
        w.Header().Add("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusOK)
        fmt.Fprintf(w, "OK")
    })

    mux.HandleFunc("POST /api/chirps", func(w http.ResponseWriter, req *http.Request) {
        type errResponse struct {
            Error string `json:"error"`
        }

        type newChirpRequest struct {
            Body string `json:"body"`
            UserId uuid.UUID `json:"user_id"`
        }
        
        type newChirpResponse struct {
            Body string `json:"body"`
            Id uuid.UUID `json:"id"`
            CreatedAt time.Time `json:"created_at"`
            UpdatedAt time.Time `json:"updated_at"`
            UserId uuid.UUID `json:"user_id"`
        }

        decoder := json.NewDecoder(req.Body)
        var params newChirpRequest
        if err := decoder.Decode(&params); err != nil {
            log.Printf("error decoding prameters: %s", err)
            w.WriteHeader(http.StatusInternalServerError)
            resp := errResponse{Error: "Something went wrong"}
            jsonData, err := json.Marshal(resp)
            if err != nil {
                log.Fatal("could not marshal response")
            }
            w.Write(jsonData)
            return
        }

        if len(params.Body) > 140 {
            resp := errResponse{Error: "Chirp is too long"}
            w.WriteHeader(http.StatusBadRequest)
            jsonData, err := json.Marshal(resp)
            if err != nil {
                log.Fatal("could not marshal response")
            }
            w.Write(jsonData)
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
            w.WriteHeader(http.StatusInternalServerError)
            resp := errResponse{Error: "Something went wrong"}
            jsonData, err := json.Marshal(resp)
            if err != nil {
                log.Fatal("could not marshal response")
            }
            w.Write(jsonData)
            return

        }

        resp := newChirpResponse{
            Id: chirp.ID,
            UserId: chirp.UserID,
            CreatedAt: chirp.CreatedAt,
            UpdatedAt: chirp.UpdatedAt,
            Body: chirp.Body,
        }
        jsonData, err := json.Marshal(resp)
        if err != nil {
            log.Fatal("could not marshal response")
        }
        w.WriteHeader(http.StatusCreated)
        w.Write(jsonData)
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

    mux.HandleFunc("POST /admin/reset", func(w http.ResponseWriter, req *http.Request) {
        if cfg.platform != "dev" {
            w.WriteHeader(http.StatusForbidden)
            return
        }

        if err := dbQueries.DeleteUsers(context.Background()); err != nil {
            log.Printf("error deleting users; err: %s", err)
            w.WriteHeader(http.StatusInternalServerError)
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
