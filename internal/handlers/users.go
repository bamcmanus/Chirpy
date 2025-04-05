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
}

func NewUserHandler(qs *database.Queries) UserHandler {
    return UserHandler{
        dbQueries: qs,
    }
}

type newUserResponse struct {
    Id uuid.UUID `json:"id"`
    CreatedAt time.Time `json:"created_at"`
    UpdatedAt time.Time `json:"updated_at"`
    Email string `json:"email"`
}

func (u UserHandler) CreateUser(w http.ResponseWriter, req *http.Request) {
    type newUserRequest struct {
        Email string `json:"email"`
        Password string `json:"password"`
    }

    var newUserReq newUserRequest
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

    res := newUserResponse {
        Email: user.Email,
        CreatedAt: user.CreatedAt,
        UpdatedAt: user.UpdatedAt,
        Id: user.ID,
    }

    _ = respondWithJSON(w, http.StatusCreated, res)
}
