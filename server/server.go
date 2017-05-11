package server

import (
	"net/http"
	"github.com/gorilla/schema"
	"github.com/gorilla/mux"
	"log"
	"encoding/json"
	"sync"
	"strings"
	"fmt"
	"os"
	"io/ioutil"
	"strconv"
	"../prometheus"
)

var decoder = schema.NewDecoder()
var mu = &sync.Mutex{}
var logPrintf = log.Printf

type Serve struct {
	Scrapes map[string]prometheus.Scrape
	Alerts map[string]prometheus.Alert
}

type Response struct {
	Status      int
	Message     string
	Alerts      []prometheus.Alert
	prometheus.Scrape
}

var httpListenAndServe = http.ListenAndServe

var New = func() *Serve {
	return &Serve{
		Alerts: make(map[string]prometheus.Alert),
		Scrapes: make(map[string]prometheus.Scrape),
	}
}

func (s *Serve) Execute() error {
	s.InitialConfig()
	prometheus.WriteConfig(s.Scrapes, s.Alerts)
	go prometheus.Run()
	address := "0.0.0.0:8080"
	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/v1/docker-flow-monitor/reconfigure", s.ReconfigureHandler)
	r.HandleFunc("/v1/docker-flow-monitor/remove", s.RemoveHandler)
	r.HandleFunc("/v1/docker-flow-monitor/", s.EmptyHandler)
	logPrintf("Starting Docker Flow Monitor")
	if err := httpListenAndServe(address, r); err != nil {
		logPrintf(err.Error())
		return err
	}
	return nil
}

