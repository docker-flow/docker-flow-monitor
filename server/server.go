package server

import (
	"net/http"
	"github.com/gorilla/schema"
	"github.com/gorilla/mux"
	"log"
	"encoding/json"
	"sync"
	"github.com/spf13/afero"
	"text/template"
	"bytes"
	"strings"
	"fmt"
	"os"
	"os/exec"
	"io/ioutil"
)

var decoder = schema.NewDecoder()
var mu = &sync.Mutex{}
var fs = afero.NewOsFs()
var prometheusAddr = "http://localhost:9090"
var cmdRun = func(cmd *exec.Cmd) error {
	return cmd.Run()
}
var logPrintf = log.Printf

type Alert struct {
	AlertName string `json:"alertName"`
	AlertNameFormatted string
	AlertFor string `json:"alertFor,omitempty"`
	AlertIf   string `json:"alertIf,omitempty"`
	ServiceName string `json:"serviceName"`
}

type Scrape struct {
	ScrapePort 	int `json:"scrapePort,string,omitempty"`
	ServiceName string `json:"serviceName"`
}

type Serve struct {
	Scrapes map[string]Scrape
	Alerts map[string]Alert
}

type Response struct {
	Status      int
	Message     string
	Alerts      []Alert
	Scrape
}

var httpListenAndServe = http.ListenAndServe

var New = func() *Serve {
	return &Serve{
		Alerts: make(map[string]Alert),
		Scrapes: make(map[string]Scrape),
	}
}

