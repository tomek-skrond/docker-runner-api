package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
)

type JSONResponse struct {
	ResponseContent any    `json:"response"`
	HTTPStatus      int    `json:"http_status"`
	Message         string `json:"message"`
}

func NewJSONResponse(status int, msg string, content any) JSONResponse {
	return JSONResponse{
		ResponseContent: content,
		HTTPStatus:      status,
		Message:         msg,
	}
}

type ServerConfig struct {
	ListenPort string
	LogsPath   string
}

type APIServer struct {
	ServerConfig
	runner        *ContainerRunner
	backupService *BackupService
	loginService  *LoginService
	InfoLogger    *log.Logger
	ErrorLogger   *log.Logger
	jwtSecret     []byte
}

func NewAPIServer(lp string, logsPath string, loginSvc *LoginService, r *ContainerRunner, b *BackupService, secret string) *APIServer {
	return &APIServer{
		ServerConfig: ServerConfig{
			ListenPort: lp,
			LogsPath:   logsPath,
		},
		loginService:  loginSvc,
		runner:        r,
		backupService: b,
		jwtSecret:     []byte(secret),
	}
}

func (s *APIServer) Run() {

	r := mux.NewRouter()

	r.HandleFunc("/login", s.LoginHandler).Methods("POST")

	r.Handle("/stop", s.JwtAuth(http.HandlerFunc(s.StopHandler))).Methods("POST")
	r.Handle("/start", s.JwtAuth(http.HandlerFunc(s.StartHandler))).Methods("POST")

	r.Handle("/logs", s.JwtAuth(http.HandlerFunc(s.LogsHandler))).Methods("GET")

	r.Handle("/backup", s.JwtAuth(http.HandlerFunc(s.BackupHandler))).Methods("POST")
	r.Handle("/backup", s.JwtAuth(http.HandlerFunc(s.GetBackupHandler))).Methods("GET")

	r.Handle("/backup/delete", s.JwtAuth(http.HandlerFunc(s.DeleteBackupHandler))).Methods("DELETE")
	r.Handle("/backup/load", s.JwtAuth(http.HandlerFunc(s.LoadBackupHandler))).Methods("POST")

	r.Handle("/sync", s.JwtAuth(http.HandlerFunc(s.SyncHandler))).Methods("POST")

	fmt.Printf("Server listening on port %v\n", s.ListenPort)
	if err := http.ListenAndServe(s.ListenPort, r); err != nil {
		panic(err)
	}
}

// To load backup:
// shutdown server
// format the time to match the desired format
// backup current state
// remove current server files
// unzip backup to mcdata/
// start the server
func (s *APIServer) LoadBackupHandler(w http.ResponseWriter, r *http.Request) {
	doesExist, _ := exists("backups")
	if !doesExist {
		if err := os.Mkdir("backups", os.FileMode(0755)); err != nil {
			log.Fatalln("cannot create directory", err)
			return
		}
	}

	// Stop the server container
	if err := s.runner.StopContainer(); err != nil {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, err.Error(), nil))
		return
	}

	fileFlag := r.URL.Query().Get("file")

	if fileFlag == "true" {
		r.Header.Set("Content-Type", "multipart/form-data")
		r.Body = http.MaxBytesReader(w, r.Body, 1<<30) // 1 GB limit

		// Create a progress logger to log the upload status
		totalBytes := r.ContentLength
		progressReader := &ProgressReader{
			Reader:      r.Body,
			TotalBytes:  totalBytes,
			LoggedBytes: 0,
			Logger:      log.Default(),
		}

		if err := s.backupService.UploadBackupMultipart(progressReader); err != nil {
			WriteJSON(w, messageToJSON(http.StatusInternalServerError, "backup multipart failure", nil))
			return
		}

	} else {
		var backupFileName struct {
			Backup string `json:"backup"`
		}

		// Decode JSON body to get the backup file name
		if err := json.NewDecoder(r.Body).Decode(&backupFileName); err != nil {
			WriteJSON(w, messageToJSON(http.StatusInternalServerError, "error decoding body", nil))
			return
		}
		backupFile := backupFileName.Backup

		if err := s.backupService.LoadBackupFromDisk(backupFile); err != nil {
			WriteJSON(w, messageToJSON(http.StatusInternalServerError, "loading data from disk failed", nil))
			return
		}

		if err := s.runner.Containerize(); err != nil {
			WriteJSON(w, messageToJSON(http.StatusInternalServerError, "failed to start server", nil))
			return
		}
	}

	WriteJSON(w, messageToJSON(http.StatusOK, "loading data successful", "TODO"))
}

