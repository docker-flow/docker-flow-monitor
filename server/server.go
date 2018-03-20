package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"text/template"
	"time"

	"../prometheus"
	"github.com/gorilla/mux"
	"github.com/gorilla/schema"
	"github.com/spf13/afero"
	yaml "gopkg.in/yaml.v2"
)

// FS defines file system used to read and write configuration files
var FS = afero.NewOsFs()
var decoder = schema.NewDecoder()
var mu = &sync.Mutex{}
var logPrintf = log.Printf
var listenerTimeout = 30 * time.Second
var shortcutsPath = "/etc/dfm/shortcuts.yaml"
var alertIfShortcutData map[string]AlertIfShortcut

type serve struct {
	scrapes    map[string]prometheus.Scrape
	alerts     map[string]prometheus.Alert
	nodeLabels map[string]map[string]string
	configPath string
}

type response struct {
	Status  int
	Message string
	Alerts  []prometheus.Alert
	prometheus.Scrape
}

type nodeResponse struct {
	Status    int
	NodeID    string
	Message   string
	NodeLabel map[string]string
}

var httpListenAndServe = http.ListenAndServe

const scrapePort = "SCRAPE_PORT"
const serviceName = "SERVICE_NAME"

// New returns instance of the `serve` structure
var New = func() *serve {
	promConfig := os.Getenv("ARG_CONFIG_FILE")
	if len(promConfig) == 0 {
		promConfig = "/etc/prometheus/prometheus.yml"
	}
	alertIfShortcutData = GetShortcuts()
	return &serve{
		alerts:     make(map[string]prometheus.Alert),
		scrapes:    make(map[string]prometheus.Scrape),
		nodeLabels: make(map[string]map[string]string),
		configPath: promConfig,
	}
}

