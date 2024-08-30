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
	containerService *ContainerService
	backupService    *BackupService
	loginService     *LoginService
	InfoLogger       *log.Logger
	ErrorLogger      *log.Logger
	jwtSecret        []byte
}

func NewAPIServer(lp string, logsPath string, loginSvc *LoginService, r *ContainerService, b *BackupService, secret string) *APIServer {
	return &APIServer{
		ServerConfig: ServerConfig{
			ListenPort: lp,
			LogsPath:   logsPath,
		},
		loginService:     loginSvc,
		containerService: r,
		backupService:    b,
		jwtSecret:        []byte(secret),
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
	var returnData any
	doesExist, _ := exists("backups")
	if !doesExist {
		if err := os.Mkdir("backups", os.FileMode(0755)); err != nil {
			log.Println("cannot create directory", err)
			WriteJSON(w, messageToJSON(http.StatusInternalServerError, err.Error(), nil))
			return
		}
	}

	// Stop the server container
	if _, err := s.containerService.StopContainer(); err != nil {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, err.Error(), nil))
		return
	}

	fileFlag := r.URL.Query().Get("file")

	if fileFlag == "true" {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<30) // 1 GB limit

		// Parse multipart form data
		if err := r.ParseMultipartForm(1 << 30); err != nil { // 1 GB limit
			WriteJSON(w, messageToJSON(http.StatusBadRequest, "unable to parse multipart form", nil))
			return
		}

		// Get the file from the form
		file, header, err := r.FormFile("file")
		if err != nil {
			WriteJSON(w, messageToJSON(http.StatusBadRequest, "error retrieving the file", nil))
			return
		}
		defer file.Close()

		// Extract the real file name
		fileName := header.Filename

		// Create a progress logger to log the upload status
		progressReader := &ProgressReader{
			Reader:      file,
			TotalBytes:  header.Size,
			LoggedBytes: 0,
			Logger:      log.Default(),
		}

		data, err := s.backupService.UploadBackupMultipart(progressReader, fileName)
		if err != nil {
			WriteJSON(w, messageToJSON(http.StatusInternalServerError, err.Error(), nil))
			return
		}
		returnData = data

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

		data, err := s.backupService.LoadBackupFromDisk(backupFile)
		if err != nil {
			WriteJSON(w, messageToJSON(http.StatusInternalServerError, "loading data from disk failed", nil))
			return
		}

		if _, err := s.containerService.Containerize(); err != nil {
			WriteJSON(w, messageToJSON(http.StatusInternalServerError, "failed to start server", nil))
			return
		}
		returnData = data
	}

	WriteJSON(w, messageToJSON(http.StatusOK, "loading data successful", returnData))
}

func (s *APIServer) SyncHandler(w http.ResponseWriter, r *http.Request) {

	backupData, err := s.backupService.Sync()
	if err != nil {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "error syncing data", nil))
	}
	WriteJSON(w, messageToJSON(http.StatusOK, "synced successfully", backupData))

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
	WriteJSON(w, messageToJSON(http.StatusOK, "retrieved backups", response))
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

	backupData, err := s.backupService.Backup(backupName)
	if err != nil {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "backup failed", nil))
		return
	}
	WriteJSON(w, messageToJSON(http.StatusOK, "backup successful", backupData))
}

func (s *APIServer) DeleteBackupHandler(w http.ResponseWriter, r *http.Request) {
	backupToDelete := r.URL.Query().Get("delete")

	if fileExists, _ := exists(fmt.Sprintf("backups/%s", backupToDelete)); !fileExists {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "backup file does not exist", nil))
		return
	}

	fileSize, err := getFileSize(fmt.Sprintf("backups/%s", backupToDelete))
	if err != nil {
		log.Println(err)
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "failed getting file size", nil))
		return
	}

	start := time.Now()
	removePath := fmt.Sprintf("backups/%s", backupToDelete)
	if err := os.Remove(removePath); err != nil {
		log.Println(err)
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "failed removing backup file", nil))
		return
	}

	deleteTime := time.Since(start)
	uploadTime := time.Duration(0)
	fileSizeFloat := float64(fileSize)

	backupData := NewBackupData(&Bucket{}, backupToDelete, fileSizeFloat, &deleteTime, &uploadTime, &start)

	WriteJSON(w, messageToJSON(http.StatusOK, fmt.Sprintf("backup %s deleted", backupToDelete), backupData))
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

	data, err := s.containerService.StopContainer()
	if err != nil {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "internal server error", nil))
		return
	}

	WriteJSON(w, messageToJSON(http.StatusOK, "server stopped", data))
}

func (s *APIServer) StartHandler(w http.ResponseWriter, r *http.Request) {

	data, err := s.containerService.Containerize()
	if err != nil {
		WriteJSON(w, messageToJSON(http.StatusInternalServerError, "internal server error", data))
		return
	}

	WriteJSON(w, messageToJSON(http.StatusOK, "server started", data))

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
