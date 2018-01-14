package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"../prometheus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"
)

type ServerTestSuite struct {
	suite.Suite
}

func (s *ServerTestSuite) SetupTest() {
}

func TestServerUnitTestSuite(t *testing.T) {
	s := new(ServerTestSuite)
	logPrintlnOrig := logPrintf
	listenerTimeoutOrig := listenerTimeout
	defer func() {
		logPrintf = logPrintlnOrig
		listenerTimeout = listenerTimeoutOrig
	}()
	listenerTimeout = 10 * time.Millisecond
	logPrintf = func(format string, v ...interface{}) {}
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer testServer.Close()
	os.Setenv("GLOBAL_SCRAPE_INTERVAL", "5s")
	os.Setenv("ARG_CONFIG_FILE", "/etc/prometheus/prometheus.yml")
	os.Setenv("ARG_STORAGE_LOCAL_PATH", "/prometheus")
	os.Setenv("ARG_WEB_CONSOLE_LIBRARIES", "/usr/share/prometheus/console_libraries")
	os.Setenv("ARG_WEB_CONSOLE_TEMPLATES", "/usr/share/prometheus/consoles")
	os.Setenv("ARG_ALERTMANAGER_URL", "http://alert-manager:9093")

	shortcutsPathOrig := shortcutsPath
	defer func() { shortcutsPath = shortcutsPathOrig }()
	shortcutsPath = "../conf/shortcuts.yaml"

	suite.Run(t, s)
}

// NewServe

func (s *ServerTestSuite) Test_New_ReturnsServe() {
	serve := New()

	s.NotNil(serve)
}

func (s *ServerTestSuite) Test_New_InitializesAlerts() {
	serve := New()

	s.NotNil(serve.alerts)
	s.Len(serve.alerts, 0)
}

