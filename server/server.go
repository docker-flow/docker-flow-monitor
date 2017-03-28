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
// TODO: Test
var cmdRun = func(cmd *exec.Cmd) error {
	return cmd.Run()
}

type Alert struct {
	AlertName string
	AlertIf   string
	AlertFrom string
}

type Scrape struct {
	ServiceName string `json:"serviceName,omitempty"`
	ScrapePort 	int `json:"scrapePort,string,omitempty"`
}

type Serve struct {
	Scrapes map[string]Scrape
	Alerts map[string]Alert
}

type Response struct {
	Status      int
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
	s.InitialConfig()
	go s.RunPrometheus()
	address := "0.0.0.0:8080"
	r := mux.NewRouter().StrictSlash(true)
	// TODO: Add DELETE method
	r.HandleFunc("/v1/docker-flow-monitor/reconfigure", s.Handler).Methods("GET")
	log.Println("Starting Docker Flow Monitor")
	if err := httpListenAndServe(address, r); err != nil {
		log.Println(err.Error())
		return err
	}
	return nil
}

func (s *Serve) Handler(w http.ResponseWriter, req *http.Request) {
	log.Println("Processing " + req.URL.Path)
	req.ParseForm()
	// TODO: Add serviceName to the alertName
	// TODO: Create alert configs
	// TODO: Handle multiple alerts
	alert := s.getAlert(req)
	scrape := s.getScrape(req)
	s.WriteConfig()
	promResp, err := s.reloadPrometheus()
	statusCode := http.StatusInternalServerError
	if promResp != nil {
		statusCode = promResp.StatusCode
	}
	resp := s.getResponse(&alert, &scrape, err, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	js, _ := json.Marshal(resp)
	w.Write(js)
}

func (s *Serve) WriteConfig() {
	log.Println("Writing config")
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

func (s *Serve) RunPrometheus() error {
	log.Println("Starting Prometheus")
	cmd := exec.Command("/bin/sh", "-c", "prometheus -config.file=/etc/prometheus/prometheus.yml -storage.local.path=/prometheus -web.console.libraries=/usr/share/prometheus/console_libraries -web.console.templates=/usr/share/prometheus/consoles")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmdRun(cmd)
}

func (s *Serve) InitialConfig() error {
	if len(os.Getenv("LISTENER_ADDRESS")) > 0 {
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

func (s *Serve) getAlert(req *http.Request) Alert {
	alert := Alert{}
	decoder.Decode(&alert, req.Form)
	if len(alert.AlertName) > 0 {
		alert.AlertName = strings.Replace(alert.AlertName, "-", "", -1)
		alert.AlertName = strings.Replace(alert.AlertName, "_", "", -1)
		s.Alerts[alert.AlertName] = alert
		log.Println("Adding alert " + alert.AlertName)
	}
	return alert
}

func (s *Serve) getScrape(req *http.Request) Scrape {
	scrape := Scrape{}
	decoder.Decode(&scrape, req.Form)
	if len(scrape.ServiceName) > 0 {
		s.Scrapes[scrape.ServiceName] = scrape
		log.Println("Adding scrape " + scrape.ServiceName)
	}
	return scrape
}

func (s *Serve) reloadPrometheus() (*http.Response, error) {
	return http.Post(prometheusAddr + "/-/reload", "application/json", nil)
}

func (s *Serve) getResponse(alert *Alert, scrape *Scrape, err error, statusCode int) Response {
	resp := Response{
		Status: statusCode,
		Alert: *alert,
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