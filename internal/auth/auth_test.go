package auth

import (
	"net/http"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestJWT(t *testing.T) {
    t.Run("Happy path", func(t *testing.T){
        id := uuid.New()
        tokenSecret := "my-secret-key"
        expiresIn := 1 * time.Minute

        jwt, err := MakeJWT(id, tokenSecret, expiresIn)

        assert.NotEmpty(t, jwt)
        assert.NoError(t, err)

        parsedId, err := ValidateJWT(jwt, tokenSecret)

        assert.NoError(t, err)
        assert.Equal(t, id, parsedId)
    })

    t.Run("Wrong secret fails", func(t *testing.T) {
        id := uuid.New()
        tokenSecret := "my-secret-key"
        expiresIn := 1 * time.Minute

        jwt, err := MakeJWT(id, tokenSecret, expiresIn)

        assert.NotEmpty(t, jwt)
        assert.NoError(t, err)

        _, err = ValidateJWT(jwt, "not-the-secret")

        assert.EqualError(t, err, "token signature is invalid: signature is invalid")
    })

    t.Run("expiration", func(t *testing.T) {
        id := uuid.New()
        tokenSecret := "my-secret-key"
        expiresIn := 1 * time.Millisecond

        jwt, err := MakeJWT(id, tokenSecret, expiresIn)

        assert.NotEmpty(t, jwt)
        assert.NoError(t, err)

        _, err = ValidateJWT(jwt, tokenSecret)

        assert.EqualError(t, err, "token has invalid claims: token is expired")
    })
}

func TestHeader(t *testing.T) {
    t.Run("no authorization header", func(t *testing.T) {
        var header http.Header

        jwt, err := GetBearerToken(header)

        assert.EqualError(t, err, "missing Authorization header")
        assert.Empty(t, jwt)
    })

    t.Run("happy path", func(t *testing.T) {
        header := make(http.Header)
        header.Set("Authorization", "Bearer token")

        jwt, err := GetBearerToken(header)

        assert.NoError(t, err)
        assert.Equal(t, "token", jwt)
    })
}