func (s *ServerTestSuite) Test_New_InitializesScrapes() {
	serve := New()

	s.NotNil(serve.scrapes)
	s.Len(serve.scrapes, 0)
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
	expected := `global:
  scrape_interval: 5s
alerting:
  alertmanagers:
  - static_configs:
    - targets:
      - alert-manager:9093
    scheme: http
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
		ServiceName:        "my-service",
		AlertName:          "my-alert",
		AlertIf:            "a>b",
		AlertFor:           "my-for",
		AlertNameFormatted: "myservice_myalert",
		AlertAnnotations:   map[string]string{"a1": "v1", "a2": "v2"},
		AlertLabels:        map[string]string{"l1": "v1"},
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=%s&alertFor=%s&alertAnnotations=%s&alertLabels=%s",
		expected.ServiceName,
		expected.AlertName,
		url.QueryEscape(expected.AlertIf),
		expected.AlertFor,
		url.QueryEscape("a1=v1,a2=v2"),
		url.QueryEscape("l1=v1"),
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, serve.alerts[expected.AlertNameFormatted])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_ExpandsShortcuts() {
	testData := []struct {
		expected    string
		shortcut    string
		annotations map[string]string
		labels      map[string]string
	}{
		{
			`container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="my-service"} > 0.8`,
			`@service_mem_limit:0.8`,
			map[string]string{"summary": "Memory of the service my-service is over 0.8"},
			map[string]string{"receiver": "system", "service": "my-service"},
		}, {
			`(sum by (instance) (node_memory_MemTotal{job="my-service"}) - sum by (instance) (node_memory_MemFree{job="my-service"} + node_memory_Buffers{job="my-service"} + node_memory_Cached{job="my-service"})) / sum by (instance) (node_memory_MemTotal{job="my-service"}) > 0.8`,
			`@node_mem_limit:0.8`,
			map[string]string{"summary": "Memory of a node is over 0.8"},
			map[string]string{"receiver": "system", "service": "my-service"},
		}, {
			`(sum(node_memory_MemTotal{job="my-service"}) - sum(node_memory_MemFree{job="my-service"} + node_memory_Buffers{job="my-service"} + node_memory_Cached{job="my-service"})) / sum(node_memory_MemTotal{job="my-service"}) > 0.8`,
			`@node_mem_limit_total_above:0.8`,
			map[string]string{"summary": "Total memory of the nodes is over 0.8"},
			map[string]string{"receiver": "system", "service": "my-service", "scale": "up", "type": "node"},
		}, {
			`(sum(node_memory_MemTotal{job="my-service"}) - sum(node_memory_MemFree{job="my-service"} + node_memory_Buffers{job="my-service"} + node_memory_Cached{job="my-service"})) / sum(node_memory_MemTotal{job="my-service"}) < 0.4`,
			`@node_mem_limit_total_below:0.4`,
			map[string]string{"summary": "Total memory of the nodes is below 0.4"},
			map[string]string{"receiver": "system", "service": "my-service", "scale": "down", "type": "node"},
		}, {
			`(node_filesystem_size{fstype="aufs", job="my-service"} - node_filesystem_free{fstype="aufs", job="my-service"}) / node_filesystem_size{fstype="aufs", job="my-service"} > 0.8`,
			`@node_fs_limit:0.8`,
			map[string]string{"summary": "Disk usage of a node is over 0.8"},
			map[string]string{"receiver": "system", "service": "my-service"},
		}, {
			`sum(rate(http_server_resp_time_bucket{job="my-service", le="0.1"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) < 0.9999`,
			`@resp_time_above:0.1,5m,0.9999`,
			map[string]string{"summary": "Response time of the service my-service is above 0.1"},
			map[string]string{"receiver": "system", "service": "my-service", "scale": "up", "type": "service"},
		}, {
			`sum(rate(http_server_resp_time_bucket{job="my-service", le="0.025"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) > 0.75`,
			`@resp_time_below:0.025,5m,0.75`,
			map[string]string{"summary": "Response time of the service my-service is below 0.025"},
			map[string]string{"receiver": "system", "service": "my-service", "scale": "down", "type": "service"},
		}, {
			`count(container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}) != 3`,
			`@replicas_running`,
			map[string]string{"summary": "The number of running replicas of the service my-service is not 3"},
			map[string]string{"receiver": "system", "service": "my-service", "scale": "up", "type": "node"},
		}, {
			`count(container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}) > 3`,
			`@replicas_more_than`,
			map[string]string{"summary": "The number of running replicas of the service my-service is more than 3"},
			map[string]string{"receiver": "system", "service": "my-service", "scale": "up", "type": "node"},
		}, {
			`count(container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}) < 3`,
			`@replicas_less_than`,
			map[string]string{"summary": "The number of running replicas of the service my-service is less than 3"},
			map[string]string{"receiver": "system", "service": "my-service", "scale": "up", "type": "node"},
		}, {
			`sum(rate(http_server_resp_time_count{job="my-service", code=~"^5..$$"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) > 0.001`,
			`@resp_time_server_error:5m,0.001`,
			map[string]string{"summary": "Error rate of the service my-service is above 0.001"},
			map[string]string{"receiver": "system", "service": "my-service", "type": "errors"},
		},
	}
	for _, data := range testData {
		expected := prometheus.Alert{
			AlertAnnotations:   data.annotations,
			AlertFor:           "my-for",
			AlertIf:            data.expected,
			AlertLabels:        data.labels,
			AlertName:          "my-alert",
			AlertNameFormatted: "myservice_myalert",
			ServiceName:        "my-service",
			Replicas:           3,
		}
		rwMock := ResponseWriterMock{}
		addr := fmt.Sprintf(
			"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=%s&alertFor=%s&replicas=3",
			expected.ServiceName,
			expected.AlertName,
			data.shortcut,
			expected.AlertFor,
		)
		req, _ := http.NewRequest("GET", addr, nil)

		serve := New()
		serve.ReconfigureHandler(rwMock, req)

		s.Equal(expected, serve.alerts[expected.AlertNameFormatted])
	}
}

func (s *ServerTestSuite) Test_ReconfigureHandler_DoesNotExpandAnnotationsAndLabels_WhenTheyAreAlreadySet() {
	testData := struct {
		expected    string
		shortcut    string
		annotations map[string]string
		labels      map[string]string
	}{
		`container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="my-service"} > 0.8`,
		`@service_mem_limit:0.8`,
		map[string]string{"summary": "Memory of the service my-service is over 0.8"},
		map[string]string{"receiver": "system", "service": "my-service"},
	}
	expected := prometheus.Alert{
		AlertAnnotations:   map[string]string{"summary": "not-again"},
		AlertFor:           "my-for",
		AlertIf:            testData.expected,
		AlertLabels:        map[string]string{"receiver": "system", "service": "ugly-service"},
		AlertName:          "my-alert",
		AlertNameFormatted: "myservice_myalert",
		ServiceName:        "my-service",
		Replicas:           3,
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=%s&alertFor=%s&replicas=3&alertAnnotations=summary=not-again&alertLabels=service=ugly-service",
		expected.ServiceName,
		expected.AlertName,
		testData.shortcut,
		expected.AlertFor,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, serve.alerts[expected.AlertNameFormatted])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_RemovesOldAlerts() {
	expected := prometheus.Alert{
		ServiceName:        "my-service",
		AlertName:          "my-alert",
		AlertIf:            "a>b",
		AlertFor:           "my-for",
		AlertNameFormatted: "myservice_myalert",
		AlertAnnotations:   map[string]string{},
		AlertLabels:        map[string]string{},
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
	serve.alerts["myservicesomeotheralert"] = prometheus.Alert{
		ServiceName: "my-service",
		AlertName:   "some-other-alert",
	}
	serve.alerts["anotherservicemyalert"] = prometheus.Alert{
		ServiceName: "another-service",
		AlertName:   "my-alert",
	}
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(2, len(serve.alerts))
	s.Equal(expected, serve.alerts[expected.AlertNameFormatted])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_AddsMultipleAlerts() {
	expected := []prometheus.Alert{}
	for i := 1; i <= 2; i++ {
		expected = append(expected, prometheus.Alert{
			ServiceName:        "my-service",
			AlertName:          fmt.Sprintf("my-alert-%d", i),
			AlertIf:            fmt.Sprintf("my-if-%d", i),
			AlertFor:           fmt.Sprintf("my-for-%d", i),
			AlertNameFormatted: fmt.Sprintf("myservice_myalert%d", i),
			AlertAnnotations:   map[string]string{"annotation": fmt.Sprintf("annotation-value-%d", i)},
			AlertLabels:        map[string]string{"label": fmt.Sprintf("label-value-%d", i)},
			Replicas:           3,
		})
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName.1=%s&alertIf.1=%s&alertFor.1=%s&alertName.2=%s&alertIf.2=%s&alertFor.2=%s&alertAnnotations.1=%s&alertAnnotations.2=%s&alertLabels.1=%s&alertLabels.2=%s&replicas=3",
		expected[0].ServiceName,
		expected[0].AlertName,
		expected[0].AlertIf,
		expected[0].AlertFor,
		expected[1].AlertName,
		expected[1].AlertIf,
		expected[1].AlertFor,
		url.QueryEscape("annotation=annotation-value-1"),
		url.QueryEscape("annotation=annotation-value-2"),
		url.QueryEscape("label=label-value-1"),
		url.QueryEscape("label=label-value-2"),
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(2, len(serve.alerts))
	s.Contains(serve.alerts, expected[0].AlertNameFormatted)
	s.Equal(expected[0].AlertLabels, serve.alerts[expected[0].AlertNameFormatted].AlertLabels)
	s.Contains(expected, serve.alerts[expected[0].AlertNameFormatted])
	s.Contains(serve.alerts, expected[1].AlertNameFormatted)
	s.Contains(expected, serve.alerts[expected[1].AlertNameFormatted])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_AddsScrape() {
	expected := prometheus.Scrape{
		ServiceName: "my-service",
		ScrapePort:  1234,
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

	s.Equal(expected, serve.scrapes[expected.ServiceName])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_AddsScrapeType() {
	expected := prometheus.Scrape{
		ServiceName: "my-service",
		ScrapePort:  1234,
		ScrapeType:  "static_configs",
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&scrapePort=%d&scrapeType=%s",
		expected.ServiceName,
		expected.ScrapePort,
		expected.ScrapeType,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, serve.scrapes[expected.ServiceName])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_DoesNotAddAlert_WhenAlertNameIsEmpty() {
	rwMock := ResponseWriterMock{}
	req, _ := http.NewRequest("GET", "/v1/docker-flow-monitor", nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(0, len(serve.alerts))
}

func (s *ServerTestSuite) Test_ReconfigureHandler_DoesNotAddScrape_WhenScrapePortZero() {
	rwMock := ResponseWriterMock{}
	req, _ := http.NewRequest("GET", "/v1/docker-flow-monitor?serviceName=my-service", nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(0, len(serve.scrapes))
}

func (s *ServerTestSuite) Test_ReconfigureHandler_AddsAlertNameFormatted() {
	expected := prometheus.Alert{
		AlertName:          "my-alert",
		AlertIf:            "my-if",
		AlertFor:           "my-for",
		AlertNameFormatted: "myservice_myalert",
		AlertAnnotations:   map[string]string{},
		AlertLabels:        map[string]string{},
		ServiceName:        "my-service",
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?alertName=%s&alertIf=%s&alertFor=%s&serviceName=my-service",
		expected.AlertName,
		expected.AlertIf,
		expected.AlertFor,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, serve.alerts["myservice_myalert"])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_ReturnsJson() {
	reloadOrig := prometheus.Reload
	defer func() { prometheus.Reload = reloadOrig }()
	prometheus.Reload = func() error {
		return nil
	}
	expected := response{
		Status: http.StatusOK,
		Alerts: []prometheus.Alert{{
			ServiceName:        "my-service",
			AlertName:          "myalert",
			AlertIf:            "my-if",
			AlertFor:           "my-for",
			AlertNameFormatted: "myservice_myalert",
		}},
		Scrape: prometheus.Scrape{
			ServiceName: "my-service",
			ScrapePort:  1234,
		},
	}
	actual := response{}
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
	expected := `global:
  scrape_interval: 5s
alerting:
  alertmanagers:
  - static_configs:
    - targets:
      - alert-manager:9093
    scheme: http
rule_files:
- alert.rules
scrape_configs:
- job_name: my-service
  metrics_path: /metrics
  dns_sd_configs:
  - names:
    - tasks.my-service
    type: A
    port: 1234
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
	actualResponse := response{}
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
	actualResponse := response{}
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
	serve.scrapes["my-service-1"] = prometheus.Scrape{ServiceName: "my-service-1", ScrapePort: 1111}
	serve.scrapes["my-service-2"] = prometheus.Scrape{ServiceName: "my-service-2", ScrapePort: 2222}
	serve.RemoveHandler(rwMock, req)

	s.Len(serve.scrapes, 1)
}

func (s *ServerTestSuite) Test_RemoveHandler_RemovesAlerts() {
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service-1"
	req, _ := http.NewRequest("DELETE", addr, nil)

	serve := New()
	serve.alerts["myservice1alert1"] = prometheus.Alert{ServiceName: "my-service-1", AlertName: "my-alert-1"}
	serve.alerts["myservice1alert2"] = prometheus.Alert{ServiceName: "my-service-1", AlertName: "my-alert-1"}
	serve.alerts["myservice2alert1"] = prometheus.Alert{ServiceName: "my-service-2", AlertName: "my-alert-1"}
	serve.RemoveHandler(rwMock, req)

	s.Len(serve.alerts, 1)
}

func (s *ServerTestSuite) Test_RemoveHandler_ReturnsJson() {
	reloadOrig := prometheus.Reload
	defer func() { prometheus.Reload = reloadOrig }()
	prometheus.Reload = func() error {
		return nil
	}
	expected := response{
		Status: http.StatusOK,
		Alerts: []prometheus.Alert{
			{ServiceName: "my-service", AlertName: "my-alert"},
		},
		Scrape: prometheus.Scrape{
			ServiceName: "my-service",
			ScrapePort:  1234,
		},
	}
	actual := response{}
	rwMock := ResponseWriterMock{
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actual)
			return 0, nil
		},
	}
	addr := "/v1/docker-flow-monitor?serviceName=my-service"
	req, _ := http.NewRequest("DELETE", addr, nil)

	serve := New()
	serve.scrapes[expected.Scrape.ServiceName] = expected.Scrape
	alertKey := serve.getNameFormatted(fmt.Sprintf("%s%s", expected.Alerts[0].ServiceName, expected.Alerts[0].AlertName))
	serve.alerts[alertKey] = expected.Alerts[0]
	serve.RemoveHandler(rwMock, req)

	s.Equal(expected, actual)
}

