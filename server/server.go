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
	r.HandleFunc("/v1/docker-flow-monitor/ping", s.PingHandler)
	// TODO: Do we need catch all?
	r.HandleFunc("/v1/docker-flow-monitor/", s.EmptyHandler)
	logPrintf("Starting Docker Flow Monitor")
	if err := httpListenAndServe(address, r); err != nil {
		logPrintf(err.Error())
		return err
	}
	return nil
}

func (s *Serve) PingHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (s *Serve) EmptyHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (s *Serve) ReconfigureHandler(w http.ResponseWriter, req *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	logPrintf("Processing " + req.URL.String())
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
		alert.AlertAnnotations = s.getMapFromString(data["alertAnnotations" + suffix])
		alert.AlertFor = data["alertFor" + suffix]
		alert.AlertIf = data["alertIf" + suffix]
		alert.AlertLabels = s.getMapFromString(data["alertLabels" + suffix])
		alert.AlertName = data["alertName" + suffix]
		alert.ServiceName = data["serviceName"]
		s.formatAlert(&alert)
		if s.isValidAlert(&alert) {
			return alert, nil
		}
	}
	return prometheus.Alert{}, fmt.Errorf("Not a valid alert")
}

func (s *Serve) getMapFromString(value string) map[string]string {
	mappedValue := map[string]string{}
	if len(value) > 0 {
		for _, label := range strings.Split(value, ",") {
			values := strings.Split(label, "=")
			mappedValue[values[0]] = values[1]
		}
	}
	return mappedValue
}

func (s *Serve) getAlerts(req *http.Request) []prometheus.Alert {
	alerts := []prometheus.Alert{}
	alertDecode := prometheus.Alert{}
	decoder.Decode(&alertDecode, req.Form)
	if s.isValidAlert(&alertDecode) {
		s.formatAlert(&alertDecode)
		s.Alerts[alertDecode.AlertNameFormatted] = alertDecode
		alerts = append(alerts, alertDecode)
		logPrintf("Adding alert %s for the service %s\n", alertDecode.AlertName, alertDecode.ServiceName, alertDecode)
	}
	for i:=1; i <= 10; i++ {
		alertName := req.URL.Query().Get(fmt.Sprintf("alertName.%d", i))
		alert := prometheus.Alert{
			ServiceName: alertDecode.ServiceName,
			AlertName: alertName,
			AlertIf: req.URL.Query().Get(fmt.Sprintf("alertIf.%d", i)),
			AlertFor: req.URL.Query().Get(fmt.Sprintf("alertFor.%d", i)),
		}
		s.formatAlert(&alert)
		if !s.isValidAlert(&alert) {
			break
		}
		s.Alerts[alert.AlertNameFormatted] = alert
		logPrintf("Adding alert %s for the service %s\n", alert.AlertName, alert.ServiceName, alert)
		alerts = append(alerts, alert)
	}
	return alerts
}

type alertIfShortcut struct {
	expanded    string
	shortcut    string
	annotations map[string]string
	labels      map[string]string
}

var alertIfShortcutData = []alertIfShortcut{
	{
		`container_memory_usage_bytes{container_label_com_docker_swarm_service_name="[SERVICE_NAME]"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="[SERVICE_NAME]"} > [VALUE]`,
		`@service_mem_limit:`,
		map[string]string{"summary": "Memory of the service [SERVICE_NAME] is over [VALUE]"},
		map[string]string{"receiver": "system", "service": "[SERVICE_NAME]"},
	}, {
		`(sum by (instance) (node_memory_MemTotal) - sum by (instance) (node_memory_MemFree + node_memory_Buffers + node_memory_Cached)) / sum by (instance) (node_memory_MemTotal) > [VALUE]`,
		`@node_mem_limit:`,
		map[string]string{"summary": "Memory of a node is over [VALUE]"},
		map[string]string{"receiver": "system", "service": "[SERVICE_NAME]"},
	}, {
		`(node_filesystem_size{fstype="aufs"} - node_filesystem_free{fstype="aufs"}) / node_filesystem_size{fstype="aufs"} > [VALUE]`,
		`@node_fs_limit:`,
		map[string]string{"summary": "Disk usage of a node is over [VALUE]"},
		map[string]string{"receiver": "system", "service": "[SERVICE_NAME]"},
	},
}

func (s *Serve) formatAlert(alert *prometheus.Alert) {
	alert.AlertNameFormatted = s.getNameFormatted(fmt.Sprintf("%s%s", alert.ServiceName, alert.AlertName))
	if strings.HasPrefix(alert.AlertIf, "@") {
		for _, data := range alertIfShortcutData {
			if strings.HasPrefix(alert.AlertIf, data.shortcut) {
				value := strings.Split(alert.AlertIf, ":")[1]
				alert.AlertIf = s.replaceTags(data.expanded, alert.ServiceName, value)
				if alert.AlertAnnotations == nil {
					alert.AlertAnnotations = map[string]string{}
				}
				for k, v := range data.annotations {
					alert.AlertAnnotations[k] = s.replaceTags(v, alert.ServiceName, value)
				}
				if alert.AlertLabels == nil {
					alert.AlertLabels= map[string]string{}
				}
				for k, v := range data.labels {
					alert.AlertLabels[k] = s.replaceTags(v, alert.ServiceName, value)
				}
			}
		}
	}
}

// TODO: Change to template
func (s *Serve) replaceTags(tag, serviceName, value string) string {
	replaced := strings.Replace(tag, "[SERVICE_NAME]", serviceName, -1)
	println(replaced)
	replaced = strings.Replace(replaced, "[VALUE]", value, -1)
	println(replaced)
	return replaced
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

func (s *Serve) getNameFormatted(name string) string {
	value := strings.Replace(name, "-", "", -1)
	return strings.Replace(value, "_", "", -1)
}

func (s *Serve) getScrape(req *http.Request) prometheus.Scrape {
	scrape := prometheus.Scrape{}
	decoder.Decode(&scrape, req.Form)
	if s.isValidScrape(&scrape) {
		s.Scrapes[scrape.ServiceName] = scrape
		logPrintf("Adding scrape %s\n%v", scrape.ServiceName, scrape)
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
