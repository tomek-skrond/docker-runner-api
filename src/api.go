package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
)

type ServerConfig struct {
	ListenPort string
	LogsPath   string
}

type APIServer struct {
	ServerConfig
	runner       *ContainerRunner
	bucket       *Bucket
	loginService *LoginService
	InfoLogger   *log.Logger
	ErrorLogger  *log.Logger
	jwtSecret    []byte
}

func NewAPIServer(lp string, logsPath string, loginSvc *LoginService, r *ContainerRunner, b *Bucket, secret string) *APIServer {
	return &APIServer{
		ServerConfig: ServerConfig{
			ListenPort: lp,
			LogsPath:   logsPath,
		},
		loginService: loginSvc,
		runner:       r,
		bucket:       b,
		jwtSecret:    []byte(secret),
	}
}

func (s *APIServer) Run() {

	r := mux.NewRouter()

	r.HandleFunc("/login", s.LoginHandler).Methods("POST")

	r.Handle("/stop", s.JwtAuth(http.HandlerFunc(s.StopHandler))).Methods("POST")
	r.Handle("/start", s.JwtAuth(http.HandlerFunc(s.StartHandler))).Methods("POST")

	r.Handle("/logs", s.JwtAuth(http.HandlerFunc(s.LogsHandler))).Methods("GET")

	r.Handle("/backup", s.JwtAuth(http.HandlerFunc(s.Backup))).Methods("POST")

	r.Handle(
		"/backup",
		s.JwtAuth(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			WriteJSON(w, http.StatusNotImplemented, nil)
		})),
	).Methods("GET")

	r.Handle("/backup/delete", s.JwtAuth(http.HandlerFunc(s.DeleteBackupHandler))).Methods("DELETE")
	r.Handle("/backup/load", s.JwtAuth(http.HandlerFunc(s.LoadBackupHandler))).Methods("POST")

	r.Handle("/sync", s.JwtAuth(http.HandlerFunc(s.Sync))).Methods("POST")

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
			panic(err)
		}
	}

	// Stop the server container
	if err := s.runner.StopContainer(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, messageToJSON(err.Error()))
		return
	}

	fileFlag := r.URL.Query().Get("file")

	if fileFlag == "true" {
		r.Header.Set("Content-Type", "multipart/form-data")
		r.Body = http.MaxBytesReader(w, r.Body, 1<<30) // 1 GB limit

		// Create a temporary file to store the uploaded data
		tempFile, err := os.CreateTemp("", "backup-*")
		if err != nil {
			WriteJSON(w, http.StatusInternalServerError, messageToJSON("failed to create temporary file"))
			return
		}
		defer tempFile.Close()

		// Create a progress logger to log the upload status
		totalBytes := r.ContentLength
		progressReader := &ProgressReader{
			Reader:      r.Body,
			TotalBytes:  totalBytes,
			LoggedBytes: 0,
			Logger:      log.Default(),
		}

		// Write the uploaded file data to the temp file, logging progress
		if _, err := io.Copy(tempFile, progressReader); err != nil {
			WriteJSON(w, http.StatusInternalServerError, messageToJSON("failed to write file data"))
			return
		}

		// Call your method to handle the file
		if err := s.LoadBackupChooseFile(tempFile, tempFile.Name()); err != nil {
			log.Fatalln(err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	} else {
		var backupFileName struct {
			Backup string `json:"backup"`
		}

		// Decode JSON body to get the backup file name
		if err := json.NewDecoder(r.Body).Decode(&backupFileName); err != nil {
			WriteJSON(w, http.StatusInternalServerError, messageToJSON("error decoding body"))
			return
		}
		backupFile := backupFileName.Backup

		if err := s.LoadBackupFromDisk(backupFile); err != nil {
			WriteJSON(w, http.StatusInternalServerError, messageToJSON("loading data from disk failed"))
			return
		}
	}

	WriteJSON(w, http.StatusOK, messageToJSON("loading data successful"))
}

// ProgressReader wraps an io.Reader to log the progress of reading data
type ProgressReader struct {
	Reader       io.Reader
	TotalBytes   int64
	LoggedBytes  int64
	Logger       *log.Logger
	NextLogPoint int64
}

// Read overrides the Read method to add progress logging
func (p *ProgressReader) Read(b []byte) (int, error) {
	n, err := p.Reader.Read(b)
	if n > 0 {
		p.LoggedBytes += int64(n)
		percentage := float64(p.LoggedBytes) / float64(p.TotalBytes) * 100

		if p.NextLogPoint == 0 {
			p.NextLogPoint = 5
		}

		if percentage >= float64(p.NextLogPoint) {
			p.Logger.Printf("Uploaded %.0f%%", percentage)
			p.NextLogPoint += 5
		}
	}

	return n, err
}

func (s *APIServer) LoadBackupFromDisk(backupFile string) error {

	log.Println("loading new backup initiated")
	currentTime := time.Now()

	formattedTime := currentTime.Format("20060102_150405")

	fileName := fmt.Sprintf("%s_%s.zip", "mcdata", formattedTime)

	if err := zipit("mcdata", "backups/"+fileName, false); err != nil {
		log.Fatalln(err)
		return err
	}

	if err := removeAllFilesInDir("mcdata"); err != nil {
		log.Fatalln(err)
		return err

	}

	if err := unzip(fmt.Sprintf("backups/%s", backupFile), "mcdata"); err != nil {
		log.Fatalln(err)
		return err

	}

	s.runner.Containerize()

	return nil
}

func (s *APIServer) LoadBackupChooseFile(file multipart.File, backupName string) error {

	// You could save the file or process it further here
	// For example, save the file to disk
	out, err := os.Create(fmt.Sprintf("%s", backupName))
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	return err

	// log.Printf("File uploaded successfully")
	// return nil
}

func (s *APIServer) UploadDataToCloud(backupsStrArr []string) error {
	for _, backup := range backupsStrArr {
		objectPath := fmt.Sprintf("backups/%s", backup)

		// Check if the object already exists in GCS
		log.Println("check if object exists", backup)
		exists, err := s.bucket.ObjectExists(backup)
		if err != nil {
			log.Printf("Error checking if object exists in GCS: %v", err)
			return err
		}
		if exists {
			log.Printf("Object %s already exists in GCS. Skipping upload.", objectPath)
			continue
		}
		log.Printf("uploading file %s to GCS\n", backup)
		if err := s.bucket.UploadFileToGCS(objectPath); err != nil {
			log.Fatalln(err)
			return err
		}
	}
	fmt.Println("uploading data to cloud successful")

	return nil
}

func (s *APIServer) DownloadDataFromCloud(backupsInCloud []string) error {
	log.Println("getting available backups from disk")
	backupsOnDisk, err := GetAvailableBackups("backups/")
	if err != nil {
		log.Fatalln(err)
		return err
	}
	for _, backup := range backupsInCloud {
		if !contains(backupsOnDisk, backup) {
			log.Printf("downloading backup %s from cloud", backup)
			if err := s.bucket.DownloadDataFromBucket(context.Background(), backup); err != nil {
				log.Fatalln(err)
				return err
			}
		}
	}
	fmt.Println("downloading data to disk successful")

	return nil
}

func (s *APIServer) Sync(w http.ResponseWriter, r *http.Request) {

	http.Redirect(w, r, "/backups", http.StatusSeeOther)

	if s.bucket.projectID == "" || s.bucket.Name == "" {
		return
	}

	backupsStringArr, err := GetAvailableBackups("backups/")
	if err != nil {
		log.Fatalln(err)
	}

	if err := s.bucket.CreateGCSBucket(); err != nil {
		log.Println(err)
		http.Redirect(w, r, "/backups", http.StatusInternalServerError)
	}

	// upload all files to cloud

	if err := s.UploadDataToCloud(backupsStringArr); err != nil {
		log.Fatalln(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}

	backupsInCloudStringArr, err := s.bucket.RetrieveObjectsInBucket(context.Background())
	if err != nil {
		log.Fatalln(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}

	// upload all files to disk
	if err := s.DownloadDataFromCloud(backupsInCloudStringArr); err != nil {
		log.Fatalln(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}

}

func (s *APIServer) LoginHandler(w http.ResponseWriter, r *http.Request) {
	var credentials struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		WriteJSON(w, http.StatusBadRequest, messageToJSON("invalid request payload"))
		return
	}

	// Extract the username and password
	username := credentials.Username
	password := credentials.Password

	if username == "" || password == "" {
		WriteJSON(w, http.StatusUnauthorized, messageToJSON("login information incomplete"))
		return
	}

	tokenString, expirationTime, err := s.loginService.Login(username, password)
	if err != nil {
		WriteJSON(w, http.StatusUnauthorized, messageToJSON("unauthorized"))
		return
	}

	// Set the Authorization header in the response
	w.Header().Set("Authorization", "Bearer "+*tokenString)
	w.Header().Set("Content-Type", "application/json")

	// Return the token and expiration time in the response body
	WriteJSON(w, http.StatusOK, map[string]interface{}{
		"token":          *tokenString,
		"expirationTime": expirationTime.Format(time.RFC3339),
	})
}

func (s *APIServer) Backup(w http.ResponseWriter, r *http.Request) {
	var backupFileName struct {
		Backup string `json:"backup"`
	}

	// Decode JSON body to get the backup file name
	if err := json.NewDecoder(r.Body).Decode(&backupFileName); err != nil {
		WriteJSON(w, http.StatusInternalServerError, messageToJSON("error decoding body"))
		return
	}
	backupName := backupFileName.Backup
	if backupName == "" {
		backupName = "server"
	}

	// Perform backup operation here
	currentTime := time.Now()
	formattedTime := currentTime.Format("20060102_150405")
	fileName := fmt.Sprintf("%s_%s.zip", backupName, formattedTime)

	if err := zipit("mcdata", "backups/"+fileName, false); err != nil {
		WriteJSON(w, http.StatusInternalServerError, messageToJSON("error creating backup"))
		return
	}
	WriteJSON(w, http.StatusOK, messageToJSON("backup successful"))
}

func (s *APIServer) DeleteBackupHandler(w http.ResponseWriter, r *http.Request) {
	backupToDelete := r.URL.Query().Get("delete")
	removePath := fmt.Sprintf("backups/%s", backupToDelete)
	if err := os.Remove(removePath); err != nil {
		log.Fatalln(err)
	}
	log.Printf("backup %s deleted\n", backupToDelete)
}

func (s *APIServer) LogsHandler(w http.ResponseWriter, r *http.Request) {
	logsPath := s.LogsPath
	logs, err := GetMcServerLogs(logsPath)
	if err != nil {
		WriteJSON(w, http.StatusInternalServerError, map[string]string{
			"logs": "error reading logs",
		})
		return
	}

	WriteJSON(w, http.StatusOK, map[string][]string{
		"logs": logs,
	})
}

func (s *APIServer) StopHandler(w http.ResponseWriter, r *http.Request) {

	if err := s.runner.StopContainer(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, messageToJSON("internal server error"))
		return
	}

	WriteJSON(w, http.StatusOK, messageToJSON("server stopped"))
}

func (s *APIServer) StartHandler(w http.ResponseWriter, r *http.Request) {

	if err := s.runner.Containerize(); err != nil {
		WriteJSON(w, http.StatusInternalServerError, messageToJSON("internal server error"))
		return
	}

	WriteJSON(w, http.StatusOK, messageToJSON("server started"))

}

func messageToJSON(msg string) map[string]string {
	return map[string]string{
		"message": msg,
	}
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	w.Header().Add("Content-Type", "application/json")
	log.Printf("request status: %d\n", status)
	json.NewEncoder(w).Encode(v)
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
