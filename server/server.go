package server

import (
	"encoding/json"
	"github.com/gorilla/mux"
	"net/http"
	"github.com/gorilla/schema"
	"log"
)

var decoder = schema.NewDecoder()
var httpWriterSetContentType = func(w http.ResponseWriter, value string) {
	w.Header().Set("Content-Type", value)
}

type Serve struct {}

type Alert struct {
	AlertName string
	AlertIf   string
	AlertFrom string
}

type Response struct {
	Status      string
}

var httpListenAndServe = http.ListenAndServe

var New = func() (*Serve, error) {
	return &Serve{}, nil
}

func (s *Serve) Execute() error {
	// TODO: Request initial data from swarm-listener
	address := "0.0.0.0:8080"
	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/v1/docker-flow-monitor/alert", s.AlertHandler).Methods("GET")
	// TODO: Add DELETE method
	if err := httpListenAndServe(address, r); err != nil {
		log.Println(err.Error())
		return err
	}
	return nil
}

func (s *Serve) AlertHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	alert := new(Alert)
	// TODO: Update alerts config
	// TODO: Reload prometheus
	// TODO: Be omnipotent
	decoder.Decode(alert, req.Form)
	println("NAME", alert.AlertName)
	println("IF", alert.AlertIf)
	println("FROM", alert.AlertFrom)
	httpWriterSetContentType(w, "application/json")
	response := Response{
		Status: "OK",
	}
	js, _ := json.Marshal(response)
	w.Write(js)
}
