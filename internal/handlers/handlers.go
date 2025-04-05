package handlers

import (
	"encoding/json"
	"net/http"
)

type errResponse struct {
    Error string `json:"error"`
}

func respondWithJSON(w http.ResponseWriter, code int, payload any) error {
    response, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    w.Write(response)
    return nil
}

func respondWithError(w http.ResponseWriter, code int, msg string) error {
    return respondWithJSON(w, code, errResponse{Error: msg})
}

