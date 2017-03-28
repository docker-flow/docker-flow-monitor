package server

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"net/http"
	"time"
	"fmt"
	"encoding/json"
	"github.com/spf13/afero"
	"os"
	"net/http/httptest"
	"os/exec"
)

type ServerTestSuite struct {
	suite.Suite
}

func (s *ServerTestSuite) SetupTest() {
}

func TestServerUnitTestSuite(t *testing.T) {
	s := new(ServerTestSuite)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer testServer.Close()
	prometheusAddrOrig := prometheusAddr
	defer func() { prometheusAddr = prometheusAddrOrig }()
	prometheusAddr = testServer.URL
	suite.Run(t, s)
}

// NewServe

func (s *ServerTestSuite) Test_New_ReturnsServe() {
	serve := New()

	s.NotNil(serve)
}

func (s *ServerTestSuite) Test_New_InitializesAlerts() {
	serve := New()

	s.NotNil(serve.Alerts)
	s.Len(serve.Alerts, 0)
}

func (s *ServerTestSuite) Test_New_InitializesScrapes() {
	serve := New()

	s.NotNil(serve.Scrapes)
	s.Len(serve.Scrapes, 0)
}

// Execute

func (s *ServerTestSuite) Test_Execute_InvokesHTTPListenAndServe() {
	serve := New()
	var actual string
	expected := "0.0.0.0:8080"
	httpListenAndServe = func(addr string, handler http.Handler) error {
		actual = addr
		return nil
	}

	serve.Execute()
	time.Sleep(1 * time.Millisecond)

	s.Equal(expected, actual)
}

func (s *ServerTestSuite) Test_Execute_ReturnsError_WhenHTTPListenAndServeFails() {
	orig := httpListenAndServe
	defer func() { httpListenAndServe = orig }()
	httpListenAndServe = func(addr string, handler http.Handler) error {
		return fmt.Errorf("This is an error")
	}

	serve := New()
	actual := serve.Execute()

	s.Error(actual)
}

func (s *ServerTestSuite) Test_Execute_WritesConfig() {
	expected := `
global:
  scrape_interval: 5s
`
	fsOrig := fs
	defer func() { fs = fsOrig }()
	fs = afero.NewMemMapFs()

	serve := New()
	serve.Execute()

	actual, _ := afero.ReadFile(fs, "/etc/prometheus/prometheus.yml")
	s.Equal(expected, string(actual))
}

// Handler

func (s *ServerTestSuite) Test_Handler_SetsContentHeaderToJson() {
	actual := http.Header{}
	rwMock := ResponseWriterMock{
		HeaderMock: func() http.Header {
			return actual
		},
	}
	addr := "/v1/docker-flow-monitor?alertName=my-alert&alertIf=my-if"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.Handler(rwMock, req)

	s.Equal("application/json", actual.Get("Content-Type"))
}

