package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/golang-jwt/jwt"
	"github.com/gorilla/mux"
)

type ServerConfig struct {
	ListenPort   string
	TemplatePath string
	LogsPath     string
}

type APIServer struct {
	ServerConfig
	Runner      *ContainerRunner
	bucket      *Bucket
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
	jwtSecret   []byte
}

func NewAPIServer(lp string, templatePath string, logsPath string, r *ContainerRunner, b *Bucket, secret string) *APIServer {
	return &APIServer{
		ServerConfig: ServerConfig{
			ListenPort:   lp,
			TemplatePath: templatePath,
			LogsPath:     logsPath,
		},
		Runner:    r,
		bucket:    b,
		jwtSecret: []byte(secret),
	}
}

func (s *APIServer) Run() {

	r := mux.NewRouter()

	r.HandleFunc("/", s.LoginPage).Methods("GET")
	r.HandleFunc("/login", s.Login).Methods("POST")

	// r.HandleFunc("/stop", s.Stop).Methods("POST")
	r.Handle("/stop", s.JwtAuth(http.HandlerFunc(s.Stop))).Methods("POST")
	r.Handle("/start", s.JwtAuth(http.HandlerFunc(s.Start))).Methods("POST")

	r.Handle("/home", s.JwtAuth(http.HandlerFunc(s.Home))).Methods("GET")
	r.Handle("/logs", s.JwtAuth(http.HandlerFunc(s.Logs))).Methods("GET")

	r.Handle("/backups", s.JwtAuth(http.HandlerFunc(s.BackupPage))).Methods("GET")
	r.Handle("/backup", s.JwtAuth(http.HandlerFunc(s.Backup))).Methods("POST")
	r.Handle("/load-backup", s.JwtAuth(http.HandlerFunc(s.LoadBackup))).Methods("POST")

	r.Handle("/sync", s.JwtAuth(http.HandlerFunc(s.Sync))).Methods("POST")

	fmt.Printf("Server listening on port %v\n", s.ListenPort)
	if err := http.ListenAndServe(s.ListenPort, r); err != nil {
		panic(err)
	}
}

func (s *APIServer) LoadBackup(w http.ResponseWriter, r *http.Request) {
	// shutdown server
	s.Runner.StopContainer()
	backupFile := r.FormValue("backup")
	fmt.Println(backupFile)

	log.Println("loading new backup initiated")
	currentTime := time.Now()
	// Format the time to match the desired format
	formattedTime := currentTime.Format("20060102_150405")

	fileName := fmt.Sprintf("%s_%s.zip", "mcdata", formattedTime)

	//backup current state
	if err := zipit("mcdata", "backups/"+fileName, false); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatalln(err)
	}
	//remove current server files
	if err := removeAllFilesInDir("mcdata"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatalln(err)
	}
	//unzip backup to mcdata/
	if err := unzip(fmt.Sprintf("backups/%s", backupFile), "mcdata"); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		log.Fatalln(err)
	}
	//start the server
	s.Runner.Containerize()

	http.Redirect(w, r, "/backups", http.StatusSeeOther)
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

	if err := s.DownloadDataFromCloud(backupsInCloudStringArr); err != nil {
		log.Fatalln(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
	}
	// upload all files to disk

	http.Redirect(w, r, "/backups", http.StatusSeeOther)

}

func (s *APIServer) LoginPage(w http.ResponseWriter, r *http.Request) {
	// Check if the user already has a valid JWT token
	cookie, err := r.Cookie("token")
	if err == nil {
		tokenString := cookie.Value
		claims := &jwt.StandardClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return s.jwtSecret, nil
		})

		if err == nil && token.Valid {
			// If the token is valid, redirect to the home page
			http.Redirect(w, r, "/home", http.StatusSeeOther)
			return
		}
	}

	// If there's no valid token, show the login page
	path := s.TemplatePath
	t, err := template.ParseFiles(path + "login.html")
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	t.Execute(w, nil)
}

func (s *APIServer) Login(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		s.LoginPage(w, r)
		return
	}

	username, password := r.FormValue("username"), r.FormValue("password")

	if username != os.Getenv("ADMIN_USER") || password != os.Getenv("ADMIN_PASSWORD") {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &jwt.StandardClaims{
		ExpiresAt: expirationTime.Unix(),
		Issuer:    username,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString(s.jwtSecret)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "token",
		Value:    tokenString,
		HttpOnly: true,
		Expires:  expirationTime,
	})

	http.Redirect(w, r, "/home", http.StatusSeeOther)
}

