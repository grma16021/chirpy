package auth

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/alexedwards/argon2id"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func HashPassword(password string) (string, error) {
	hash, err := argon2id.CreateHash(password, argon2id.DefaultParams)
	if err != nil {
		return "", fmt.Errorf("Error hashing password")
	}

	return hash, nil
}

func CheckPasswordHash(password, hash string) (bool, error) {
	match, err := argon2id.ComparePasswordAndHash(password, hash)
	if err != nil {
		return match, fmt.Errorf("error comparing password and hash: %s", err)
	}

	return match, nil

}

func MakeJWT(userID uuid.UUID, tokenSecret string, expiresIn time.Duration) (string, error) {

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    "chirpy-access",
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(expiresIn)),
		Subject:   userID.String(),
	})

	ss, err := token.SignedString([]byte(tokenSecret))
	if err != nil {
		return "", err
	}

	return ss, nil
}

func ValidateJWT(tokenString, tokenSecret string) (uuid.UUID, error) {

	type myClaims struct {
		jwt.RegisteredClaims
	}

	token, err := jwt.ParseWithClaims(tokenString, &myClaims{}, func(token *jwt.Token) (any, error) {
		return []byte(tokenSecret), nil
	})
	if err != nil {
		return uuid.Nil, err
	} else if claims, ok := token.Claims.(*myClaims); ok {
		//fmt.Println(claims.Subject)
		userID := claims.Subject
		return uuid.Parse(userID)
	} else {
		log.Fatal("unknown claims type cannot proceed")
	}

	return uuid.Nil, err
}

func GetBearerToken(headers http.Header) (string, error) {
	tokenHeader := headers.Get("Authorization")
	if tokenHeader == "" {
		return "", fmt.Errorf("error getting token from header")
	}
	splitHeader := strings.Split(tokenHeader, " ")
	token := splitHeader[1]
	return token, nil
}

func MakeRefreshToken() string {
	key := make([]byte, 32)
	rand.Read(key)
	keyStr := hex.EncodeToString(key)

	return keyStr
}
