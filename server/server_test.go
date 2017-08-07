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
	"net/url"
	"../prometheus"
)

type ServerTestSuite struct {
	suite.Suite
}

func (s *ServerTestSuite) SetupTest() {
}

func TestServerUnitTestSuite(t *testing.T) {
	s := new(ServerTestSuite)
	logPrintlnOrig := logPrintf
	defer func() {
		logPrintf = logPrintlnOrig
		prometheus.LogPrintf = logPrintlnOrig
	}()
	logPrintf = func(format string, v ...interface{}) {}
	prometheus.LogPrintf = func(format string, v ...interface{}) {}
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer testServer.Close()
	os.Setenv("GLOBAL_SCRAPE_INTERVAL", "5s")
	os.Setenv("ARG_CONFIG_FILE", "/etc/prometheus/prometheus.yml")
	os.Setenv("ARG_STORAGE_LOCAL_PATH", "/prometheus")
	os.Setenv("ARG_WEB_CONSOLE_LIBRARIES", "/usr/share/prometheus/console_libraries")
	os.Setenv("ARG_WEB_CONSOLE_TEMPLATES", "/usr/share/prometheus/consoles")
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
	fsOrig := prometheus.FS
	defer func() { prometheus.FS = fsOrig }()
	prometheus.FS = afero.NewMemMapFs()

	serve := New()
	serve.Execute()

	actual, _ := afero.ReadFile(prometheus.FS, "/etc/prometheus/prometheus.yml")
	s.Equal(expected, string(actual))
}

// EmptyHandler

func (s *ServerTestSuite) Test_EmptyHandler_SetsContentHeaderToJson() {
	actual := http.Header{}
	rwMock := ResponseWriterMock{
		HeaderMock: func() http.Header {
			return actual
		},
	}
	addr := "/v1/docker-flow-monitor"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.EmptyHandler(rwMock, req)

	s.Equal("application/json", actual.Get("Content-Type"))
}

func (s *ServerTestSuite) Test_EmptyHandler_SetsStatusCodeTo200() {
	actual := 0
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(status int) {
			actual = status
		},
	}
	addr := "/v1/docker-flow-monitor"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.EmptyHandler(rwMock, req)

	s.Equal(200, actual)
}

// ReconfigureHandler

func (s *ServerTestSuite) Test_ReconfigureHandler_SetsContentHeaderToJson() {
	actual := http.Header{}
	rwMock := ResponseWriterMock{
		HeaderMock: func() http.Header {
			return actual
		},
	}
	addr := "/v1/docker-flow-monitor?alertName=my-alert&alertIf=my-if"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal("application/json", actual.Get("Content-Type"))
}

func (s *ServerTestSuite) Test_ReconfigureHandler_SetsStatusCodeTo200() {
	actual := 0
	reloadOrig := prometheus.Reload
	defer func() { prometheus.Reload = reloadOrig }()
	prometheus.Reload = func() error {
		return nil
	}
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(status int) {
			actual = status
		},
	}
	addr := "/v1/docker-flow-monitor?alertName=my-alert&alertIf=my-if"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(200, actual)
}