func (s *ServerTestSuite) Test_Handler_AddsAlert() {
	expected := Alert{
			AlertName: "myAlert",
			AlertIf: "my-if",
			AlertFrom: "my-from",
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?alertName=%s&alertIf=%s&alertFrom=%s",
		expected.AlertName,
		expected.AlertIf,
		expected.AlertFrom,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.Handler(rwMock, req)

	s.Equal(expected, serve.Alerts[expected.AlertName])
}

func (s *ServerTestSuite) Test_Handler_AddsScrape() {
	expected := Scrape{
		ServiceName: "my-service",
		ScrapePort: 1234,
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&scrapePort=%d",
		expected.ServiceName,
		expected.ScrapePort,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.Handler(rwMock, req)

	s.Equal(expected, serve.Scrapes[expected.ServiceName])
}

func (s *ServerTestSuite) Test_Handler_DoesNotAddAlert_WhenAlertNameIsEmpty() {
	rwMock := ResponseWriterMock{}
	req, _ := http.NewRequest("GET", "/v1/docker-flow-monitor", nil)

	serve := New()
	serve.Handler(rwMock, req)

	s.Equal(0, len(serve.Alerts))
}

func (s *ServerTestSuite) Test_Handler_RemovesSpecialCharactersFromTheAlertName() {
	expected := Alert{
		AlertName: "myalert",
		AlertIf: "my-if",
		AlertFrom: "my-from",
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?alertName=my-alert&alertIf=%s&alertFrom=%s",
		expected.AlertIf,
		expected.AlertFrom,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.Handler(rwMock, req)

	s.Equal(expected, serve.Alerts["myalert"])
}

func (s *ServerTestSuite) Test_Handler_ReturnsJson() {
	expected := Response{
		Status: http.StatusOK,
		Alert: Alert{
			AlertName: "myalert",
			AlertIf: "my-if",
			AlertFrom: "my-from",
		},
		Scrape: Scrape{
			ServiceName: "my-service",
			ScrapePort: 1234,
		},
	}
	actual := Response{}
	rwMock := ResponseWriterMock{
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actual)
			return 0, nil
		},
	}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&scrapePort=%d&alertName=%s&alertIf=%s&alertFrom=%s",
		expected.ServiceName,
		expected.ScrapePort,
		expected.AlertName,
		expected.AlertIf,
		expected.AlertFrom,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.Handler(rwMock, req)

	s.Equal(expected, actual)
}

func (s *ServerTestSuite) Test_Handler_CallsWriteConfig() {
	expected := `
global:
  scrape_interval: 5s

scrape_configs:
  - job_name: "my-service"
    dns_sd_configs:
      - names: ["tasks.my-service"]
        type: A
        port: 1234
`
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service&scrapePort=1234&alertName=my-alert&alertIf=my-if&alertFrom=my-from"
	req, _ := http.NewRequest("GET", addr, nil)
	fsOrig := fs
	defer func() { fs = fsOrig }()
	fs = afero.NewMemMapFs()

	serve := New()
	serve.Handler(rwMock, req)

	actual, _ := afero.ReadFile(fs, "/etc/prometheus/prometheus.yml")
	s.Equal(expected, string(actual))
}

func (s *ServerTestSuite) Test_Handler_SendsReloadRequestToPrometheus() {
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service&scrapePort=1234"
	req, _ := http.NewRequest("GET", addr, nil)
	actualMethod := ""
	actualPath := ""
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualMethod = r.Method
		actualPath = r.URL.Path
	}))
	defer testServer.Close()
	prometheusAddrOrig := prometheusAddr
	defer func() { prometheusAddr = prometheusAddrOrig }()
	prometheusAddr = testServer.URL

	serve := New()
	serve.Handler(rwMock, req)

	s.Equal("POST", actualMethod)
	s.Equal("/-/reload", actualPath)
}

func (s *ServerTestSuite) Test_Handler_ReturnsNokWhenPrometheusReloadFails() {
	actualResponse := Response{}
	rwMock := ResponseWriterMock{
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actualResponse)
			return 0, nil
		},
	}
	addr := "/v1/docker-flow-monitor?serviceName=my-service&scrapePort=1234"
	req, _ := http.NewRequest("GET", addr, nil)
	prometheusAddrOrig := prometheusAddr
	defer func() { prometheusAddr = prometheusAddrOrig }()
	prometheusAddr = "this-url-does-not-exist"

	serve := New()
	serve.Handler(rwMock, req)

	s.Equal(http.StatusInternalServerError, actualResponse.Status)
}

func (s *ServerTestSuite) Test_Handler_ReturnsStatusCodeFromPrometheus() {
	actualResponse := Response{}
	actualStatus := 0
	rwMock := ResponseWriterMock{
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actualResponse)
			return 0, nil
		},
		WriteHeaderMock: func(header int) {
			actualStatus = header
		},
	}
	addr := "/v1/docker-flow-monitor?serviceName=my-service&scrapePort=1234"
	req, _ := http.NewRequest("GET", addr, nil)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer testServer.Close()
	prometheusAddrOrig := prometheusAddr
	defer func() { prometheusAddr = prometheusAddrOrig }()
	prometheusAddr = testServer.URL

	serve := New()
	serve.Handler(rwMock, req)

	s.Equal(http.StatusBadGateway, actualResponse.Status)
	s.Equal(http.StatusBadGateway, actualStatus)
}