func (s *APIServer) JwtAuth(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("token")
		if err != nil {
			if err == http.ErrNoCookie {
				http.Error(w, "Forbidden", http.StatusForbidden)
				return
			}
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}

		tokenString := cookie.Value
		claims := &jwt.StandardClaims{}
		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return s.jwtSecret, nil
		})

		if err != nil || !token.Valid {
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *APIServer) Backup(w http.ResponseWriter, r *http.Request) {
	backupName := r.FormValue("name")
	if backupName == "" {
		backupName = "server"
	}

	log.Println("backup initiated")
	currentTime := time.Now()
	// Format the time to match the desired format
	formattedTime := currentTime.Format("20060102_150405")

	fileName := fmt.Sprintf("%s_%s.zip", backupName, formattedTime)

	if err := zipit("mcdata", "backups/"+fileName, false); err != nil {
		log.Fatalln(err)
	}

	http.Redirect(w, r, "/backups", http.StatusSeeOther)
}

func (s *APIServer) BackupPage(w http.ResponseWriter, r *http.Request) {
	path := s.TemplatePath

	t, err := template.ParseFiles(path + "backups.html")
	if err != nil {
		log.Println(err)
	}

	backupsStringArr, err := GetAvailableBackups("backups/")
	if err != nil {
		log.Fatalln(err)
	}

	cloudBackupsArr, err := s.bucket.RetrieveObjectsInBucket(context.Background())
	if err != nil {
		log.Fatalln("unable to download object data from cloud", err)
	}

	backups := BackupTemplateData{
		Backups:      backupsStringArr,
		CloudBackups: cloudBackupsArr,
	}

	t.Execute(w, backups)
}

func (s *APIServer) Logs(w http.ResponseWriter, r *http.Request) {
	logsPath := s.LogsPath
	logs, err := GetMcServerLogs(logsPath)
	if err != nil {
		log.Println(err)
	}

	s.WriteTemplate(w, logs, "logs.html")
	log.Println("logs accessed")

}

func (s *APIServer) Home(w http.ResponseWriter, r *http.Request) {
	data := map[string]string{
		"Title": "Minecraft Server Management",
	}

	if err := s.WriteTemplate(w, data, "home.html"); err != nil {
		log.Printf("Error rendering home template: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	log.Println("Home page accessed")
}
func (s *APIServer) Stop(w http.ResponseWriter, r *http.Request) {
	// s.WriteTemplate(w, "home.html", nil)
	http.Redirect(w, r, "/home", http.StatusSeeOther)
	s.Runner.StopContainer()
	// WriteJSON(w, http.StatusOK, "Stop container accessed")
}

func (s *APIServer) Start(w http.ResponseWriter, r *http.Request) {
	// s.WriteTemplate(w, "home.html", nil)
	http.Redirect(w, r, "/home", http.StatusSeeOther)
	s.Runner.Containerize()
	// WriteJSON(w, http.StatusOK, "Start container accessed")
	log.Println("container accessed")
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	w.Header().Add("Content-Type", "application/json")
	fmt.Printf("%v Request Status: %d \n", time.Now().UTC(), status)
	w.Write([]byte(fmt.Sprintf("Status: %v", status)))
}

func (s *APIServer) WriteTemplate2(w http.ResponseWriter, site string, v any) {

	templatePath := s.TemplatePath

	t, err := template.ParseFiles(templatePath + site)
	if err != nil {
		fmt.Println("error in home template")
		panic(err)
	}

	t.Execute(w, v)

}

func (s *APIServer) WriteTemplate(w http.ResponseWriter, v any, site ...string) error {
	var templates []string
	for _, t := range site {
		templates = append(templates, filepath.Join(s.TemplatePath, t))
	}

	// Print out the final template paths (for debugging)
	fmt.Println("Loading templates:", templates)

	t, err := template.ParseFiles(templates...)
	if err != nil {
		log.Printf("Template parsing error: %v", err)
		return err
	}

	if err := t.Execute(w, v); err != nil {
		log.Printf("Template execution error: %v", err)
		return err
	}

	return nil
}