func (s *serve) Execute() error {
	s.InitialConfig()
	prometheus.WriteConfig(s.configPath, s.scrapes, s.alerts, s.nodeLabels)
	go prometheus.Run()
	address := "0.0.0.0:8080"
	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/v1/docker-flow-monitor/reconfigure", s.ReconfigureHandler)
	r.HandleFunc("/v1/docker-flow-monitor/remove", s.RemoveHandler)
	r.HandleFunc("/v1/docker-flow-monitor/node/reconfigure", s.ReconfigureNodeHandler)
	r.HandleFunc("/v1/docker-flow-monitor/node/remove", s.RemoveNodeHandler)
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
	s.deleteAlerts(scrape.ServiceName, false)
	alerts := s.getAlerts(req)
	prometheus.WriteConfig(s.configPath, s.scrapes, s.alerts, s.nodeLabels)
	err := prometheus.Reload()
	statusCode := http.StatusOK
	resp := s.getResponse(&alerts, &scrape, err, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	js, _ := json.Marshal(resp)
	w.Write(js)
}

func (s *serve) RemoveHandler(w http.ResponseWriter, req *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	logPrintf("Processing " + req.URL.Path)
	req.ParseForm()
	serviceName := req.URL.Query().Get("serviceName")
	scrape := s.scrapes[serviceName]
	delete(s.scrapes, serviceName)
	alerts := s.deleteAlerts(serviceName, true)
	prometheus.WriteConfig(s.configPath, s.scrapes, s.alerts, s.nodeLabels)
	err := prometheus.Reload()
	statusCode := http.StatusOK
	resp := s.getResponse(&alerts, &scrape, err, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.Status)
	js, _ := json.Marshal(resp)
	w.Write(js)
}

func (s *serve) ReconfigureNodeHandler(w http.ResponseWriter, req *http.Request) {
	mu.Lock()
	defer mu.Unlock()
	logPrintf("Processing " + req.URL.String())
	req.ParseForm()
	nodeID, nodeLabel, err := s.getNodeLabel(req)
	if err != nil {
		status := http.StatusBadRequest
		resp := s.getNoIDNodeResponse(err, status)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		js, _ := json.Marshal(resp)
		w.Write(js)
		return
	}

	prometheus.WriteConfig(s.configPath, s.scrapes, s.alerts, s.nodeLabels)
	err = prometheus.Reload()
	statusCode := http.StatusOK
	resp := s.getNodeResponse(nodeID, nodeLabel, err, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	js, _ := json.Marshal(resp)
	w.Write(js)
}

func (s *serve) RemoveNodeHandler(w http.ResponseWriter, req *http.Request) {

	mu.Lock()
	defer mu.Unlock()
	logPrintf("Processing " + req.URL.String())
	req.ParseForm()
	nodeID := req.URL.Query().Get("id")
	if len(nodeID) == 0 {
		status := http.StatusBadRequest
		resp := s.getNoIDNodeResponse(errors.New("node id not in query"), status)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		js, _ := json.Marshal(resp)
		w.Write(js)
		return
	}
	nodeLabel := s.nodeLabels[nodeID]
	delete(s.nodeLabels, nodeID)

	prometheus.WriteConfig(s.configPath, s.scrapes, s.alerts, s.nodeLabels)
	err := prometheus.Reload()
	statusCode := http.StatusOK
	resp := s.getNodeResponse(nodeID, nodeLabel, err, statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
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

	// Get Nodes
	// Will return nil for errors since nodeLabels are not needed for DFM to function
	if len(os.Getenv("DF_GET_NODES_URL")) > 0 {
		logPrintf("Requesting nodes from Docker Flow Swarm Listener")
		timeout := time.Duration(listenerTimeout)
		client := http.Client{Timeout: timeout}

		resp, err := client.Get(os.Getenv("DF_GET_NODES_URL"))
		if err != nil {
			logPrintf("Error with node request: %v", err)
			return nil
		}
		body, err := ioutil.ReadAll(resp.Body)
		defer resp.Body.Close()
		if err != nil {
			logPrintf("Error with respone body parsing: %v", err)
			return nil
		}
		logPrintf("Processing: %s", string(body))
		data := []map[string]string{}
		json.Unmarshal(body, &data)

		if len(os.Getenv("DF_NODE_TARGET_LABELS")) == 0 {
			return nil
		}
		nodeTargetLabels := s.getTargetLabelsFromEnv()
		for _, row := range data {
			nodeID, ok := row["id"]
			if !ok {
				continue
			}

			labels := map[string]string{}
			for _, targetLabel := range nodeTargetLabels {
				if v, ok := row[targetLabel]; ok {
					labels[targetLabel] = v
				}
			}
			s.nodeLabels[nodeID] = labels
		}
	}

	return nil
}

func (s *serve) getScrapeFromMap(data map[string]string) (prometheus.Scrape, error) {
	scrape := prometheus.Scrape{}
	if port, err := strconv.Atoi(data["scrapePort"]); err == nil {
		scrape.ScrapePort = port
	}
	scrape.ScrapeInterval = data["scrapeInterval"]
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
		persistent := req.URL.Query().Get(fmt.Sprintf("alertPersistent.%d", i)) == "true"

		alert := prometheus.Alert{
			ServiceName:      alertDecode.ServiceName,
			AlertName:        alertName,
			AlertIf:          req.URL.Query().Get(fmt.Sprintf("alertIf.%d", i)),
			AlertFor:         req.URL.Query().Get(fmt.Sprintf("alertFor.%d", i)),
			AlertAnnotations: annotations,
			AlertLabels:      labels,
			Replicas:         replicas,
			AlertPersistent:  persistent,
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

// AlertIfShortcut defines how to expand a alertIf shortcut
type AlertIfShortcut struct {
	Expanded    string            `yaml:"expanded"`
	Annotations map[string]string `yaml:"annotations"`
	Labels      map[string]string `yaml:"labels"`
}

type alertTemplateInput struct {
	Alert  *prometheus.Alert
	Values []string
}

// GetShortcuts returns shortcuts from a YAML file
func GetShortcuts() map[string]AlertIfShortcut {
	yamlData, err := afero.ReadFile(FS, shortcutsPath)
	if err != nil {
		logPrintf(err.Error())
		return map[string]AlertIfShortcut{}
	}
	shortcuts := map[string]AlertIfShortcut{}
	err = yaml.Unmarshal(yamlData, &shortcuts)

	if err != nil {
		logPrintf(err.Error())
		return map[string]AlertIfShortcut{}
	}

	if isDir, err := afero.IsDir(FS, "/run/secrets"); err != nil || !isDir {
		return shortcuts
	}

	// Load alertIf shortcuts from secrets
	files, err := afero.ReadDir(FS, "/run/secrets")
	if err != nil {
		logPrintf(err.Error())
		return shortcuts
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		lName := strings.ToLower(file.Name())
		if !strings.HasPrefix(lName, "alertif-") &&
			!strings.HasPrefix(lName, "alertif_") {
			continue
		}

		path := fmt.Sprintf("/run/secrets/%s", file.Name())
		yamlData, err = afero.ReadFile(FS, path)
		if err != nil {
			logPrintf("Unable to read %s, error: %v", path, err)
			continue
		}

		secretShortcuts := map[string]AlertIfShortcut{}
		err = yaml.Unmarshal(yamlData, &secretShortcuts)
		if err != nil {
			logPrintf("YAML decoding reading %s, error: %v", path, err)
			continue
		}

		for k, v := range secretShortcuts {
			shortcuts[k] = v
		}
	}

	return shortcuts
}

func (s *serve) formatAlert(alert *prometheus.Alert) {
	alert.AlertNameFormatted = s.getNameFormatted(fmt.Sprintf("%s_%s", alert.ServiceName, alert.AlertName))
	if !strings.HasPrefix(alert.AlertIf, "@") {
		return
	}

	_, bOp, _ := splitCompoundOp(alert.AlertIf)
	if len(bOp) > 0 {
		formatCompoundAlert(alert)
	} else {
		formatSingleAlert(alert)
	}

}

func formatSingleAlert(alert *prometheus.Alert) {

	value := ""
	alertSplit := strings.Split(alert.AlertIf, ":")
	shortcut := alertSplit[0]

	if len(alertSplit) > 1 {
		value = alertSplit[1]
	}

	data, ok := alertIfShortcutData[shortcut]
	if !ok {
		return
	}

	alert.AlertIf = replaceTags(data.Expanded, alert, value)

	if alert.AlertAnnotations == nil {
		alert.AlertAnnotations = map[string]string{}
	}
	for k, v := range data.Annotations {
		if _, ok := alert.AlertAnnotations[k]; !ok {
			alert.AlertAnnotations[k] = replaceTags(v, alert, value)
		}
	}

	if alert.AlertLabels == nil {
		alert.AlertLabels = map[string]string{}
	}
	for k, v := range data.Labels {
		if _, ok := alert.AlertLabels[k]; !ok {
			alert.AlertLabels[k] = replaceTags(v, alert, value)
		}
	}
}

func formatCompoundAlert(alert *prometheus.Alert) {
	alertIfStr := alert.AlertIf
	alertAnnotations := map[string]string{}
	immutableAnnotations := map[string]struct{}{}

	// copy alert annotations and alert labels
	if alert.AlertAnnotations != nil {
		for k := range alert.AlertAnnotations {
			immutableAnnotations[k] = struct{}{}
		}
	}

	var alertIfFormattedBuffer bytes.Buffer

	currentAlert, bOp, alertIfStr := splitCompoundOp(alertIfStr)

	for len(currentAlert) > 0 {
		value := ""
		alertSplit := strings.Split(currentAlert, ":")
		shortcut := alertSplit[0]

		if len(alertSplit) > 1 {
			value = alertSplit[1]
		}
		data, ok := alertIfShortcutData[shortcut]
		if !ok {
			return
		}

		alertIfFormattedBuffer.WriteString(replaceTags(data.Expanded, alert, value))
		if len(bOp) > 0 {
			alertIfFormattedBuffer.WriteString(fmt.Sprintf(" %s ", bOp))
		}

		for k, v := range data.Annotations {
			if _, ok := immutableAnnotations[k]; ok {
				continue
			}
			alertAnnotations[k] += replaceTags(v, alert, value)
			if len(bOp) > 0 {
				alertAnnotations[k] += fmt.Sprintf(" %s ", bOp)
			}
		}
		currentAlert, bOp, alertIfStr = splitCompoundOp(alertIfStr)
	}

	alert.AlertIf = alertIfFormattedBuffer.String()

	if alert.AlertAnnotations == nil {
		alert.AlertAnnotations = map[string]string{}
	}

	for k, v := range alertAnnotations {
		if _, ok := immutableAnnotations[k]; ok {
			continue
		}
		alert.AlertAnnotations[k] = v
	}

}

// splitCompoundOp find splits string into three pieces if it includes _unless_,
// _and_, or _or_. For example, hello_and_world_or_earth will return [hello, and, world_or_earth]
func splitCompoundOp(s string) (string, string, string) {
	binaryOps := []string{"unless", "and", "or"}

	minIdx := len(s)
	minOp := ""
	for _, bOp := range binaryOps {
		idx := strings.Index(s, fmt.Sprintf("_%s_", bOp))
		if idx != -1 && idx < minIdx {
			minIdx = idx
			minOp = bOp
		}
	}

	if len(minOp) > 0 {
		return s[:minIdx], minOp, s[minIdx+len(minOp)+2:]
	}
	return s, "", ""

}

func replaceTags(tag string, alert *prometheus.Alert, value string) string {

	alertInput := alertTemplateInput{
		Alert:  alert,
		Values: strings.Split(value, ","),
	}
	t := template.Must(template.New("tag").Parse(tag))
	b := new(bytes.Buffer)
	t.Execute(b, alertInput)

	return b.String()
}

func (s *serve) isValidAlert(alert *prometheus.Alert) bool {
	return len(alert.AlertName) > 0 && len(alert.AlertIf) > 0
}

func (s *serve) deleteAlerts(
	serviceName string, keepPersistantAlerts bool) []prometheus.Alert {
	alerts := []prometheus.Alert{}
	serviceNameFormatted := s.getNameFormatted(serviceName)
	for k, v := range s.alerts {
		if strings.HasPrefix(k, serviceNameFormatted) {
			if !keepPersistantAlerts || !v.AlertPersistent {
				alerts = append(alerts, v)
				delete(s.alerts, k)
			}
		}
	}
	return alerts
}

func (s *serve) getNameFormatted(name string) string {
	return strings.Replace(name, "-", "", -1)
}

func (s *serve) getNodeLabel(req *http.Request) (string, map[string]string, error) {
	nodeLabel := map[string]string{}
	nodeID := req.Form.Get("id")
	if len(nodeID) == 0 {
		return "", nodeLabel, errors.New("node id not included in requests")
	}

	nodeTargetLabels := s.getTargetLabelsFromEnv()
	for _, targetLabel := range nodeTargetLabels {
		if v := req.Form.Get(targetLabel); len(v) > 0 {
			nodeLabel[targetLabel] = v
		}
	}
	s.nodeLabels[nodeID] = nodeLabel
	return nodeID, nodeLabel, nil
}

func (s *serve) getScrape(req *http.Request) prometheus.Scrape {
	scrape := prometheus.Scrape{}
	decoder.Decode(&scrape, req.Form)
	if !s.isValidScrape(&scrape) {
		return scrape
	}

	if nodeInfoStr := req.Form.Get("nodeInfo"); len(nodeInfoStr) > 0 {
		nodeInfo := prometheus.NodeIPSet{}
		json.Unmarshal([]byte(nodeInfoStr), &nodeInfo)
		scrape.NodeInfo = nodeInfo
	}

	if scrape.NodeInfo != nil && len(scrape.NodeInfo) > 0 {
		scrape.ScrapeLabels = &map[string]string{}
		if targetLabels := os.Getenv("DF_SCRAPE_TARGET_LABELS"); len(targetLabels) > 0 {
			labels := strings.Split(targetLabels, ",")
			for _, label := range labels {
				value := req.Form.Get(label)
				if len(value) > 0 {
					(*scrape.ScrapeLabels)[label] = value
				}
			}
		}
	}

	s.scrapes[scrape.ServiceName] = scrape
	logPrintf("Adding scrape %s\n%v", scrape.ServiceName, scrape)

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

func (s *serve) getNodeResponse(ID string, nodeLabel map[string]string, err error, statusCode int) nodeResponse {
	resp := nodeResponse{
		Status:    statusCode,
		NodeID:    ID,
		NodeLabel: nodeLabel,
	}
	if err != nil {
		resp.Message = err.Error()
		resp.Status = http.StatusInternalServerError
	}
	return resp
}

func (s *serve) getTargetLabelsFromEnv() []string {
	nodeTargetLabels := strings.Split(os.Getenv("DF_NODE_TARGET_LABELS"), ",")

	o := []string{}
	for _, targetLabel := range nodeTargetLabels {
		o = append(o, strings.Replace(targetLabel, "-", "_", -1))
	}
	return o
}

func (s *serve) getNoIDNodeResponse(err error, status int) nodeResponse {
	resp := nodeResponse{
		Status:  status,
		Message: err.Error(),
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
		msg := fmt.Errorf("SCRAPE_PORT_* and SERVICE_NAME_* environment variable configuration are not valid")
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