func (s *ServerTestSuite) Test_ReconfigureHandler_AddsAlert() {
	expected := prometheus.Alert{
		ServiceName: "my-service",
		AlertName: "my-alert",
		AlertIf: "a>b",
		AlertFor: "my-for",
		AlertNameFormatted: "myservicemyalert",
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=%s&alertFor=%s",
		expected.ServiceName,
		expected.AlertName,
		url.QueryEscape(expected.AlertIf),
		expected.AlertFor,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, serve.Alerts[expected.AlertNameFormatted])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_AddsFormattedAlert() {
	testData := []struct{
		expected string
		shortcut string
		annotations map[string]string
		labels map[string]string
	}{
		{
			`container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="my-service"} > 0.8`,
			`@service_mem_limit:0.8`,
			map[string]string{"summary": "Memory of the service my-service is over 0.8"},
			map[string]string{"receiver": "system", "service": "my-service"},
		}, {
			`(sum by (instance) (node_memory_MemTotal) - sum by (instance) (node_memory_MemFree + node_memory_Buffers + node_memory_Cached)) / sum by (instance) (node_memory_MemTotal) > 0.8`,
			`@node_mem_limit:0.8`,
			map[string]string{"summary": "Memory of a node is over 0.8"},
			map[string]string{"receiver": "system", "service": "my-service"},
		}, {
			`(node_filesystem_size{fstype="aufs"} - node_filesystem_free{fstype="aufs"}) / node_filesystem_size{fstype="aufs"} > 0.8`,
			`@node_fs_limit:0.8`,
			map[string]string{"summary": "Disk usage of a node is over 0.8"},
			map[string]string{"receiver": "system", "service": "my-service"},
		},
	}
	for _, data := range testData {
		expected := prometheus.Alert{
			AlertAnnotations:   data.annotations,
			AlertFor:           "my-for",
			AlertIf:            data.expected,
			AlertLabels:        data.labels,
			AlertName:          "my-alert",
			AlertNameFormatted: "myservicemyalert",
			ServiceName:        "my-service",
		}
		rwMock := ResponseWriterMock{}
		addr := fmt.Sprintf(
			"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=%s&alertFor=%s",
			expected.ServiceName,
			expected.AlertName,
			data.shortcut,
			expected.AlertFor,
		)
		req, _ := http.NewRequest("GET", addr, nil)

		serve := New()
		serve.ReconfigureHandler(rwMock, req)

		s.Equal(expected, serve.Alerts[expected.AlertNameFormatted])
	}
}

func (s *ServerTestSuite) Test_ReconfigureHandler_RemovesOldAlerts() {
	expected := prometheus.Alert{
		ServiceName: "my-service",
		AlertName: "my-alert",
		AlertIf: "a>b",
		AlertFor: "my-for",
		AlertNameFormatted: "myservicemyalert",
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=%s&alertFor=%s",
		expected.ServiceName,
		expected.AlertName,
		url.QueryEscape(expected.AlertIf),
		expected.AlertFor,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.Alerts["myservicesomeotheralert"] = prometheus.Alert{
		ServiceName: "my-service",
		AlertName: "some-other-alert",
	}
	serve.Alerts["anotherservicemyalert"] = prometheus.Alert{
		ServiceName: "another-service",
		AlertName: "my-alert",
	}
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(2, len(serve.Alerts))
	s.Equal(expected, serve.Alerts[expected.AlertNameFormatted])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_AddsMultipleAlerts() {
	expected := []prometheus.Alert{}
	for i:=1; i <=2; i++ {
		expected = append(expected, prometheus.Alert{
			ServiceName: "my-service",
			AlertName: fmt.Sprintf("my-alert-%d", i),
			AlertIf: fmt.Sprintf("my-if-%d", i),
			AlertFor: fmt.Sprintf("my-for-%d", i),
			AlertNameFormatted: fmt.Sprintf("myservicemyalert%d", i),
		})
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName.1=%s&alertIf.1=%s&alertFor.1=%s&alertName.2=%s&alertIf.2=%s&alertFor.2=%s",
		expected[0].ServiceName,
		expected[0].AlertName,
		expected[0].AlertIf,
		expected[0].AlertFor,
		expected[1].AlertName,
		expected[1].AlertIf,
		expected[1].AlertFor,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(2, len(serve.Alerts))
	s.Contains(serve.Alerts, expected[0].AlertNameFormatted)
	s.Contains(expected, serve.Alerts[expected[0].AlertNameFormatted])
	s.Contains(serve.Alerts, expected[1].AlertNameFormatted)
	s.Contains(expected, serve.Alerts[expected[1].AlertNameFormatted])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_AddsScrape() {
	expected := prometheus.Scrape{
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
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, serve.Scrapes[expected.ServiceName])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_DoesNotAddAlert_WhenAlertNameIsEmpty() {
	rwMock := ResponseWriterMock{}
	req, _ := http.NewRequest("GET", "/v1/docker-flow-monitor", nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(0, len(serve.Alerts))
}

func (s *ServerTestSuite) Test_ReconfigureHandler_DoesNotAddScrape_WhenScrapePortZero() {
	rwMock := ResponseWriterMock{}
	req, _ := http.NewRequest("GET", "/v1/docker-flow-monitor?serviceName=my-service", nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(0, len(serve.Scrapes))
}

func (s *ServerTestSuite) Test_ReconfigureHandler_AddsAlertNameFormatted() {
	expected := prometheus.Alert{
		AlertName: "my-alert",
		AlertIf: "my-if",
		AlertFor: "my-for",
		AlertNameFormatted: "myalert",
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?alertName=%s&alertIf=%s&alertFor=%s",
		expected.AlertName,
		expected.AlertIf,
		expected.AlertFor,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, serve.Alerts["myalert"])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_ReturnsJson() {
	reloadOrig := prometheus.Reload
	defer func() { prometheus.Reload = reloadOrig }()
	prometheus.Reload = func() error {
		return nil
	}
	expected := Response{
		Status: http.StatusOK,
		Alerts: []prometheus.Alert{prometheus.Alert{
			ServiceName: "my-service",
			AlertName: "myalert",
			AlertIf: "my-if",
			AlertFor: "my-for",
			AlertNameFormatted: "myservicemyalert",
		}},
		Scrape: prometheus.Scrape{
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
		"/v1/docker-flow-monitor?serviceName=%s&scrapePort=%d&alertName=%s&alertIf=%s&alertFor=%s",
		expected.ServiceName,
		expected.ScrapePort,
		expected.Alerts[0].AlertName,
		expected.Alerts[0].AlertIf,
		expected.Alerts[0].AlertFor,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, actual)
}

func (s *ServerTestSuite) Test_ReconfigureHandler_CallsWriteConfig() {
	expected := `
global:
  scrape_interval: 5s

scrape_configs:
  - job_name: "my-service"
    dns_sd_configs:
      - names: ["tasks.my-service"]
        type: A
        port: 1234

rule_files:
  - 'alert.rules'
`
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service&scrapePort=1234&alertName=my-alert&alertIf=my-if&alertFor=my-for"
	req, _ := http.NewRequest("GET", addr, nil)
	fsOrig := prometheus.FS
	defer func() { prometheus.FS = fsOrig }()
	prometheus.FS = afero.NewMemMapFs()

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	actual, _ := afero.ReadFile(prometheus.FS, "/etc/prometheus/prometheus.yml")
	s.Equal(expected, string(actual))
}

func (s *ServerTestSuite) Test_ReconfigureHandler_SendsReloadRequestToPrometheus() {
	reloadOrig := prometheus.Reload
	defer func() { prometheus.Reload = reloadOrig }()
	called := false
	prometheus.Reload = func() error {
		called = true
		return nil
	}
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service&scrapePort=1234"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.True(called)
}

func (s *ServerTestSuite) Test_ReconfigureHandler_ReturnsNokWhenPrometheusReloadFails() {
	actualResponse := Response{}
	rwMock := ResponseWriterMock{
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actualResponse)
			return 0, nil
		},
	}
	addr := "/v1/docker-flow-monitor?serviceName=my-service&scrapePort=1234"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(http.StatusInternalServerError, actualResponse.Status)
}

func (s *ServerTestSuite) Test_ReconfigureHandler_ReturnsStatusCodeFromPrometheus() {
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

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(http.StatusInternalServerError, actualResponse.Status)
	s.Equal(http.StatusInternalServerError, actualStatus)
}

// PingHandler

func (s *ServerTestSuite) Test_PingHandler_SetsContentHeaderToJson() {
	actual := http.Header{}
	rwMock := ResponseWriterMock{
		HeaderMock: func() http.Header {
			return actual
		},
	}
	addr := "/v1/docker-flow-monitor/pingx"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.PingHandler(rwMock, req)

	s.Equal("application/json", actual.Get("Content-Type"))
}

func (s *ServerTestSuite) Test_PingHandler_SetsStatusCodeTo200() {
	actual := 0
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(status int) {
			actual = status
		},
	}
	addr := "/v1/docker-flow-monitor/pingx"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.PingHandler(rwMock, req)

	s.Equal(200, actual)
}

// RemoveHandler

func (s *ServerTestSuite) Test_RemoveHandler_SetsContentHeaderToJson() {
	actual := http.Header{}
	rwMock := ResponseWriterMock{
		HeaderMock: func() http.Header {
			return actual
		},
	}
	addr := "/v1/docker-flow-monitor?alertName=my-alert&alertIf=my-if"
	req, _ := http.NewRequest("DELETE", addr, nil)

	serve := New()
	serve.RemoveHandler(rwMock, req)

	s.Equal("application/json", actual.Get("Content-Type"))
}

func (s *ServerTestSuite) Test_RemoveHandler_RemovesScrape() {
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service-1"
	req, _ := http.NewRequest("DELETE", addr, nil)

	serve := New()
	serve.Scrapes["my-service-1"] = prometheus.Scrape{ServiceName: "my-service-1", ScrapePort: 1111}
	serve.Scrapes["my-service-2"] = prometheus.Scrape{ServiceName: "my-service-2", ScrapePort: 2222}
	serve.RemoveHandler(rwMock, req)

	s.Len(serve.Scrapes, 1)
}

func (s *ServerTestSuite) Test_RemoveHandler_RemovesAlerts() {
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service-1"
	req, _ := http.NewRequest("DELETE", addr, nil)

	serve := New()
	serve.Alerts["myservice1alert1"] = prometheus.Alert{ServiceName: "my-service-1", AlertName: "my-alert-1"}
	serve.Alerts["myservice1alert2"] = prometheus.Alert{ServiceName: "my-service-1", AlertName: "my-alert-1"}
	serve.Alerts["myservice2alert1"] = prometheus.Alert{ServiceName: "my-service-2", AlertName: "my-alert-1"}
	serve.RemoveHandler(rwMock, req)

	s.Len(serve.Alerts, 1)
}

func (s *ServerTestSuite) Test_RemoveHandler_ReturnsJson() {
	reloadOrig := prometheus.Reload
	defer func() { prometheus.Reload = reloadOrig }()
	prometheus.Reload = func() error {
		return nil
	}
	expected := Response{
		Status: http.StatusOK,
		Alerts: []prometheus.Alert{
			prometheus.Alert{ServiceName: "my-service", AlertName: "my-alert"},
		},
		Scrape: prometheus.Scrape{
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
	addr := "/v1/docker-flow-monitor?serviceName=my-service"
	req, _ := http.NewRequest("DELETE", addr, nil)

	serve := New()
	serve.Scrapes[expected.Scrape.ServiceName] = expected.Scrape
	alertKey := serve.getNameFormatted(fmt.Sprintf("%s%s", expected.Alerts[0].ServiceName, expected.Alerts[0].AlertName))
	serve.Alerts[alertKey] = expected.Alerts[0]
	serve.RemoveHandler(rwMock, req)

	s.Equal(expected, actual)
}

func (s *ServerTestSuite) Test_RemoveHandler_CallsWriteConfig() {
	expectedAfterGet := `
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
	addr := "/v1/docker-flow-monitor?serviceName=my-service&scrapePort=1234"
	req, _ := http.NewRequest("GET", addr, nil)
	fsOrig := prometheus.FS
	defer func() { prometheus.FS = fsOrig }()
	prometheus.FS = afero.NewMemMapFs()

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	actual, _ := afero.ReadFile(prometheus.FS, "/etc/prometheus/prometheus.yml")
	s.Equal(expectedAfterGet, string(actual))

	expectedAfterDelete := `
global:
  scrape_interval: 5s
`
	addr = "/v1/docker-flow-monitor?serviceName=my-service"
	req, _ = http.NewRequest("DELETE", addr, nil)

	serve.RemoveHandler(rwMock, req)

	actual, _ = afero.ReadFile(prometheus.FS, "/etc/prometheus/prometheus.yml")
	s.Equal(expectedAfterDelete, string(actual))
}

func (s *ServerTestSuite) Test_RemoveHandler_SendsReloadRequestToPrometheus() {
	called := false
	reloadOrig := prometheus.Reload
	defer func() { prometheus.Reload = reloadOrig }()
	prometheus.Reload = func() error {
		called = true
		return nil
	}
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service"
	req, _ := http.NewRequest("DELETE", addr, nil)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer testServer.Close()

	serve := New()
	serve.RemoveHandler(rwMock, req)

	s.True(called)
}

func (s *ServerTestSuite) Test_RemoveHandler_ReturnsNokWhenPrometheusReloadFails() {
	actualResponse := Response{}
	rwMock := ResponseWriterMock{
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actualResponse)
			return 0, nil
		},
	}
	addr := "/v1/docker-flow-monitor?serviceName=my-service"
	req, _ := http.NewRequest("DELETE", addr, nil)

	serve := New()
	serve.RemoveHandler(rwMock, req)

	s.Equal(http.StatusInternalServerError, actualResponse.Status)
}

func (s *ServerTestSuite) Test_RemoveHandler_ReturnsStatusCodeFromPrometheus() {
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
	addr := "/v1/docker-flow-monitor?serviceName=my-service"
	req, _ := http.NewRequest("DELETE", addr, nil)

	serve := New()
	serve.RemoveHandler(rwMock, req)

	s.Equal(http.StatusInternalServerError, actualResponse.Status)
	s.Equal(http.StatusInternalServerError, actualStatus)
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
	os.Setenv("LISTENER_ADDRESS", "123.456.789.0")

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
	expected := map[string]prometheus.Scrape{
		"service-1": prometheus.Scrape{ServiceName: "service-1", ScrapePort: 1111},
		"service-2": prometheus.Scrape{ServiceName: "service-2", ScrapePort: 2222},
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

func (s *ServerTestSuite) Test_InitialConfig_AddsScrapesFromEnv() {
	 expected := map[string]prometheus.Scrape{
		 "service-1": prometheus.Scrape{ServiceName: "service-1", ScrapePort: 1111},
		 "service-2": prometheus.Scrape{ServiceName: "service-2", ScrapePort: 2222},
		 "service-4": prometheus.Scrape{ServiceName: "service-4", ScrapePort: 4444, ScrapeType: "static_configs"},
		 "service-3": prometheus.Scrape{ServiceName: "service-3", ScrapePort: 3333, ScrapeType: "static_configs"},
	 }
	 testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		 w.WriteHeader(http.StatusOK)
		 resp := []map[string]string{}
		 resp = append(resp, map[string]string{"scrapePort": "1111", "serviceName": "service-1"})
		 resp = append(resp, map[string]string{"scrapePort": "2222", "serviceName": "service-2"})
		 resp = append(resp, map[string]string{"scrapePort": "4444", "serviceName": "service-4", "scrapeType": "static_configs"})
		 js, _ := json.Marshal(resp)
		 w.Write(js)
	 }))
	 defer testServer.Close()
	 defer func() { os.Unsetenv("LISTENER_ADDRESS") }()
	 os.Setenv("LISTENER_ADDRESS", testServer.URL)
	 os.Setenv("SCRAPE_PORT_1", "3333")
	 os.Setenv("SERVICE_NAME_1", "service-3")

	 serve := New()
	 serve.InitialConfig()

	 s.Equal(expected, serve.Scrapes)
 }
func (s *ServerTestSuite) Test_InitialConfig_AddsAlerts() {
	expected := map[string]prometheus.Alert{
		"myservicealert1": prometheus.Alert{
			AlertAnnotations: map[string]string{},
			AlertName:        "alert-1",
			AlertIf:          "if-1",
			AlertFor:         "for-1",
			AlertLabels:      map[string]string{
				"label-1-1": "value-1-1",
				"label-1-2": "value-1-2",
			},
			ServiceName:        "my-service",
			AlertNameFormatted: "myservicealert1",
		},
		"myservicealert21": prometheus.Alert{
			AlertAnnotations:   map[string]string{},
			AlertFor:           "for-21",
			AlertIf:            "if-21",
			AlertLabels:        map[string]string{},
			AlertName:          "alert-21",
			ServiceName:        "my-service",
			AlertNameFormatted: "myservicealert21",
		},
		"myservicealert22": prometheus.Alert{
			AlertAnnotations: map[string]string{
				"annotation-22-1": "value-22-1",
				"annotation-22-2": "value-22-2",
			},
			AlertFor:           "for-22",
			AlertIf:            "if-22",
			AlertLabels:        map[string]string{},
			AlertName:          "alert-22",
			ServiceName:        "my-service",
			AlertNameFormatted: "myservicealert22",
		},
	}
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := []map[string]string{}
		// Is Not included since alertIf is missing
		resp = append(resp, map[string]string{
			"serviceName": "my-service-without-if",
			"alertName": "alert-without-if",
		})
		resp = append(resp, map[string]string{
			"serviceName": "my-service",
			"alertName": "alert-1",
			"alertIf": "if-1",
			"alertFor": "for-1",
			"alertLabels": "label-1-1=value-1-1,label-1-2=value-1-2",
		})
		resp = append(resp, map[string]string{
			"serviceName": "my-service",
			"alertName.1": "alert-21",
			"alertIf.1": "if-21",
			"alertFor.1": "for-21",
			"alertName.2": "alert-22",
			"alertAnnotations.2": "annotation-22-1=value-22-1,annotation-22-2=value-22-2",
			"alertIf.2": "if-22",
			"alertFor.2": "for-22",
		})
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
