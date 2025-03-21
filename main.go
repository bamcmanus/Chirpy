package main

import (
	"log"
	"net/http"
)

func main() {
    mux := http.NewServeMux()
    mux.Handle("/", http.FileServer(http.Dir(".")))
    server := http.Server{
        Addr: ":8080",
        Handler: mux,
    }
    defer server.Close()

    if err := server.ListenAndServe(); err != nil {
        log.Fatal("failed to start server")
    }
}
