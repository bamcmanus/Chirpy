package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync/atomic"
)

type request struct {
    Body string `json:"body"`
}


type apiConfig struct {
    fileserverHits atomic.Int32
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
    var cfg apiConfig
    mux := http.NewServeMux()

    mux.Handle("/app/", cfg.middlewareMetricsInt(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))

    mux.HandleFunc("GET /api/healthz", func(w http.ResponseWriter, req *http.Request) {
        w.Header().Add("Content-Type", "text/plain; charset=utf-8")
        w.WriteHeader(http.StatusOK)
        fmt.Fprintf(w, "OK")
    })

    mux.HandleFunc("POST /api/validate_chirp", func(w http.ResponseWriter, req *http.Request) {
        type errResponse struct {
            Error string `json:"error"`
        }
        
        type cleanedResponse struct {
            Body string `json:"cleaned_body"`
        }

        decoder := json.NewDecoder(req.Body)
        var params request
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

        resp := cleanedResponse{Body: body}
        jsonData, err := json.Marshal(resp)
        if err != nil {
            log.Fatal("could not marshal response")
        }
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