// WriteConfig

func (s *ServerTestSuite) Test_WriteConfig_WritesConfig() {
	fsOrig := fs
	defer func() { fs = fsOrig }()
	fs = afero.NewMemMapFs()
	serve := New()
	serve.Scrapes = map[string]Scrape {
		"service-1": Scrape{ ServiceName: "service-1", ScrapePort: 1234 },
		"service-2": Scrape{ ServiceName: "service-2", ScrapePort: 5678 },
	}
	gc, _ := serve.GetGlobalConfig()
	sc := serve.GetScrapeConfig()
	expected := fmt.Sprintf(`%s
%s`,
		gc,
		sc,
	)

	serve.WriteConfig()

	actual, _ := afero.ReadFile(fs, "/etc/prometheus/prometheus.yml")
	s.Equal(expected, string(actual))
}

// GetGlobalConfig

func (s *ServerTestSuite) Test_GlobalConfig_ReturnsConfigWithData() {
	scrapeIntervalOrig := os.Getenv("SCRAPE_INTERVAL")
	defer func() { os.Setenv("SCRAPE_INTERVAL", scrapeIntervalOrig) }()
	os.Setenv("SCRAPE_INTERVAL", "123")
	serve := New()
	expected := `
global:
  scrape_interval: 123s`

	actual, _ := serve.GetGlobalConfig()
	s.Equal(expected, actual)
}

func (s *ServerTestSuite) Test_GlobalConfig_ReturnsError_WhenScrapeIntervalIsNotNumber() {
	scrapeIntervalOrig := os.Getenv("SCRAPE_INTERVAL")
	defer func() { os.Setenv("SCRAPE_INTERVAL", scrapeIntervalOrig) }()
	os.Setenv("SCRAPE_INTERVAL", "xxx")
	serve := New()

	_, err := serve.GetGlobalConfig()
	s.Error(err)
}

// GetScrapeConfig

func (s *ServerTestSuite) Test_GetScrapeConfig_ReturnsConfigWithData() {
	serve := New()
	expected := `
scrape_configs:
  - job_name: "service-1"
    dns_sd_configs:
      - names: ["tasks.service-1"]
        type: A
        port: 1234
  - job_name: "service-2"
    dns_sd_configs:
      - names: ["tasks.service-2"]
        type: A
        port: 5678
`
	serve.Scrapes = map[string]Scrape {
		"service-1": Scrape{ ServiceName: "service-1", ScrapePort: 1234 },
		"service-2": Scrape{ ServiceName: "service-2", ScrapePort: 5678 },
	}

	actual := serve.GetScrapeConfig()

	s.Equal(expected, actual)
}

func (s *ServerTestSuite) Test_GetScrapeConfig_ReturnsEmptyString_WhenNoData() {
	serve := New()

	actual := serve.GetScrapeConfig()

	s.Empty(actual)
}

// GetAlertConfig

func (s *ServerTestSuite) Test_GetAlertConfig_ReturnsConfigWithData() {
	serve := New()
	expected := `
ALERT alert-name-1
  IF alert-if-1
  FROM alert-from-1

ALERT alert-name-2
  IF alert-if-2
`
	serve.Alerts = map[string]Alert {
		"alert-name-1": Alert{ AlertName: "alert-name-1", AlertIf: "alert-if-1", AlertFrom: "alert-from-1" },
		"alert-name-2": Alert{ AlertName: "alert-name-2", AlertIf: "alert-if-2" },
	}

	actual := serve.GetAlertConfig()

	s.Equal(expected, actual)
}

// RunPrometheus

func (s *ServerTestSuite) Test_RunPrometheus_ExecutesPrometheus() {
	cmdRunOrig := cmdRun
	defer func() { cmdRun = cmdRunOrig }()
	actualArgs := []string{}
	cmdRun = func(cmd *exec.Cmd) error {
		actualArgs = cmd.Args
		return nil
	}

	serve := New()
	serve.RunPrometheus()

	s.Equal([]string{"/bin/sh", "-c", "prometheus -config.file=/etc/prometheus/prometheus.yml -storage.local.path=/prometheus -web.console.libraries=/usr/share/prometheus/console_libraries -web.console.templates=/usr/share/prometheus/consoles"}, actualArgs)
}

