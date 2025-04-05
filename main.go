package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"sync/atomic"

	"github.com/bamcmanus/Chirpy/internal/database"
	"github.com/bamcmanus/Chirpy/internal/handlers"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

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

func main() {
    godotenv.Load()
    dbUrl := os.Getenv("DB_URL")
    db, err := sql.Open("postgres", dbUrl)
    if err != nil {
        log.Fatalf("failed to connect to database; err: %s", err)
    }
    
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

    dbQueries := database.New(db)

    authHandler := handlers.NewAuthHandler(dbQueries, cfg.jwtSecret)

    mux.HandleFunc("POST /api/login", authHandler.Login)

    mux.HandleFunc("POST /api/refresh", authHandler.Refresh)

    mux.HandleFunc("POST /api/revoke", authHandler.Revoke)

    userHandler := handlers.NewUserHandler(dbQueries)

    mux.HandleFunc("POST /api/users", userHandler.CreateUser)

    mux.Handle("/app/", cfg.middlewareMetricsInt(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))

    mux.HandleFunc("GET /api/healthz", handlers.Health)

    chirpsHandler := handlers.NewChirpsHandler(dbQueries, cfg.jwtSecret)

    mux.HandleFunc("POST /api/chirps", chirpsHandler.PostChirp)

    mux.HandleFunc("GET /api/chirps", chirpsHandler.GetChirps)

    mux.HandleFunc("GET /api/chirps/{chirpID}", chirpsHandler.GetChirp)

    adminHandler := handlers.NewAdminHandler(dbQueries, cfg.platform, &cfg.fileserverHits)

    mux.HandleFunc("GET /admin/metrics", adminHandler.GetMetrics)

    mux.HandleFunc("POST /admin/reset", adminHandler.Reset)

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