func (s *ServerTestSuite) Test_RemoveHandler_CallsWriteConfig() {
	expectedAfterGet := `global:
  scrape_interval: 5s
alerting:
  alertmanagers:
  - static_configs:
    - targets:
      - alert-manager:9093
    scheme: http
scrape_configs:
- job_name: my-service
  metrics_path: /metrics
  dns_sd_configs:
  - names:
    - tasks.my-service
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

	expectedAfterDelete := `global:
  scrape_interval: 5s
alerting:
  alertmanagers:
  - static_configs:
    - targets:
      - alert-manager:9093
    scheme: http
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
	actualResponse := response{}
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
	actualResponse := response{}
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
		"service-1": {ServiceName: "service-1", ScrapePort: 1111},
		"service-2": {ServiceName: "service-2", ScrapePort: 2222},
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

	s.Equal(expected, serve.scrapes)
}

func (s *ServerTestSuite) Test_InitialConfig_AddsScrapesFromEnv() {
	expected := map[string]prometheus.Scrape{
		"service-1": {ServiceName: "service-1", ScrapePort: 1111},
		"service-2": {ServiceName: "service-2", ScrapePort: 2222},
		"service-4": {ServiceName: "service-4", ScrapePort: 4444, ScrapeType: "static_configs"},
		"service-3": {ServiceName: "service-3", ScrapePort: 3333, ScrapeType: "static_configs"},
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
	defer func() {
		os.Unsetenv("LISTENER_ADDRESS")
		os.Unsetenv("SCRAPE_PORT_1")
		os.Unsetenv("SERVICE_NAME_1")
	}()
	os.Setenv("LISTENER_ADDRESS", testServer.URL)
	os.Setenv("SCRAPE_PORT_1", "3333")
	os.Setenv("SERVICE_NAME_1", "service-3")

	serve := New()
	serve.InitialConfig()

	s.Equal(expected, serve.scrapes)
}

func (s *ServerTestSuite) Test_InitialConfig_ReturnsError_WhenPortFromEnvVarsCannotBeParsed() {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		js, _ := json.Marshal([]map[string]string{})
		w.Write(js)
	}))
	defer testServer.Close()
	defer func() {
		os.Unsetenv("LISTENER_ADDRESS")
		os.Unsetenv("SCRAPE_PORT_1")
		os.Unsetenv("SERVICE_NAME_1")
	}()
	os.Setenv("LISTENER_ADDRESS", testServer.URL)
	os.Setenv("SCRAPE_PORT_1", "xxxx") // This cannot be parsed to int
	os.Setenv("SERVICE_NAME_1", "service-3")

	serve := New()
	err := serve.InitialConfig()

	s.Error(err)
}

