package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/bamcmanus/Chirpy/internal/auth"
	"github.com/bamcmanus/Chirpy/internal/database"
	"github.com/google/uuid"
)

type AuthHandler struct {
    dbQueries *database.Queries
    jwtSecret string
}

func NewAuthHandler(qs *database.Queries, secret string) AuthHandler {
    return AuthHandler{
        dbQueries: qs,
        jwtSecret: secret,
    }
}

func (a AuthHandler) Login(w http.ResponseWriter, req *http.Request) {
    type login struct {
        Password string `json:"password"`
        Email string `json:"email"`
    }

    var loginRequest login
    decoder := json.NewDecoder(req.Body)
    if err := decoder.Decode(&loginRequest); err != nil {
        log .Printf("failed to decode request; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "failed to decode request")
        return
    }

    user, err := a.dbQueries.GetUserByEmail(req.Context(), loginRequest.Email)
    if err != nil {
        log.Printf("failed to get user; error: %s", err)
        _ = respondWithError(w, http.StatusUnauthorized, "Incorrect email or Password")
        return
    }

    err = auth.CheckPasswordHash(user.HashedPassword, loginRequest.Password)
    if err != nil {
        log.Printf("passowrds do not match; error: %s", err)
        _ = respondWithError(w, http.StatusUnauthorized, "Incorrect email or password")
        return
    }

    token, err := auth.MakeJWT(user.ID, a.jwtSecret, time.Duration(3600) * time.Second)
    if err != nil {
        log.Printf("could not create JWT; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "failed to create JWT")
        return
    }

    refreshToken, err := auth.MakeRefreshToken()
    if err != nil {
        log.Printf("could not create refresh token; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "failed to create refresh token")
        return
    }

    params := database.CreateRefreshTokenParams{
        Token: refreshToken,
        UserID: user.ID,
    }
    _, err = a.dbQueries.CreateRefreshToken(req.Context(), params)
    if err != nil {
        log.Printf("could not perisste refresh token; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "refresh token creation failed")
        return
    }

    userReponse := struct{
        Id uuid.UUID `json:"id"`
        CreatedAt time.Time `json:"created_at"`
        UpdatedAt time.Time `json:"updated_at"`
        Email string `json:"email"`
        Token string `json:"token"`
        RefreshToken string `json:"refresh_token"`
    }{
        Id: user.ID,
        CreatedAt: user.CreatedAt,
        UpdatedAt: user.UpdatedAt,
        Email: user.Email,
        Token: token,
        RefreshToken: refreshToken,
    }
    if err := respondWithJSON(w, http.StatusOK, userReponse); err != nil {
        log.Printf("failed to respond; error: %s", err)
    }
}

func (a AuthHandler) Refresh(w http.ResponseWriter, req *http.Request) {
    refreshToken, err := auth.GetBearerToken(req.Header)
    if err != nil {
        log.Printf("failed to fetch Bearer token; error: %s", err)
        _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    token, err := a.dbQueries.GetRefreshToken(req.Context(), refreshToken)
    if err != nil {
        log.Printf("error fetching refresh token; error: %s", err)
        _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    if token.RevokedAt.Valid {
        log.Printf("token is expired; expiration time: %v", token.RevokedAt)
        _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    jwt, err := auth.MakeJWT(token.UserID, a.jwtSecret, time.Duration(3600) * time.Second)
    if err != nil {
        log.Printf("could not create JWT; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "failed to create JWT")
        return
    }

    res := struct{
        Token string `json:"token"`
    }{
        Token: jwt,
    }
    if err := respondWithJSON(w, http.StatusOK, res); err != nil {
        log.Printf("failed to respond; error: %s", err)
    }
}

func (a AuthHandler) Revoke(w http.ResponseWriter, req *http.Request) {
    refreshToken, err := auth.GetBearerToken(req.Header)
    if err != nil {
        log.Printf("failed to fetch Bearer token; error: %s", err)
        _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    _, err = a.dbQueries.RevokeRefreshToken(req.Context(), refreshToken)
    if err != nil {
        log.Printf("failed to revoke token; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "")
    }

    w.WriteHeader(http.StatusNoContent) 
}

