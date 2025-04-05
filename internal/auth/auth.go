package auth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

func HashPassword(password string) (string, error) {
    hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return "", err
    }
    return string(hashedPassword), nil
}

func CheckPasswordHash(hash, password string) error {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {
    claims := newRegisteredClaims(userID, expiresIn)
    jwt := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return jwt.SignedString([]byte(tokenSecret))
}

func newRegisteredClaims(userId uuid.UUID, expirationTime time.Duration) jwt.RegisteredClaims {
    return jwt.RegisteredClaims{
        ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(expirationTime)),
        IssuedAt: jwt.NewNumericDate(time.Now().UTC()),
        NotBefore: jwt.NewNumericDate(time.Now().UTC()),
        Issuer: "chirpy",
        Subject: userId.String(),
    }
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {
    var id uuid.UUID
    var claims jwt.RegisteredClaims
    _, err := jwt.ParseWithClaims(tokenString, &claims, func(token *jwt.Token) (any, error) {
        return []byte(tokenSecret), nil
    })
    if err != nil {
        return id, err
    }
    
    userID := claims.Subject
    if err := id.UnmarshalText([]byte(userID)); err != nil {
        return id, err
    }
    return id, nil
}

func GetBearerToken(headers http.Header) (string, error) {
    authHeader := headers.Get("Authorization")
    if authHeader == "" {
        return "", errors.New("missing Authorization header")
    }

    return strings.TrimPrefix(authHeader, "Bearer "), nil
}

func MakeRefreshToken() (string, error) {

    key := make([]byte, 32)
    _, err := rand.Read(key)
    if err != nil {
        return "", err
    }

    encodedString := hex.EncodeToString(key)

    return encodedString, nil
}

