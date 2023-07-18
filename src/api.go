package main

import (
	"fmt"
	"net/http"
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

	fmt.Printf("Server listening on port %v\n", s.ListenPort)
	if err := http.ListenAndServe(s.ListenPort, r); err != nil {
		panic(err)
	}
}

func (s *APIServer) Stop(w http.ResponseWriter, r *http.Request) {
	s.Runner.StopContainer()
	WriteJSON(w, http.StatusOK, nil)
}

func (s *APIServer) Start(w http.ResponseWriter, r *http.Request) {
	s.Runner.Containerize()
	WriteJSON(w, http.StatusOK, nil)
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.WriteHeader(status)
	w.Header().Add("Content-Type", "application/json")
	fmt.Printf("%v Request Status: %d \n", time.Now().UTC(), status)
	w.Write([]byte(fmt.Sprintf("Status: %v", status)))
}