func (s *Serve) EmptyHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (s *Serve) ReconfigureHandler(w http.ResponseWriter, req *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	logPrintf("Processing " + req.URL.Path)
	req.ParseForm()
	scrape := s.getScrape(req)
	s.deleteAlerts(scrape.ServiceName)
	alerts := s.getAlerts(req)
	prometheus.WriteConfig(s.Scrapes, s.Alerts)
	err := prometheus.Reload()
	statusCode := http.StatusOK
	resp := s.getResponse(&alerts, &scrape, err, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	js, _ := json.Marshal(resp)
	w.Write(js)
}

func (s *Serve) RemoveHandler(w http.ResponseWriter, req *http.Request) {
	logPrintf("Processing " + req.URL.Path)
	req.ParseForm()
	serviceName := req.URL.Query().Get("serviceName")
	scrape := s.Scrapes[serviceName]
	delete(s.Scrapes, serviceName)
	alerts := s.deleteAlerts(serviceName)
	prometheus.WriteConfig(s.Scrapes, s.Alerts)
	err := prometheus.Reload()
	statusCode := http.StatusOK
	resp := s.getResponse(&alerts, &scrape, err, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	js, _ := json.Marshal(resp)
	w.Write(js)
}

func (s *Serve) InitialConfig() error {
	if len(os.Getenv("LISTENER_ADDRESS")) > 0 {
		logPrintf("Requesting services from Docker Flow Swarm Listener")
		addr := os.Getenv("LISTENER_ADDRESS")
		if !strings.HasPrefix(addr, "http") {
			addr = fmt.Sprintf("http://%s:8080", addr)
		}
		addr = fmt.Sprintf("%s/v1/docker-flow-swarm-listener/get-services", addr)
		resp, err := http.Get(addr)
		if err != nil {
			return err
		}
		body, _ := ioutil.ReadAll(resp.Body)
		logPrintf("Processing: %s", string(body))
		data := []map[string]string{}
		json.Unmarshal(body, &data)
		for _, row := range data {
			if scrape, err := s.getScrapeFromMap(row); err == nil {
				s.Scrapes[scrape.ServiceName] = scrape
			}
			if alert, err := s.getAlertFromMap(row, ""); err == nil {
				s.Alerts[alert.AlertNameFormatted] = alert
			}
			for i:=1; i <= 10; i++ {
				suffix := fmt.Sprintf(".%d", i)
				if alert, err := s.getAlertFromMap(row, suffix); err == nil {
					s.Alerts[alert.AlertNameFormatted] = alert
				} else {
					break
				}
			}
		}
	}
	return nil
}

func (s *Serve) getScrapeFromMap(data map[string]string) (prometheus.Scrape, error) {
	scrape := prometheus.Scrape{}
	if port, err := strconv.Atoi(data["scrapePort"]); err == nil {
		scrape.ScrapePort = port
	}
	scrape.ServiceName = data["serviceName"]
	if s.isValidScrape(&scrape) {
		return scrape, nil
	}
	return prometheus.Scrape{}, fmt.Errorf("Not a valid scrape")
}

func (s *Serve) getAlertFromMap(data map[string]string, suffix string) (prometheus.Alert, error) {
	if _, ok := data["alertName" + suffix]; ok {
		alert := prometheus.Alert{}
		alert.AlertFor = data["alertFor" + suffix]
		alert.AlertIf = data["alertIf" + suffix]
		alert.AlertName = data["alertName" + suffix]
		alert.ServiceName = data["serviceName"]
		alert.AlertNameFormatted = s.getAlertNameFormatted(alert.ServiceName, alert.AlertName)
		if s.isValidAlert(&alert) {
			return alert, nil
		}
	}
	return prometheus.Alert{}, fmt.Errorf("Not a valid alert")
}

func (s *Serve) getAlerts(req *http.Request) []prometheus.Alert {
	alerts := []prometheus.Alert{}
	alertDecode := prometheus.Alert{}
	decoder.Decode(&alertDecode, req.Form)
	if s.isValidAlert(&alertDecode) {
		alertDecode.AlertNameFormatted = s.getAlertNameFormatted(alertDecode.ServiceName, alertDecode.AlertName)
		s.Alerts[alertDecode.AlertNameFormatted] = alertDecode
		alerts = append(alerts, alertDecode)
		logPrintf("Adding alert %s for the service %s", alertDecode.AlertName, alertDecode.ServiceName)
	}
	for i:=1; i <= 10; i++ {
		alertName := req.URL.Query().Get(fmt.Sprintf("alertName.%d", i))
		alert := prometheus.Alert{
			AlertNameFormatted: s.getAlertNameFormatted(alertDecode.ServiceName, alertName),
			ServiceName: alertDecode.ServiceName,
			AlertName: alertName,
			AlertIf: req.URL.Query().Get(fmt.Sprintf("alertIf.%d", i)),
			AlertFor: req.URL.Query().Get(fmt.Sprintf("alertFor.%d", i)),
		}
		if !s.isValidAlert(&alert) {
			break
		}
		s.Alerts[alert.AlertNameFormatted] = alert
		alerts = append(alerts, alert)
	}
	return alerts
}

func (s *Serve) isValidAlert(alert *prometheus.Alert) bool {
	return len(alert.AlertName) > 0 && len(alert.AlertIf) > 0
}

func (s *Serve) deleteAlerts(serviceName string) []prometheus.Alert {
	alerts := []prometheus.Alert{}
	serviceNameFormatted := s.getNameFormatted(serviceName)
	for k, v := range s.Alerts {
		if strings.HasPrefix(k, serviceNameFormatted) {
			alerts = append(alerts, v)
			delete(s.Alerts, k)
		}
	}
	return alerts
}

func (s *Serve) getAlertNameFormatted(serviceName, alertName string) string {
	return s.getNameFormatted(fmt.Sprintf("%s%s", serviceName, alertName))
}

func (s *Serve) getNameFormatted(name string) string {
	value := strings.Replace(name, "-", "", -1)
	return strings.Replace(value, "_", "", -1)
}

func (s *Serve) getScrape(req *http.Request) prometheus.Scrape {
	scrape := prometheus.Scrape{}
	decoder.Decode(&scrape, req.Form)
	if s.isValidScrape(&scrape) {
		s.Scrapes[scrape.ServiceName] = scrape
		logPrintf("Adding scrape " + scrape.ServiceName)
	}
	return scrape
}

func (s *Serve) isValidScrape(scrape *prometheus.Scrape) bool {
	return len(scrape.ServiceName) > 0 && scrape.ScrapePort > 0
}

func (s *Serve) getResponse(alerts *[]prometheus.Alert, scrape *prometheus.Scrape, err error, statusCode int) Response {
	resp := Response{
		Status: statusCode,
		Alerts: *alerts,
		Scrape: *scrape,
	}
	if err != nil {
		resp.Message = err.Error()
		resp.Status = http.StatusInternalServerError
	}
	return resp
}
