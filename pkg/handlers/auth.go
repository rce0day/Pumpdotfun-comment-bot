package handlers

import (
    "encoding/json"
    "net/http"
    "godb/pkg/database"
)

func RequireAuth(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        cookie, err := r.Cookie("Authorization")
        if err != nil {
            respondWithError(w, http.StatusUnauthorized, "Access Denied")
            return
        }
        valid, err := database.CheckAuthToken(cookie.Value)
        if err != nil {
            respondWithError(w, http.StatusInternalServerError, "Authentication Failed")
            return
        }
        if !valid {
            respondWithError(w, http.StatusUnauthorized, "Invalid Auth Token")
            return
        }
        next(w, r)
    }
}

func respondWithError(w http.ResponseWriter, code int, message string) {
    response := Response{Error: message}
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(code)
    json.NewEncoder(w).Encode(response)
}

type Response struct {
    Error string `json:"error,omitempty"`
}