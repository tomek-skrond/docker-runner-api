package main

import (
	"errors"
	"os"
	"time"

	"github.com/golang-jwt/jwt"
)

type LoginService struct {
	jwtSecret []byte
}

func NewLoginService(jwtSecret string) *LoginService {
	return &LoginService{
		jwtSecret: []byte(jwtSecret),
	}
}
func (ls *LoginService) Login(username, password string) (*string, *time.Time, error) {

	if username != os.Getenv("ADMIN_USER") || password != os.Getenv("ADMIN_PASSWORD") {
		return nil, nil, errors.New("invalid credentials")
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &jwt.StandardClaims{
		ExpiresAt: expirationTime.Unix(),
		Issuer:    username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(ls.jwtSecret)
	if err != nil {
		return nil, nil, err
	}

	return &tokenString, &expirationTime, nil
}
