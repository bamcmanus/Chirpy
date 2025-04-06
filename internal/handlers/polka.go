package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/bamcmanus/Chirpy/internal/auth"
	"github.com/bamcmanus/Chirpy/internal/database"
	"github.com/google/uuid"
)

type PolkaHandler struct {
    dbQueries *database.Queries
    polkaKey string
}

func NewPolkaHandler(qs *database.Queries, pk string) PolkaHandler {
    return PolkaHandler{
        dbQueries: qs,
        polkaKey: pk,
    }
}

const USER_UPGRADED = "user.upgraded"

func (p PolkaHandler) UpgradeUser(w http.ResponseWriter, req *http.Request) {
    apiKey, err := auth.GetAPIKey(req.Header)
    if err != nil {
        log.Printf("missing authorization header")
        _ = respondWithError(w, http.StatusUnauthorized, "missing authorization header")
        return
    }

    if apiKey != p.polkaKey {
        log.Printf("invalid ")
        _ = respondWithError(w, http.StatusUnauthorized, "invalid API key")
        return
    }

    type upgradeRequest struct {
        Event string `json:"event"`
        Data struct {
            UserId string `json:"user_id"`
        } `json:"data"`
    }

    var ugRequest upgradeRequest
    decoder := json.NewDecoder(req.Body)
    if err := decoder.Decode(&ugRequest); err != nil {
        log.Printf("failed to decode request body; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "unknown request structure")
        return
    }

    if ugRequest.Event != USER_UPGRADED {
        w.WriteHeader(http.StatusNoContent)
        return
    }

    userId, err := uuid.Parse(ugRequest.Data.UserId)
    if err != nil {
        log.Printf("unable to parse user ID; error: %s", err)
        _ = respondWithError(w, http.StatusNotFound, "user ID not found")
    }

    _, err = p.dbQueries.UpgradeUser(req.Context(), userId)
    if err != nil {
        log.Printf("failed to upgrade user; error: %s", err)
        _ = respondWithError(w, http.StatusNotFound, "upgrade failed")
        return
    }

    w.WriteHeader(http.StatusNoContent)
}
