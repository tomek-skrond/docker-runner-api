package main

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
)

type APIServer struct {
	ListenPort string
	Runner     *ContainerRunner
}

func NewAPIServer(lp string, r *ContainerRunner) *APIServer {
	return &APIServer{
		ListenPort: lp,
		Runner:     r,
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
	path := os.Getenv("TEMPLATE_PATH")
	logsPath := os.Getenv("LOGS_PATH")

	logs, error := GetMcServerLogs(logsPath)
	if error != nil {
		fmt.Println(error)
	}

	t, err := template.ParseFiles(path + "logs.html")
	if err != nil {
		fmt.Println("error in logs template")
		panic(err)
	}

	fmt.Println(t.Execute(w, logs))

}
func (s *APIServer) Home(w http.ResponseWriter, r *http.Request) {
	path := os.Getenv("TEMPLATE_PATH")
	t, err := template.ParseFiles(path + "home.html")
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
