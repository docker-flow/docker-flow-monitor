package server

import (
	"net/http"
	"github.com/gorilla/schema"
	"github.com/gorilla/mux"
	"log"
	"encoding/json"
	"sync"
	"github.com/spf13/afero"
	"html/template"
	"bytes"
	"strings"
	"fmt"
	"os"
	"strconv"
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
	AlertFrom string `json:"alertFrom,omitempty"`
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
	go s.RunPrometheus()
	address := "0.0.0.0:8080"
	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/v1/docker-flow-monitor/reconfigure", s.GetHandler).Methods("GET")
	r.HandleFunc("/v1/docker-flow-monitor/reconfigure", s.DeleteHandler).Methods("DELETE")
	logPrintf("Starting Docker Flow Monitor")
	if err := httpListenAndServe(address, r); err != nil {
		logPrintf(err.Error())
		return err
	}
	return nil
}

func (s *Serve) GetHandler(w http.ResponseWriter, req *http.Request) {
	logPrintf("Processing " + req.URL.Path)
	req.ParseForm()
	alerts := s.getAlerts(req)
	scrape := s.getScrape(req)
	s.WriteConfig()
	promResp, err := s.reloadPrometheus()
	statusCode := http.StatusInternalServerError
	if promResp != nil {
		statusCode = promResp.StatusCode
	}
	resp := s.getResponse(&alerts, &scrape, err, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	js, _ := json.Marshal(resp)
	w.Write(js)
}

func (s *Serve) DeleteHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	serviceName := req.URL.Query().Get("serviceName")
	scrape := s.Scrapes[serviceName]
	delete(s.Scrapes, serviceName)
	alerts := s.deleteAlerts(serviceName)
	s.WriteConfig()
	promResp, err := s.reloadPrometheus()
	statusCode := http.StatusInternalServerError
	if promResp != nil {
		statusCode = promResp.StatusCode
	}
	resp := s.getResponse(&alerts, &scrape, err, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	js, _ := json.Marshal(resp)
	w.Write(js)
}

func (s *Serve) WriteConfig() {
	logPrintf("Writing config")
	mu.Lock()
	defer mu.Unlock()
	fs.MkdirAll("/etc/prometheus", 0755)
	gc, _ := s.GetGlobalConfig()
	sc := s.GetScrapeConfig()
	ruleFiles := ""
	if len(s.Alerts) > 0 {
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
	afero.WriteFile(fs, "/etc/prometheus/prometheus.yml", []byte(config), 0644)
}

func (s *Serve) GetGlobalConfig() (config string, err error) {
	scrapeInterval := 5
	if len(os.Getenv("SCRAPE_INTERVAL")) > 0 {
		scrapeInterval, err = strconv.Atoi(os.Getenv("SCRAPE_INTERVAL"))
	}
	return fmt.Sprintf(`
global:
  scrape_interval: %ds`,
		scrapeInterval,
	), err
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
  IF {{.AlertIf}}{{if .AlertFrom}}
  FROM {{.AlertFrom}}{{end}}
{{end}}`
	tmpl, _ := template.New("").Parse(templateString)
	var b bytes.Buffer
	tmpl.Execute(&b, s.Alerts)
	return b.String()
}

func (s *Serve) RunPrometheus() error {
	logPrintf("Starting Prometheus")
	cmd := exec.Command("/bin/sh", "-c", "prometheus -config.file=/etc/prometheus/prometheus.yml -storage.local.path=/prometheus -web.console.libraries=/usr/share/prometheus/console_libraries -web.console.templates=/usr/share/prometheus/consoles")
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
	s.WriteConfig()
	return nil
}

func (s *Serve) getAlerts(req *http.Request) []Alert {
	alerts := []Alert{}
	alertFromDecode := Alert{}
	decoder.Decode(&alertFromDecode, req.Form)
	if len(alertFromDecode.AlertName) > 0 {
		alertFromDecode.AlertNameFormatted = s.getAlertNameFormatted(alertFromDecode.ServiceName, alertFromDecode.AlertName)
		s.Alerts[alertFromDecode.AlertNameFormatted] = alertFromDecode
		alerts = append(alerts, alertFromDecode)
		logPrintf("Adding alert %s for the service %s", alertFromDecode.AlertName, alertFromDecode.ServiceName)
	}
	for i:=1; i <= 10; i++ {
		alertName := req.URL.Query().Get(fmt.Sprintf("alertName.%d", i))
		if len(alertName) == 0 {
			break
		}
		alert := Alert{
			AlertNameFormatted: s.getAlertNameFormatted(alertFromDecode.ServiceName, alertName),
			ServiceName: alertFromDecode.ServiceName,
			AlertName: alertName,
			AlertIf: req.URL.Query().Get(fmt.Sprintf("alertIf.%d", i)),
			AlertFrom: req.URL.Query().Get(fmt.Sprintf("alertFrom.%d", i)),
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

func (s *Serve) reloadPrometheus() (*http.Response, error) {
	return http.Post(prometheusAddr + "/-/reload", "application/json", nil)
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
	} else if statusCode != http.StatusOK {
		resp.Message = fmt.Sprintf("Prometheus returned status code %d", statusCode)
	}
	return resp
}