func (s *APIServer) SyncHandler(w http.ResponseWriter, r *http.Request) {

	if err := s.backupService.Sync(); err != nil {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "error syncing data", nil))
	}
	WriteJSON(w, messageToJSON(http.StatusOK, "synced successfully", nil))

}

func (s *APIServer) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		WriteJSON(w, messageToJSON(http.StatusBadRequest, "invalid request payload", nil))
		return
	}

	// Extract the username and password
	username := credentials.Username
	password := credentials.Password

	if username == "" || password == "" {
		WriteJSON(w, messageToJSON(http.StatusUnauthorized, "login information incomplete", nil))
		return
	}

	tokenString, expirationTime, err := s.loginService.Login(username, password)
	if err != nil {
		WriteJSON(w, messageToJSON(http.StatusUnauthorized, "unauthorized", nil))
		return
	}

	// Set the Authorization header in the response
	w.Header().Set("Authorization", "Bearer "+*tokenString)
	w.Header().Set("Content-Type", "application/json")

	response := map[string]interface{}{
		"token":          *tokenString,
		"expirationTime": expirationTime.Format(time.RFC3339),
	}
	// Return the token and expiration time in the response body
	WriteJSON(w, messageToJSON(http.StatusOK, "authorized", response))
}

func (s *APIServer) GetBackupHandler(w http.ResponseWriter, r *http.Request) {
	backups, err := s.backupService.GetBackups()
	if err != nil {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "getting backups failed", nil))
		return
	}
	response := map[string][]string{
		"backups": backups,
	}
	WriteJSON(w, messageToJSON(http.StatusOK, "backup successful", response))
}

func (s *APIServer) BackupHandler(w http.ResponseWriter, r *http.Request) {
	var backupFileName struct {
		Backup string `json:"backup"`
	}

	// Decode JSON body to get the backup file name
	if err := json.NewDecoder(r.Body).Decode(&backupFileName); err != nil {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "error decoding body", nil))
		return
	}
	backupName := backupFileName.Backup
	if backupName == "" {
		backupName = "server"
	}

	if err := s.backupService.Backup(backupName); err != nil {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "backup failed", nil))
		return
	}
	WriteJSON(w, messageToJSON(http.StatusOK, "backup successful", "todo"))
}

func (s *APIServer) DeleteBackupHandler(w http.ResponseWriter, r *http.Request) {
	backupToDelete := r.URL.Query().Get("delete")
	removePath := fmt.Sprintf("backups/%s", backupToDelete)
	if err := os.Remove(removePath); err != nil {
		log.Fatalln(err)
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "failed removing backup file", nil))
	}
	WriteJSON(w, messageToJSON(http.StatusOK, fmt.Sprintf("backup %s deleted\n", backupToDelete), "todo"))
}

func (s *APIServer) LogsHandler(w http.ResponseWriter, r *http.Request) {
	logsPath := s.LogsPath
	logs, err := GetMcServerLogs(logsPath)
	if err != nil {
		resp := map[string]string{
			"logs": "error reading logs",
		}
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "error reading logs", resp))
		return
	}

	resp := map[string][]string{
		"logs": logs,
	}

	WriteJSON(w, messageToJSON(http.StatusOK, "logs retrieved", resp))

}

func (s *APIServer) StopHandler(w http.ResponseWriter, r *http.Request) {

	if err := s.runner.StopContainer(); err != nil {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "internal server error", nil))
		return
	}

	WriteJSON(w, messageToJSON(http.StatusOK, "server stopped", "todo"))
}

func (s *APIServer) StartHandler(w http.ResponseWriter, r *http.Request) {

	if err := s.runner.Containerize(); err != nil {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "internal server error", nil))
		return
	}

	WriteJSON(w, messageToJSON(http.StatusOK, "server started", nil))

}

// middleware
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
