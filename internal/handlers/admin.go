package handlers

import (
	"fmt"
	"log"
	"net/http"
	"sync/atomic"

	"github.com/bamcmanus/Chirpy/internal/database"
)

type AdminHandler struct {
    dbQueries *database.Queries
    platform string
    fileserverHits *atomic.Int32
}

func NewAdminHandler(qs *database.Queries, platform string, fsh *atomic.Int32) AdminHandler {
    return AdminHandler {
        dbQueries: qs,
        platform: platform,
        fileserverHits: fsh,
    }
}

func Health(w http.ResponseWriter, req *http.Request) {
    w.Header().Add("Content-Type", "text/plain; charset=utf-8")
    w.WriteHeader(http.StatusOK)
    fmt.Fprintf(w, "OK")
}

func (a AdminHandler) GetMetrics(w http.ResponseWriter, req *http.Request) {
    w.Header().Add("Content-Type", "text/html")
    template := `<html>
    <body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
    </body>
    </html>`
    htmlContent := fmt.Sprintf(template, a.fileserverHits.Load())
    fmt.Fprint(w, htmlContent)
}

func (a AdminHandler) Reset(w http.ResponseWriter, req *http.Request) {
    if a.platform != "dev" {
        w.WriteHeader(http.StatusForbidden)
        return
    }

    if err := a.dbQueries.DeleteUsers(req.Context()); err != nil {
        log.Printf("error deleting users; err: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "error deleting users")
        return
    }

    a.fileserverHits.Store(0)
    w.Header().Add("Content-Type", "text/plain; charset=utf-8")
    w.WriteHeader(http.StatusOK)
}

