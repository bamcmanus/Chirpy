package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/bamcmanus/Chirpy/internal/auth"
	"github.com/bamcmanus/Chirpy/internal/database"
	"github.com/google/uuid"
)

type ChirpsHandler struct {
    dbQueries *database.Queries
    jwtSecret string
}

type newChirpResponse struct {
    Body string `json:"body"`
    Id uuid.UUID `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    UserId uuid.UUID `json:"user_id"`
}


func NewChirpsHandler(qs *database.Queries, jwtSecret string) ChirpsHandler {
    return ChirpsHandler{
        dbQueries: qs,
        jwtSecret: jwtSecret,
    }
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

func (c ChirpsHandler) PostChirp(w http.ResponseWriter, req *http.Request) {
    type newChirpRequest struct {
        Body string `json:"body"`
    }

    token, err := auth.GetBearerToken(req.Header)
    if err != nil {
        log.Printf("failed to fetch Bearer token; error: %s", err)
        _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    userId, err := auth.ValidateJWT(token, c.jwtSecret)
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

    chirp, err := c.dbQueries.CreateChirp(req.Context(), cParams)
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
}

func (c ChirpsHandler) GetChirps(w http.ResponseWriter, req *http.Request) {
    chirps, err := c.dbQueries.ListChirps(req.Context())        
    if err != nil {
        log.Printf("error fetching chirps: %s", err)
        _ = respondWithError(w, http.StatusNotFound, "failed to fetch chirps")
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
}

func (c ChirpsHandler) GetChirp(w http.ResponseWriter, req *http.Request) {
    chirpId := req.PathValue("chirpID")
    log.Printf("received chirp ID: %s", chirpId)
    id := uuid.MustParse(chirpId)
    chirp, err := c.dbQueries.GetChirp(req.Context(), id)
    if err != nil {
        log.Printf("error fetching chirp; err: %s", err)
        _ = respondWithError(w, http.StatusNotFound, "failed to get chirp")
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
}

func (c ChirpsHandler) DeleteChirp(w http.ResponseWriter, req *http.Request) {
    token, err := auth.GetBearerToken(req.Header)
    if err != nil {
        log.Printf("failed to fetch Bearer token; error: %s", err)
        _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    userId, err := auth.ValidateJWT(token, c.jwtSecret)
    if err != nil  {
        log.Printf("JWT validation failed; error: %s", err)
        _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    chirpId, err := uuid.Parse(req.PathValue("chirpID"))
    if err != nil {
        log.Printf("could not parse chirp ID; error: %s", err)
        _ = respondWithError(w, http.StatusBadRequest, "invalid chirp ID")
        return
    }

    chirp, err := c.dbQueries.GetChirp(req.Context(), chirpId)
    if err != nil {
        log.Printf("error fetching chirp; err: %s", err)
        _ = respondWithError(w, http.StatusNotFound, "failed to get chirps")
        return
    }

    if userId != chirp.UserID {
        log.Printf("cannot delete chirp as user is not owner; chirp owner: %s; requsting user: %s", chirp.UserID.String(), userId.String())
        _ = respondWithError(w, http.StatusForbidden, "forbidden")
        return
    }

    err = c.dbQueries.DeleteChirp(req.Context(), chirpId)
    if err != nil {
        log.Printf("failed to delete chirp; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "failed to delete chirp")
        return
    }

    w.WriteHeader(http.StatusNoContent)
}

