package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
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
	InfoLogger  *log.Logger
	ErrorLogger *log.Logger
	jwtSecret   []byte
}

func NewAPIServer(lp string, templatePath string, logsPath string, r *ContainerRunner, secret string) *APIServer {
	return &APIServer{
		ServerConfig: ServerConfig{
			ListenPort:   lp,
			TemplatePath: templatePath,
			LogsPath:     logsPath,
		},
		Runner:    r,
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

	fmt.Printf("Server listening on port %v\n", s.ListenPort)
	if err := http.ListenAndServe(s.ListenPort, r); err != nil {
		panic(err)
	}
}

func (s *APIServer) LoginPage(w http.ResponseWriter, r *http.Request) {
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

	expirationTime := time.Now().Add(5 * time.Minute)
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
	log.Println("backup initiated")
}

func (s *APIServer) BackupPage(w http.ResponseWriter, r *http.Request) {
	path := s.TemplatePath

	t, err := template.ParseFiles(path + "backups.html")
	if err != nil {
		log.Println(err)
	}

	t.Execute(w, nil)
}

func (s *APIServer) Logs(w http.ResponseWriter, r *http.Request) {
	logsPath := s.LogsPath
	logs, err := GetMcServerLogs(logsPath)
	if err != nil {
		log.Println(err)
	}

	s.WriteTemplate(w, "logs.html", logs)
	log.Println("logs accessed")

}
func (s *APIServer) Home(w http.ResponseWriter, r *http.Request) {
	s.WriteTemplate(w, "home.html", nil)
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

func (s *APIServer) WriteTemplate(w http.ResponseWriter, site string, v any) {

	templatePath := s.TemplatePath

	// fmt.Println(logsPath)
	// fmt.Println(templatePath)

	t, err := template.ParseFiles(templatePath + site)
	if err != nil {
		fmt.Println("error in home template")
		panic(err)
	}

	t.Execute(w, v)

}