func (s *ServerTestSuite) Test_RunPrometheus_ReturnsError() {
	serve := New()
	// Assumes that `prometheus` does not exist
	err := serve.RunPrometheus()

	s.Error(err)
}

// InitialConfig

func (s *ServerTestSuite) Test_InitialConfig_RequestsDataFromSwarmListener() {
	actualMethod := ""
	actualPath := ""
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualMethod = r.Method
		actualPath = r.URL.Path
	}))
	defer testServer.Close()
	defer func() { os.Unsetenv("LISTENER_ADDRESS") }()
	os.Setenv("LISTENER_ADDRESS", testServer.URL)

	serve := New()
	serve.InitialConfig()

	s.Equal("GET", actualMethod)
	s.Equal("/v1/docker-flow-swarm-listener/get-services", actualPath)
}

func (s *ServerTestSuite) Test_InitialConfig_ReturnsError_WhenAddressIsInvalid() {
	defer func() { os.Unsetenv("LISTENER_ADDRESS") }()
	os.Setenv("LISTENER_ADDRESS", "127.0.0.1")

	serve := New()
	err := serve.InitialConfig()

	s.Error(err)
}

func (s *ServerTestSuite) Test_InitialConfig_DoesNotSendRequest_WhenListenerAddressIsEmpty() {
	serve := New()
	err := serve.InitialConfig()

	s.NoError(err)
}

func (s *ServerTestSuite) Test_InitialConfig_AddsScrapes() {
	expected := map[string]Scrape{
		"service-1": Scrape{ServiceName: "service-1", ScrapePort: 1111},
		"service-2": Scrape{ServiceName: "service-2", ScrapePort: 2222},
	}
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := []map[string]string{}
		resp = append(resp, map[string]string{"scrapePort": "1111", "serviceName": "service-1"})
		resp = append(resp, map[string]string{"scrapePort": "2222", "serviceName": "service-2"})
		js, _ := json.Marshal(resp)
		w.Write(js)
	}))
	defer testServer.Close()
	defer func() { os.Unsetenv("LISTENER_ADDRESS") }()
	os.Setenv("LISTENER_ADDRESS", testServer.URL)

	serve := New()
	serve.InitialConfig()

	s.Equal(expected, serve.Scrapes)
}

func (s *ServerTestSuite) Test_InitialConfig_AddsAlerts() {
	expected := map[string]Alert{
		"alert-1": Alert{AlertName: "alert-1", AlertIf: "if-1", AlertFrom: "from-1"},
		"alert-2": Alert{AlertName: "alert-2", AlertIf: "if-2", AlertFrom: "from-2"},
	}
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := []map[string]string{}
		resp = append(resp, map[string]string{"alertName": "alert-1", "alertIf": "if-1", "alertFrom": "from-1"})
		resp = append(resp, map[string]string{"alertName": "alert-2", "alertIf": "if-2", "alertFrom": "from-2"})
		js, _ := json.Marshal(resp)
		w.Write(js)
	}))
	defer testServer.Close()
	defer func() { os.Unsetenv("LISTENER_ADDRESS") }()
	os.Setenv("LISTENER_ADDRESS", testServer.URL)

	serve := New()
	serve.InitialConfig()

	s.Equal(expected, serve.Alerts)
}

// Mock

type ResponseWriterMock struct {
	HeaderMock      func() http.Header
	WriteMock       func([]byte) (int, error)
	WriteHeaderMock func(int)
}

func (m ResponseWriterMock) Header() http.Header {
	if m.HeaderMock != nil {
		return m.HeaderMock()
	}
	return http.Header{}
}

func (m ResponseWriterMock) Write(content []byte) (int, error) {
	if m.WriteMock != nil {
		return m.WriteMock(content)
	}
	return 0, nil
}

func (m ResponseWriterMock) WriteHeader(header int) {
	if m.WriteHeaderMock != nil {
		m.WriteHeaderMock(header)
	}
}
