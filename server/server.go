package server

import (
	"net/http"
	"github.com/gorilla/schema"
	"github.com/gorilla/mux"
	"log"
	"encoding/json"
)

var decoder = schema.NewDecoder()

type Serve struct {}

type Alert struct {
	AlertName string
	AlertIf   string
	AlertFrom string
}

type Response struct {
	Status      string
	Alert
}

var httpListenAndServe = http.ListenAndServe

var New = func() *Serve {
	return &Serve{}
}

func (s *Serve) Execute() error {
	// TODO: Request initial data from swarm-listener
	address := "0.0.0.0:8080"
	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/v1/docker-flow-monitor/alert", s.AlertHandler).Methods("GET")
//	// TODO: Add DELETE method
	if err := httpListenAndServe(address, r); err != nil {
		log.Println(err.Error())
		return err
	}
	return nil
}

func (s *Serve) AlertHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	alert := new(Alert)
//	// TODO: Update alerts config
//	// TODO: Reload prometheus
//	// TODO: Be omnipotent
	decoder.Decode(alert, req.Form)
	w.Header().Set("Content-Type", "application/json")
	response := Response{
		Status: "OK",
		Alert: *alert,
	}
	js, _ := json.Marshal(response)
	w.Write(js)
}
