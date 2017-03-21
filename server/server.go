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
)

var decoder = schema.NewDecoder()
var mu = &sync.Mutex{}
var fs = afero.NewOsFs()
var prometheusAddr = "http://localhost:9090"

type Alert struct {
	AlertName string
	AlertIf   string
	AlertFrom string
}

type Scrape struct {
	ServiceName string
	ScrapePort 	int
}

type Serve struct {
	Scrapes map[string]Scrape
	Alerts map[string]Alert
}

type Response struct {
	Status      string
	Message     string
	Alert
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
	// TODO: Request initial data from swarm-listener
	address := "0.0.0.0:8080"
	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/v1/docker-flow-monitor", s.Handler).Methods("GET")
	// TODO: Add DELETE method
	if err := httpListenAndServe(address, r); err != nil {
		log.Println(err.Error())
		return err
	}
	return nil
}

func (s *Serve) Handler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	alert := Alert{}
	scrape := Scrape{}
	decoder.Decode(&alert, req.Form)
	// TODO: Add serviceName to the alertName
	// TODO: Create alert configs
	// TODO: Handle multiple alerts
	if len(alert.AlertName) > 0 {
		alert.AlertName = strings.Replace(alert.AlertName, "-", "", -1)
		alert.AlertName = strings.Replace(alert.AlertName, "_", "", -1)
		s.Alerts[alert.AlertName] = alert
	}
	decoder.Decode(&scrape, req.Form)
	if len(scrape.ServiceName) > 0 {
		s.Scrapes[scrape.ServiceName] = scrape
	}
	s.WriteConfig()
	response := Response{
		Status: "OK",
		Alert: alert,
		Scrape: scrape,
	}
	promReq, _ := http.NewRequest("POST", prometheusAddr + "/-/reload", nil)
	client := &http.Client{}
	status := http.StatusOK
	if resp, err := client.Do(promReq); err != nil {
		status = http.StatusInternalServerError
		response.Message = err.Error()
	} else if resp.StatusCode != http.StatusOK {
		status = resp.StatusCode
		response.Message = fmt.Sprintf("Prometheus returned status code %d", status)
	}
	if status != http.StatusOK {
		response.Status = "NOK"
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	js, _ := json.Marshal(response)
	w.Write(js)
}

func (s *Serve) WriteConfig() {
	mu.Lock()
	defer mu.Unlock()
	fs.MkdirAll("/etc/prometheus", 0755)
	gc, _ := s.GetGlobalConfig()
	sc := s.GetScrapeConfig()
	config := fmt.Sprintf(`%s
%s`,
		gc,
		sc,
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
	templateString := `{{range .}}
scrape_configs:
  - job_name: "{{.ServiceName}}"
    dns_sd_configs:
      - names: ["tasks.{{.ServiceName}}"]
        type: A
        port: {{.ScrapePort}}
{{end}}`
	tmpl, _ := template.New("").Parse(templateString)
	var b bytes.Buffer
	tmpl.Execute(&b, s.Scrapes)
	return b.String()
}

func (s *Serve) GetAlertConfig() string {
	templateString := `{{range .}}
ALERT {{.AlertName}}
  IF {{.AlertIf}}{{if .AlertFrom}}
  FROM {{.AlertFrom}}{{end}}
{{end}}`
	tmpl, _ := template.New("").Parse(templateString)
	var b bytes.Buffer
	tmpl.Execute(&b, s.Alerts)
	return b.String()
}
