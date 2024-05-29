package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

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
}

func NewAPIServer(lp string, templatePath string, logsPath string, r *ContainerRunner) *APIServer {
	return &APIServer{
		ServerConfig: ServerConfig{
			ListenPort:   lp,
			TemplatePath: templatePath,
			LogsPath:     logsPath,
		},
		Runner: r,
	}
}

func (s *APIServer) Run() {
	r := mux.NewRouter()
	r.HandleFunc("/stop", s.Stop).Methods("POST")
	r.HandleFunc("/start", s.Start).Methods("POST")
	r.HandleFunc("/", s.Home).Methods("GET")
	r.HandleFunc("/logs", s.Logs)

	fmt.Printf("Server listening on port %v\n", s.ListenPort)
	if err := http.ListenAndServe(s.ListenPort, r); err != nil {
		panic(err)
	}
}

func (s *APIServer) Logs(w http.ResponseWriter, r *http.Request) {
	path := s.TemplatePath
	logsPath := s.LogsPath

	logs, err := GetMcServerLogs(logsPath)
	if err != nil {
		fmt.Println(err)
	}

	t, err := template.ParseFiles(path + "logs.html")
	if err != nil {

		fmt.Println("error in logs template")
		panic(err)
	}

	fmt.Println(t.Execute(w, logs))

}
func (s *APIServer) Home(w http.ResponseWriter, r *http.Request) {
	templatePath := s.TemplatePath

	// fmt.Println(logsPath)
	fmt.Println(templatePath)

	t, err := template.ParseFiles(templatePath + "home.html")
	if err != nil {
		fmt.Println("error in home template")
		panic(err)
	}

	t.Execute(w, "logs some day")
	fmt.Println("Home page accessed")
}
func (s *APIServer) Stop(w http.ResponseWriter, r *http.Request) {
	s.Runner.StopContainer()
	WriteJSON(w, http.StatusOK, "Stop container accessed")
}

func (s *APIServer) Start(w http.ResponseWriter, r *http.Request) {
	s.Runner.Containerize()
	WriteJSON(w, http.StatusOK, "Start container accessed")
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	w.Header().Add("Content-Type", "application/json")
	fmt.Printf("%v Request Status: %d \n", time.Now().UTC(), status)
	w.Write([]byte(fmt.Sprintf("Status: %v", status)))
}