func (s *Serve) Execute() error {
	s.InitialConfig()
	s.WriteConfig()
	go s.RunPrometheus()
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
	s.WriteConfig()
	err := s.reloadPrometheus()
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
	s.WriteConfig()
	err := s.reloadPrometheus()
	statusCode := http.StatusOK
	resp := s.getResponse(&alerts, &scrape, err, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	js, _ := json.Marshal(resp)
	w.Write(js)
}

func (s *Serve) WriteConfig() {
	fs.MkdirAll("/etc/prometheus", 0755)
	gc := s.GetGlobalConfig()
	sc := s.GetScrapeConfig()
	ruleFiles := ""
	if len(s.Alerts) > 0 {
		logPrintf("Writing to alert.rules")
		ruleFiles = `
rule_files:
  - 'alert.rules'
`
		afero.WriteFile(fs, "/etc/prometheus/alert.rules", []byte(s.GetAlertConfig()), 0644)
	}
	config := fmt.Sprintf(`%s
%s%s`,
		gc,
		sc,
		ruleFiles,
	)
	logPrintf("Writing to prometheus.yml")
	afero.WriteFile(fs, "/etc/prometheus/prometheus.yml", []byte(config), 0644)
}

func (s *Serve) GetGlobalConfig() string {
	config := `
global:`
	for _, e := range os.Environ() {
		if key, value := s.getArgFromEnv(e, "GLOBAL"); len(key) > 0 {
			config = fmt.Sprintf(
				`%s
  %s: %s`,
				config,
				key,
				value,
			)
		}
	}
	return config
}

func (s *Serve) GetScrapeConfig() string {
	if len(s.Scrapes) == 0 {
		return ""
	}
	templateString := `
scrape_configs:{{range .}}
  - job_name: "{{.ServiceName}}"
    dns_sd_configs:
      - names: ["tasks.{{.ServiceName}}"]
        type: A
        port: {{.ScrapePort}}{{end}}
`
	tmpl, _ := template.New("").Parse(templateString)
	var b bytes.Buffer
	tmpl.Execute(&b, s.Scrapes)
	return b.String()
}

func (s *Serve) GetAlertConfig() string {
	// TODO: Add ANNOTATIONS
	templateString := `{{range .}}
ALERT {{.AlertNameFormatted}}
  IF {{.AlertIf}}{{if .AlertFor}}
  FOR {{.AlertFor}}{{end}}
{{end}}`
	tmpl, _ := template.New("").Parse(templateString)
	var b bytes.Buffer
	tmpl.Execute(&b, s.Alerts)
	return b.String()
}

func (s *Serve) RunPrometheus() error {
	logPrintf("Starting Prometheus")
	cmdString := "prometheus"
	for _, e := range os.Environ() {
		if key, value := s.getArgFromEnv(e, "ARG"); len(key) > 0 {
			key = strings.Replace(key, "_", ".", -1)
			cmdString = fmt.Sprintf("%s -%s=%s", cmdString, key, value)
		}
	}
	cmd := exec.Command("/bin/sh", "-c", cmdString)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmdRun(cmd)
}

func (s *Serve) InitialConfig() error {
	if len(os.Getenv("LISTENER_ADDRESS")) > 0 {
		logPrintf("Requesting services from Docker Flow Swarm Listener")
		addr := os.Getenv("LISTENER_ADDRESS")
		if !strings.HasPrefix(addr, "http") {
			addr = fmt.Sprintf("http://%s:8080")
		}
		addr = fmt.Sprintf("%s/v1/docker-flow-swarm-listener/get-services", addr)
		resp, err := http.Get(addr)
		if err != nil {
			return err
		}
		body, _ := ioutil.ReadAll(resp.Body)
		scrapes := []Scrape{}
		json.Unmarshal(body, &scrapes)
		for _, scrape := range scrapes {
			s.Scrapes[scrape.ServiceName] = scrape
		}
		alerts := []Alert{}
		json.Unmarshal(body, &alerts)
		for _, alert := range alerts {
			s.Alerts[alert.AlertName] = alert
		}
	}
	return nil
}

func (s *Serve) getAlerts(req *http.Request) []Alert {
	alerts := []Alert{}
	alertDecode := Alert{}
	decoder.Decode(&alertDecode, req.Form)
	if len(alertDecode.AlertName) > 0 {
		alertDecode.AlertNameFormatted = s.getAlertNameFormatted(alertDecode.ServiceName, alertDecode.AlertName)
		s.Alerts[alertDecode.AlertNameFormatted] = alertDecode
		alerts = append(alerts, alertDecode)
		logPrintf("Adding alert %s for the service %s", alertDecode.AlertName, alertDecode.ServiceName)
	}
	for i:=1; i <= 10; i++ {
		alertName := req.URL.Query().Get(fmt.Sprintf("alertName.%d", i))
		if len(alertName) == 0 {
			break
		}
		alert := Alert{
			AlertNameFormatted: s.getAlertNameFormatted(alertDecode.ServiceName, alertName),
			ServiceName: alertDecode.ServiceName,
			AlertName: alertName,
			AlertIf: req.URL.Query().Get(fmt.Sprintf("alertIf.%d", i)),
			AlertFor: req.URL.Query().Get(fmt.Sprintf("alertFor.%d", i)),
		}
		s.Alerts[alert.AlertNameFormatted] = alert
		alerts = append(alerts, alert)
	}
	return alerts
}

func (s *Serve) deleteAlerts(serviceName string) []Alert {
	alerts := []Alert{}
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

func (s *Serve) getScrape(req *http.Request) Scrape {
	scrape := Scrape{}
	decoder.Decode(&scrape, req.Form)
	if len(scrape.ServiceName) > 0 && scrape.ScrapePort > 0 {
		s.Scrapes[scrape.ServiceName] = scrape
		logPrintf("Adding scrape " + scrape.ServiceName)
	}
	return scrape
}

func (s *Serve) reloadPrometheus() error {
	logPrintf("Reloading Prometheus")
	cmd := exec.Command("pkill", "-HUP", "prometheus")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmdRun(cmd)
}

func (s *Serve) getResponse(alerts *[]Alert, scrape *Scrape, err error, statusCode int) Response {
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

func (s *Serve) getArgFromEnv(env, prefix string) (key, value string) {
	if strings.HasPrefix(env, prefix + "_") {
		values := strings.Split(env, "=")
		key = values[0]
		key = strings.TrimLeft(key, prefix)
		key = strings.ToLower(key)
		key = strings.Replace(key, "_", "", 1)
		value = values[1]
	}
	return key, value
}