func (s *ServerTestSuite) Test_InitialConfig_ReturnsError_WhenEnvVarsAreMissing() {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		js, _ := json.Marshal([]map[string]string{})
		w.Write(js)
	}))
	defer testServer.Close()
	defer func() {
		os.Unsetenv("LISTENER_ADDRESS")
		os.Unsetenv("SCRAPE_PORT_1")
	}()
	os.Setenv("LISTENER_ADDRESS", testServer.URL)
	os.Setenv("SCRAPE_PORT_1", "1234")
	// `SERVICE_NAME_1` is missing

	serve := New()
	err := serve.InitialConfig()

	s.Error(err)
}

func (s *ServerTestSuite) Test_InitialConfig_AddsAlerts() {
	expected := map[string]prometheus.Alert{
		"myservice_alert1": {
			AlertAnnotations: map[string]string{},
			AlertName:        "alert-1",
			AlertIf:          "if-1",
			AlertFor:         "for-1",
			AlertLabels: map[string]string{
				"label-1-1": "value-1-1",
				"label-1-2": "value-1-2",
			},
			ServiceName:        "my-service",
			AlertNameFormatted: "myservice_alert1",
			Replicas:           4,
		},
		"myservice_alert21": {
			AlertAnnotations:   map[string]string{},
			AlertFor:           "for-21",
			AlertIf:            "if-21",
			AlertLabels:        map[string]string{},
			AlertName:          "alert-21",
			ServiceName:        "my-service",
			AlertNameFormatted: "myservice_alert21",
		},
		"myservice_alert22": {
			AlertAnnotations: map[string]string{
				"annotation-22-1": "value-22-1",
				"annotation-22-2": "value-22-2",
			},
			AlertFor:           "for-22",
			AlertIf:            "if-22",
			AlertLabels:        map[string]string{},
			AlertName:          "alert-22",
			ServiceName:        "my-service",
			AlertNameFormatted: "myservice_alert22",
		},
	}
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := []map[string]string{}
		// Is Not included since alertIf is missing
		resp = append(resp, map[string]string{
			"serviceName": "my-service-without-if",
			"alertName":   "alert-without-if",
		})
		resp = append(resp, map[string]string{
			"serviceName": "my-service",
			"replicas":    "4",
			"alertName":   "alert-1",
			"alertIf":     "if-1",
			"alertFor":    "for-1",
			"alertLabels": "label-1-1=value-1-1,label-1-2=value-1-2",
		})
		resp = append(resp, map[string]string{
			"serviceName":        "my-service",
			"alertName.1":        "alert-21",
			"alertIf.1":          "if-21",
			"alertFor.1":         "for-21",
			"alertName.2":        "alert-22",
			"alertAnnotations.2": "annotation-22-1=value-22-1,annotation-22-2=value-22-2",
			"alertIf.2":          "if-22",
			"alertFor.2":         "for-22",
		})
		resp = append(resp, map[string]string{"replicas": "4"})
		js, _ := json.Marshal(resp)
		w.Write(js)
	}))
	defer testServer.Close()
	defer func() { os.Unsetenv("LISTENER_ADDRESS") }()
	os.Setenv("LISTENER_ADDRESS", testServer.URL)

	serve := New()
	serve.InitialConfig()

	s.Equal(expected, serve.alerts)
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
