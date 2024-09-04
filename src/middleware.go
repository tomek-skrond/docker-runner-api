package main

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
)

// jwt middleware
func (s *APIServer) JwtAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract the Authorization header
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Split the header to get the token part
		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		tokenString := parts[1]

		// Parse the JWT token
		claims := &jwt.StandardClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return s.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		// Call the next handler in the chain
		next.ServeHTTP(w, r)
	})
}

// logger middleware

func LoggerMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// log.Printf("request started %s %s %s", r.Method, r.RequestURI, r.RemoteAddr)

		wrappedWriter := &responseWriter{w, http.StatusOK}
		next.ServeHTTP(wrappedWriter, r)

		log.Printf("%s %s %s in %v ms with status %d",
			r.Method,
			r.RequestURI,
			r.RemoteAddr,
			float64(time.Since(start).Microseconds())/1000,
			wrappedWriter.statusCode,
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// CORS middleware function
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*") // Change "*" to specific origin if needed
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		// Handle preflight request
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
