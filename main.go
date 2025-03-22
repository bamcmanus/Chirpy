package main

import (
	"log"
	"net/http"
)

func main() {
    mux := http.NewServeMux()
    mux.Handle("/app/", http.StripPrefix("/app/", http.FileServer(http.Dir("."))))
    mux.HandleFunc("/healthz", func(w http.ResponseWriter, req *http.Request) {
        w.Header().Add("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(200)
        w.Write([]byte("OK"))
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
