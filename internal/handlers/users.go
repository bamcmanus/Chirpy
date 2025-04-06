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

type UserHandler struct {
    dbQueries *database.Queries
    jwtSecret string
}

func NewUserHandler(qs *database.Queries, secret string) UserHandler {
    return UserHandler{
        dbQueries: qs,
        jwtSecret: secret,
    }
}

type userResponse struct {
    Id uuid.UUID `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Email string `json:"email"`
}

type userRequest struct {
    Email string `json:"email"`
    Password string `json:"password"`
}

func (u UserHandler) CreateUser(w http.ResponseWriter, req *http.Request) {

    var newUserReq userRequest
    decoder := json.NewDecoder(req.Body)
    if err := decoder.Decode(&newUserReq); err != nil {
        log.Printf("failed to decode request body; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "could not decode new user request")
        return
    }

    if newUserReq.Password == "" {
        _ = respondWithError(w, http.StatusBadRequest, "password required")
        return
    }

    hashedPassword, err := auth.HashPassword(newUserReq.Password)
    if err != nil {
        log.Printf("failed to hash password; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "failed hashing password")
        return
    }

    params := database.CreateUserParams{Email: newUserReq.Email, HashedPassword: hashedPassword}
    user, err := u.dbQueries.CreateUser(req.Context(), params)
    if err != nil {
        _ = respondWithError(w, http.StatusInternalServerError, "failed to create user")
        return
    }

    res := userResponse {
        Email: user.Email,
        CreatedAt: user.CreatedAt,
        UpdatedAt: user.UpdatedAt,
        Id: user.ID,
    }

    _ = respondWithJSON(w, http.StatusCreated, res)
}

func (u UserHandler) UpdateUser(w http.ResponseWriter, req *http.Request) {
    token, err := auth.GetBearerToken(req.Header)
    if err != nil {
        log.Printf("failed to fetch Bearer token; error: %s", err)
        _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    userId, err := auth.ValidateJWT(token, u.jwtSecret)
    if err != nil  {
        log.Printf("JWT validation failed; error: %s", err)
        _ = respondWithError(w, http.StatusUnauthorized, "unauthorized")
        return
    }

    var updateRequest userRequest
    decoder := json.NewDecoder(req.Body)
    if err := decoder.Decode(&updateRequest); err != nil {
        log.Printf("failed to decode request body; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "could not decode new user request")
        return
    }

    if updateRequest.Password == "" {
        _ = respondWithError(w, http.StatusBadRequest, "password required")
        return
    }

    hashedPassword, err := auth.HashPassword(updateRequest.Password)
    if err != nil {
        log.Printf("failed to hash password; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "failed hashing password")
        return
    }

    updateParams := database.UpdateUserParams {
        Email: updateRequest.Email,
        HashedPassword: hashedPassword,
        ID: userId,
    }
    user, err := u.dbQueries.UpdateUser(req.Context(), updateParams)
    if err != nil {
        log.Printf("failed to update user; error: %s", err)
        _ = respondWithError(w, http.StatusInternalServerError, "error updating user")
        return
    }

    res := userResponse{
        Id: user.ID,
        CreatedAt: user.CreatedAt,
        UpdatedAt: user.UpdatedAt,
        Email: user.Email,
    }

    _ = respondWithJSON(w, http.StatusOK, res)
}
