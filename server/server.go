package server

import (
	"../prometheus"
	"encoding/json"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

var decoder = schema.NewDecoder()
var mu = &sync.Mutex{}
var logPrintf = log.Printf
var listenerTimeout = 30 * time.Second

type serve struct {
	scrapes map[string]prometheus.Scrape
	alerts  map[string]prometheus.Alert
}

type response struct {
	Status  int
	Message string
	Alerts  []prometheus.Alert
	prometheus.Scrape
}

var httpListenAndServe = http.ListenAndServe

const scrapePort = "SCRAPE_PORT"
const serviceName = "SERVICE_NAME"

// New returns instance of the `serve` structure
var New = func() *serve {
	return &serve{
		alerts:  make(map[string]prometheus.Alert),
		scrapes: make(map[string]prometheus.Scrape),
	}
}

func (s *serve) Execute() error {
	s.InitialConfig()
	prometheus.WriteConfig(s.scrapes, s.alerts)
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

func (s *serve) PingHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (s *serve) EmptyHandler(w http.ResponseWriter, req *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
}

func (s *serve) ReconfigureHandler(w http.ResponseWriter, req *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	logPrintf("Processing " + req.URL.String())
	req.ParseForm()
	scrape := s.getScrape(req)
	s.deleteAlerts(scrape.ServiceName)
	alerts := s.getAlerts(req)
	prometheus.WriteConfig(s.scrapes, s.alerts)
	err := prometheus.Reload()
	statusCode := http.StatusOK
	resp := s.getResponse(&alerts, &scrape, err, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	js, _ := json.Marshal(resp)
	w.Write(js)
}

func (s *serve) RemoveHandler(w http.ResponseWriter, req *http.Request) {
	logPrintf("Processing " + req.URL.Path)
	req.ParseForm()
	serviceName := req.URL.Query().Get("serviceName")
	scrape := s.scrapes[serviceName]
	delete(s.scrapes, serviceName)
	alerts := s.deleteAlerts(serviceName)
	prometheus.WriteConfig(s.scrapes, s.alerts)
	err := prometheus.Reload()
	statusCode := http.StatusOK
	resp := s.getResponse(&alerts, &scrape, err, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	js, _ := json.Marshal(resp)
	w.Write(js)
}

func (s *serve) InitialConfig() error {
	if len(os.Getenv("LISTENER_ADDRESS")) > 0 {
		logPrintf("Requesting services from Docker Flow Swarm Listener")
		addr := os.Getenv("LISTENER_ADDRESS")
		if !strings.HasPrefix(addr, "http") {
			addr = fmt.Sprintf("http://%s:8080", addr)
		}
		addr = fmt.Sprintf("%s/v1/docker-flow-swarm-listener/get-services", addr)
		timeout := time.Duration(listenerTimeout)
		client := http.Client{Timeout: timeout}
		resp, err := client.Get(addr)
		if err != nil {
			return err
		}
		body, _ := ioutil.ReadAll(resp.Body)
		logPrintf("Processing: %s", string(body))
		data := []map[string]string{}
		json.Unmarshal(body, &data)
		for _, row := range data {
			if scrape, err := s.getScrapeFromMap(row); err == nil {
				s.scrapes[scrape.ServiceName] = scrape
			}
			if alert, err := s.getAlertFromMap(row, ""); err == nil {
				s.alerts[alert.AlertNameFormatted] = alert
			}
			for i := 1; i <= 10; i++ {
				suffix := fmt.Sprintf(".%d", i)
				if alert, err := s.getAlertFromMap(row, suffix); err == nil {
					s.alerts[alert.AlertNameFormatted] = alert
				} else {
					break
				}
			}
		}

		scrapeVariablesFromEnv := s.getScrapeVariablesFromEnv()
		if len(scrapeVariablesFromEnv) > 0 {
			scrape, err := s.parseScrapeFromEnvMap(scrapeVariablesFromEnv)
			if err != nil {
				return err
			}
			for _, row := range scrape {
				s.scrapes[row.ServiceName] = row
			}
		}
	}
	return nil
}

func (s *serve) getScrapeFromMap(data map[string]string) (prometheus.Scrape, error) {
	scrape := prometheus.Scrape{}
	if port, err := strconv.Atoi(data["scrapePort"]); err == nil {
		scrape.ScrapePort = port
	}
	scrape.ServiceName = data["serviceName"]
	scrape.ScrapeType = data["scrapeType"]

	if s.isValidScrape(&scrape) {
		return scrape, nil
	}
	return prometheus.Scrape{}, fmt.Errorf("Not a valid scrape")
}

func (s *serve) getAlertFromMap(data map[string]string, suffix string) (prometheus.Alert, error) {
	if _, ok := data["alertName"+suffix]; ok {
		alert := prometheus.Alert{}
		alert.AlertAnnotations = s.getMapFromString(data["alertAnnotations"+suffix])
		alert.AlertFor = data["alertFor"+suffix]
		alert.AlertIf = data["alertIf"+suffix]
		alert.AlertLabels = s.getMapFromString(data["alertLabels"+suffix])
		alert.AlertName = data["alertName"+suffix]
		alert.ServiceName = data["serviceName"]
		if len(data["replicas"]) > 0 {
			alert.Replicas, _ = strconv.Atoi(data["replicas"])
		}
		s.formatAlert(&alert)
		if s.isValidAlert(&alert) {
			return alert, nil
		}
	}
	return prometheus.Alert{}, fmt.Errorf("Not a valid alert")
}

func (s *serve) getMapFromString(value string) map[string]string {
	mappedValue := map[string]string{}
	if len(value) > 0 {
		for _, label := range strings.Split(value, ",") {
			values := strings.Split(label, "=")
			mappedValue[values[0]] = values[1]
		}
	}
	return mappedValue
}

func (s *serve) getAlerts(req *http.Request) []prometheus.Alert {
	alerts := []prometheus.Alert{}
	alertDecode := prometheus.Alert{}
	decoder.Decode(&alertDecode, req.Form)
	if s.isValidAlert(&alertDecode) {
		alertDecode.AlertAnnotations = s.getMapFromString(req.URL.Query().Get("alertAnnotations"))
		alertDecode.AlertLabels = s.getMapFromString(req.URL.Query().Get("alertLabels"))
		s.formatAlert(&alertDecode)
		s.alerts[alertDecode.AlertNameFormatted] = alertDecode
		alerts = append(alerts, alertDecode)
		logPrintf("Adding alert %s for the service %s\n", alertDecode.AlertName, alertDecode.ServiceName, alertDecode)
	}
	replicas := 0
	if len(req.URL.Query().Get("replicas")) > 0 {
		replicas, _ = strconv.Atoi(req.URL.Query().Get("replicas"))
	}
	for i := 1; i <= 10; i++ {
		alertName := req.URL.Query().Get(fmt.Sprintf("alertName.%d", i))
		annotations := s.getMapFromString(req.URL.Query().Get(fmt.Sprintf("alertAnnotations.%d", i)))
		labels := s.getMapFromString(req.URL.Query().Get(fmt.Sprintf("alertLabels.%d", i)))
		alert := prometheus.Alert{
			ServiceName:      alertDecode.ServiceName,
			AlertName:        alertName,
			AlertIf:          req.URL.Query().Get(fmt.Sprintf("alertIf.%d", i)),
			AlertFor:         req.URL.Query().Get(fmt.Sprintf("alertFor.%d", i)),
			AlertAnnotations: annotations,
			AlertLabels:      labels,
			Replicas:         replicas,
		}
		s.formatAlert(&alert)
		if !s.isValidAlert(&alert) {
			break
		}
		s.alerts[alert.AlertNameFormatted] = alert
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
		expanded:    `container_memory_usage_bytes{container_label_com_docker_swarm_service_name="[SERVICE_NAME]"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="[SERVICE_NAME]"} > [VALUE]`,
		shortcut:    `@service_mem_limit:`,
		annotations: map[string]string{"summary": "Memory of the service [SERVICE_NAME] is over [VALUE]"},
		labels:      map[string]string{"receiver": "system", "service": "[SERVICE_NAME]"},
	}, {
		expanded:    `(sum by (instance) (node_memory_MemTotal) - sum by (instance) (node_memory_MemFree + node_memory_Buffers + node_memory_Cached)) / sum by (instance) (node_memory_MemTotal) > [VALUE]`,
		shortcut:    `@node_mem_limit:`,
		annotations: map[string]string{"summary": "Memory of a node is over [VALUE]"},
		labels:      map[string]string{"receiver": "system", "service": "[SERVICE_NAME]"},
	}, {
		expanded:    `(sum(node_memory_MemTotal{job="[SERVICE_NAME]"}) - sum(node_memory_MemFree{job="[SERVICE_NAME]"} + node_memory_Buffers{job="[SERVICE_NAME]"} + node_memory_Cached{job="[SERVICE_NAME]"})) / sum(node_memory_MemTotal{job="[SERVICE_NAME]"}) > [VALUE]`,
		shortcut:    `@node_mem_limit_total_above:`,
		annotations: map[string]string{"summary": "Total memory of the nodes is over [VALUE]"},
		labels:      map[string]string{"receiver": "system", "service": "[SERVICE_NAME]", "scale": "up", "type": "node"},
	}, {
		expanded:    `(sum(node_memory_MemTotal{job="[SERVICE_NAME]"}) - sum(node_memory_MemFree{job="[SERVICE_NAME]"} + node_memory_Buffers{job="[SERVICE_NAME]"} + node_memory_Cached{job="[SERVICE_NAME]"})) / sum(node_memory_MemTotal{job="[SERVICE_NAME]"}) < [VALUE]`,
		shortcut:    `@node_mem_limit_total_below:`,
		annotations: map[string]string{"summary": "Total memory of the nodes is below [VALUE]"},
		labels:      map[string]string{"receiver": "system", "service": "[SERVICE_NAME]", "scale": "down", "type": "node"},
	}, {
		expanded:    `(node_filesystem_size{fstype="aufs"} - node_filesystem_free{fstype="aufs"}) / node_filesystem_size{fstype="aufs"} > [VALUE]`,
		shortcut:    `@node_fs_limit:`,
		annotations: map[string]string{"summary": "Disk usage of a node is over [VALUE]"},
		labels:      map[string]string{"receiver": "system", "service": "[SERVICE_NAME]"},
	}, {
		expanded:    `sum(rate(http_server_resp_time_bucket{job="[SERVICE_NAME]", le="[VALUE_0]"}[[VALUE_1]])) / sum(rate(http_server_resp_time_count{job="[SERVICE_NAME]"}[[VALUE_1]])) < [VALUE_2]`,
		shortcut:    `@resp_time_above:`,
		annotations: map[string]string{"summary": "Response time of the service [SERVICE_NAME] is above [VALUE_0]"},
		labels:      map[string]string{"receiver": "system", "service": "[SERVICE_NAME]", "scale": "up", "type": "service"},
	}, {
		expanded:    `sum(rate(http_server_resp_time_bucket{job="[SERVICE_NAME]", le="[VALUE_0]"}[[VALUE_1]])) / sum(rate(http_server_resp_time_count{job="[SERVICE_NAME]"}[[VALUE_1]])) > [VALUE_2]`,
		shortcut:    `@resp_time_below:`,
		annotations: map[string]string{"summary": "Response time of the service [SERVICE_NAME] is below [VALUE_0]"},
		labels:      map[string]string{"receiver": "system", "service": "[SERVICE_NAME]", "scale": "down", "type": "service"},
	}, {
		expanded:    `count(container_memory_usage_bytes{container_label_com_docker_swarm_service_name="[SERVICE_NAME]"}) != [REPLICAS]`,
		shortcut:    `@replicas_running`,
		annotations: map[string]string{"summary": "The number of running replicas of the service [SERVICE_NAME] is not [REPLICAS]"},
		labels:      map[string]string{"receiver": "system", "service": "[SERVICE_NAME]", "scale": "up", "type": "node"},
	}, {
		expanded:    `count(container_memory_usage_bytes{container_label_com_docker_swarm_service_name="[SERVICE_NAME]"}) < [REPLICAS]`,
		shortcut:    `@replicas_less_than`,
		annotations: map[string]string{"summary": "The number of running replicas of the service [SERVICE_NAME] is less than [REPLICAS]"},
		labels:      map[string]string{"receiver": "system", "service": "[SERVICE_NAME]", "scale": "up", "type": "node"},
	}, {
		expanded:    `count(container_memory_usage_bytes{container_label_com_docker_swarm_service_name="[SERVICE_NAME]"}) > [REPLICAS]`,
		shortcut:    `@replicas_more_than`,
		annotations: map[string]string{"summary": "The number of running replicas of the service [SERVICE_NAME] is more than [REPLICAS]"},
		labels:      map[string]string{"receiver": "system", "service": "[SERVICE_NAME]", "scale": "up", "type": "node"},
	}, {
		expanded:    `sum(rate(http_server_resp_time_count{job="[SERVICE_NAME]", code=~"^5..$$"}[[VALUE_0]])) / sum(rate(http_server_resp_time_count{job="[SERVICE_NAME]"}[[VALUE_0]])) > [VALUE_1]`,
		shortcut:    `@resp_time_server_error:`,
		annotations: map[string]string{"summary": "Error rate of the service [SERVICE_NAME] is above [VALUE_1]"},
		labels:      map[string]string{"receiver": "system", "service": "[SERVICE_NAME]", "type": "errors"},
	},
}

func (s *serve) formatAlert(alert *prometheus.Alert) {
	alert.AlertNameFormatted = s.getNameFormatted(fmt.Sprintf("%s_%s", alert.ServiceName, alert.AlertName))
	if strings.HasPrefix(alert.AlertIf, "@") {
		for _, data := range alertIfShortcutData {
			if strings.HasPrefix(alert.AlertIf, data.shortcut) {
				value := ""
				if strings.Contains(alert.AlertIf, ":") {
					value = strings.Split(alert.AlertIf, ":")[1]
				}
				alert.AlertIf = s.replaceTags(data.expanded, alert, value)
				if alert.AlertAnnotations == nil {
					alert.AlertAnnotations = map[string]string{}
				}
				for k, v := range data.annotations {
					if _, ok := alert.AlertAnnotations[k]; !ok {
						alert.AlertAnnotations[k] = s.replaceTags(v, alert, value)
					}
				}
				if alert.AlertLabels == nil {
					alert.AlertLabels = map[string]string{}
				}
				for k, v := range data.labels {
					if _, ok := alert.AlertLabels[k]; !ok {
						alert.AlertLabels[k] = s.replaceTags(v, alert, value)
					}
				}
			}
		}
	}
}

// TODO: Change to template
func (s *serve) replaceTags(tag string, alert *prometheus.Alert, value string) string {
	replaced := strings.Replace(tag, "[SERVICE_NAME]", alert.ServiceName, -1)
	replaced = strings.Replace(replaced, "[VALUE]", value, -1)
	replaced = strings.Replace(replaced, "[REPLICAS]", strconv.Itoa(alert.Replicas), -1)
	values := strings.Split(value, ",")
	for i, v := range values {
		old := fmt.Sprintf("[VALUE_%d]", i)
		replaced = strings.Replace(replaced, old, v, -1)
	}
	return replaced
}

func (s *serve) isValidAlert(alert *prometheus.Alert) bool {
	return len(alert.AlertName) > 0 && len(alert.AlertIf) > 0
}

func (s *serve) deleteAlerts(serviceName string) []prometheus.Alert {
	alerts := []prometheus.Alert{}
	serviceNameFormatted := s.getNameFormatted(serviceName)
	for k, v := range s.alerts {
		if strings.HasPrefix(k, serviceNameFormatted) {
			alerts = append(alerts, v)
			delete(s.alerts, k)
		}
	}
	return alerts
}

func (s *serve) getNameFormatted(name string) string {
	return strings.Replace(name, "-", "", -1)
}

func (s *serve) getScrape(req *http.Request) prometheus.Scrape {
	scrape := prometheus.Scrape{}
	decoder.Decode(&scrape, req.Form)
	if s.isValidScrape(&scrape) {
		s.scrapes[scrape.ServiceName] = scrape
		logPrintf("Adding scrape %s\n%v", scrape.ServiceName, scrape)
	}
	return scrape
}

func (s *serve) isValidScrape(scrape *prometheus.Scrape) bool {
	return len(scrape.ServiceName) > 0 && scrape.ScrapePort > 0
}

func (s *serve) getResponse(alerts *[]prometheus.Alert, scrape *prometheus.Scrape, err error, statusCode int) response {
	resp := response{
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

func (s *serve) getScrapeVariablesFromEnv() map[string]string {
	scrapeVariablesPrefix := []string{
		scrapePort,
		serviceName,
	}

	scrapesVariables := map[string]string{}
	for _, e := range os.Environ() {
		if key, value := getScrapeFromEnv(e, scrapeVariablesPrefix); len(key) > 0 {
			scrapesVariables[key] = value
		}
	}

	return scrapesVariables
}

func (s *serve) parseScrapeFromEnvMap(data map[string]string) ([]prometheus.Scrape, error) {
	count := len(data) / 2

	// If an odd number was find in the environment variables it means it is missing variables
	if len(data)%2 != 0 {
		msg := fmt.Errorf("SCRAPE_PORT_* and SERVICE_NAME_* environment variable configuration are not valid.")
		return []prometheus.Scrape{}, msg
	}

	scrapeFromEnv := []prometheus.Scrape{}
	for i := 1; i <= count; i++ {

		index := strconv.Itoa(i)
		if len(data[serviceName+"_"+index]) > 0 && len(data[scrapePort+"_"+index]) > 0 {
			scrapePort, err := strconv.Atoi(data[scrapePort+"_"+index])
			if err != nil {
				return []prometheus.Scrape{}, err
			}

			scrapeFromEnv = append(scrapeFromEnv, prometheus.Scrape{
				ScrapePort:  scrapePort,
				ServiceName: data[serviceName+"_"+index],
				ScrapeType:  "static_configs",
			})
		}

	}

	return scrapeFromEnv, nil
}
