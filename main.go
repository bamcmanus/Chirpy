package main

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
    fileserverHits atomic.Int32
}

func (c *apiConfig) middlewareMetricsInt(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
        c.fileserverHits.Add(1)
        next.ServeHTTP(w, req)
    })
}

func main() {
    var cfg apiConfig
    mux := http.NewServeMux()

    mux.Handle("/app/", cfg.middlewareMetricsInt(http.FileServer(http.Dir("."))))

    mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, req *http.Request) {
        w.Header().Add("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusOK)
        fmt.Fprintf(w, "OK")
    })

    mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, req *http.Request) {
        w.Header().Add("Content-Type", "text/plain; charset=utf-8")
        fmt.Fprintf(w, "Hits: %d", cfg.fileserverHits.Load())
    })

    mux.HandleFunc("POST /reset", func(w http.ResponseWriter, req *http.Request) {
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
        log.Fatal("failed to start server")
    }
}
