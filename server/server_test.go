package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"../prometheus"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"
	yaml "gopkg.in/yaml.v2"
)

type ServerTestSuite struct {
	suite.Suite
	reloadCalledNum int

	reloadOrig func() error
	runOrig    func() error
}

func (s *ServerTestSuite) SetupTest() {
	s.reloadOrig = prometheus.Reload
	s.runOrig = prometheus.Run
	s.reloadCalledNum = 0
	prometheus.Reload = func() error {
		s.reloadCalledNum++
		return nil
	}
	prometheus.Run = func() error {
		return nil
	}
}

func (s *ServerTestSuite) TearDownTest() {
	prometheus.Reload = s.reloadOrig
	prometheus.Run = s.runOrig
}

func TestServerUnitTestSuite(t *testing.T) {
	s := new(ServerTestSuite)
	logPrintlnOrig := logPrintf
	listenerTimeoutOrig := listenerTimeout

	fsOrig := FS
	defer func() {
		logPrintf = logPrintlnOrig
		listenerTimeout = listenerTimeoutOrig
		FS = fsOrig
	}()

	listenerTimeout = 10 * time.Millisecond
	logPrintf = func(format string, v ...interface{}) {}
	FS = afero.NewMemMapFs()

	// move ../confg/shortcuts.yaml file to `shortcutsPath`
	shortcutData, _ := afero.ReadFile(afero.NewOsFs(), "../conf/shortcuts.yaml")
	afero.WriteFile(FS, shortcutsPath, shortcutData, 0644)

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer testServer.Close()
	os.Setenv("GLOBAL_SCRAPE_INTERVAL", "5s")
	os.Setenv("ARG_CONFIG_FILE", "/etc/prometheus/prometheus.yml")
	os.Setenv("ARG_STORAGE_LOCAL_PATH", "/prometheus")
	os.Setenv("ARG_WEB_CONSOLE_LIBRARIES", "/usr/share/prometheus/console_libraries")
	os.Setenv("ARG_WEB_CONSOLE_TEMPLATES", "/usr/share/prometheus/consoles")
	os.Setenv("ARG_ALERTMANAGER_URL", "http://alert-manager:9093")

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
	s.Equal(1, s.reloadCalledNum)
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

func (s *ServerTestSuite) Test_ReconfigureHandler_UpdatesPersistantAlert() {
	// When service is reconfigured, all alert, including persistent alerts, will be removed
	// and queried again.
	expected := prometheus.Alert{
		ServiceName:        "my-service",
		AlertName:          "my-alert",
		AlertIf:            "a>b",
		AlertFor:           "my-for",
		AlertNameFormatted: "myservice_myalert",
		AlertAnnotations:   map[string]string{"a1": "v1", "a2": "v2"},
		AlertLabels:        map[string]string{"l1": "v1"},
		AlertPersistent:    true,
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=%s&alertFor=%s&alertAnnotations=%s&alertLabels=%s&alertPersistent=%t",
		expected.ServiceName,
		expected.AlertName,
		url.QueryEscape(expected.AlertIf),
		expected.AlertFor,
		url.QueryEscape("a1=v1,a2=v2"),
		url.QueryEscape("l1=v1"),
		expected.AlertPersistent,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, serve.alerts[expected.AlertNameFormatted])

	// Change alert slightly
	expected.AlertIf = "a<b"
	rwMock = ResponseWriterMock{}
	addr = fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=%s&alertFor=%s&alertAnnotations=%s&alertLabels=%s&alertPersistent=%t",
		expected.ServiceName,
		expected.AlertName,
		url.QueryEscape(expected.AlertIf),
		expected.AlertFor,
		url.QueryEscape("a1=v1,a2=v2"),
		url.QueryEscape("l1=v1"),
		expected.AlertPersistent,
	)
	req, _ = http.NewRequest("GET", addr, nil)

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

func (s *ServerTestSuite) Test_ReconfigureHandler_ExpandsShortcuts_Configured_WithSecrets() {
	alertIfSecret1 := `"@require":
  expanded: "{{ .Alert.ServiceName }} == {{ .Alert.Replicas }}"
  annotations:
    summary: "{{ .Alert.ServiceName }} equals {{ .Alert.Replicas }}"
  labels:
    receiver: system
    service: "{{ .Alert.ServiceName }}"
"@unused":
  expanded: not used
  annotations:
    summary: A summary
`
	alertIfSecret2 := `"@another":
  expanded: "{{ .Alert.ServiceName }} != {{ index .Values 0 }}"
  annotations:
    summary: "{{ .Alert.ServiceName }} is not {{ index .Values 0 }}"
  labels:
    receiver: node
`
	err := afero.WriteFile(FS, "/run/secrets/alertif-first", []byte(alertIfSecret1), 0644)
	if err != nil {
		s.Fail(err.Error())
	}
	err = afero.WriteFile(FS, "/run/secrets/alertif_second", []byte(alertIfSecret2), 0644)
	if err != nil {
		s.Fail(err.Error())
	}

	// Check first alertIf secret
	expected := prometheus.Alert{
		AlertAnnotations: map[string]string{
			"summary": "my-service equals 3"},
		AlertFor: "my-for",
		AlertIf:  "my-service == 3",
		AlertLabels: map[string]string{
			"receiver": "system", "service": "my-service"},
		AlertName:          "my-alert1",
		AlertNameFormatted: "myservice_myalert1",
		ServiceName:        "my-service",
		Replicas:           3,
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=@require&alertFor=%s&replicas=3",
		expected.ServiceName,
		expected.AlertName,
		expected.AlertFor,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, serve.alerts[expected.AlertNameFormatted])

	// Check second alertIf secret
	expected = prometheus.Alert{
		AlertAnnotations: map[string]string{
			"summary": "my-service is not 1"},
		AlertFor: "my-for",
		AlertIf:  "my-service != 1",
		AlertLabels: map[string]string{
			"receiver": "node"},
		AlertName:          "my-alert2",
		AlertNameFormatted: "myservice_myalert2",
		ServiceName:        "my-service",
		Replicas:           3,
	}
	rwMock = ResponseWriterMock{}
	addr = fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=@another:1&alertFor=%s&replicas=3",
		expected.ServiceName,
		expected.AlertName,
		expected.AlertFor,
	)
	req, _ = http.NewRequest("GET", addr, nil)

	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, serve.alerts[expected.AlertNameFormatted])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_ExpandsShortcuts_CompoundOps() {
	testData := []struct {
		expected    string
		shortcut    string
		annotations map[string]string
		labels      map[string]string
	}{
		{
			`sum(rate(http_server_resp_time_bucket{job="my-service", le="0.025"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) > 0.75 unless sum(rate(http_server_resp_time_bucket{job="my-service", le="0.1"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) < 0.99`,
			`@resp_time_below:0.025,5m,0.75_unless_@resp_time_above:0.1,5m,0.99`,
			map[string]string{"summary": "Response time of the service my-service is below 0.025 unless Response time of the service my-service is above 0.1"},
			map[string]string{},
		},
		{
			`sum(rate(http_server_resp_time_bucket{job="my-service", le="0.025"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) > 0.75 unless sum(rate(http_server_resp_time_bucket{job="my-service", le="0.1"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) < 0.99`,
			`@resp_time_below:0.025,5m,0.75_unless_@resp_time_above:0.1,5m,0.99`,
			map[string]string{"summary": "Response time of the service my-service is below 0.025 unless Response time of the service my-service is above 0.1"},
			map[string]string{"receiver": "system", "service": "my-service", "type": "service"},
		},
		{
			`sum(rate(http_server_resp_time_bucket{job="my-service", le="0.1"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) < 0.99 and container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="my-service"} > 0.8`,
			`@resp_time_above:0.1,5m,0.99_and_@service_mem_limit:0.8`,
			map[string]string{"summary": "Response time of the service my-service is above 0.1 and Memory of the service my-service is over 0.8"},
			map[string]string{"receiver": "system", "service": "my-service"},
		},
		{
			`sum(rate(http_server_resp_time_bucket{job="my-service", le="0.1"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) < 0.99 or container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="my-service"} > 0.8`,
			`@resp_time_above:0.1,5m,0.99_or_@service_mem_limit:0.8`,
			map[string]string{"summary": "Response time of the service my-service is above 0.1 or Memory of the service my-service is over 0.8"},
			map[string]string{"receiver": "system"},
		},
		{
			`container_memory_usage_bytes{container_label_com_docker_swarm_service_name="my-service"}/container_spec_memory_limit_bytes{container_label_com_docker_swarm_service_name="my-service"} > 0.8 and sum(rate(http_server_resp_time_bucket{job="my-service", le="0.025"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) > 0.75 unless sum(rate(http_server_resp_time_bucket{job="my-service", le="0.1"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) < 0.99`,
			`@service_mem_limit:0.8_and_@resp_time_below:0.025,5m,0.75_unless_@resp_time_above:0.1,5m,0.99`,
			map[string]string{"summary": "Memory of the service my-service is over 0.8 and Response time of the service my-service is below 0.025 unless Response time of the service my-service is above 0.1"},
			map[string]string{"receiver": "system"},
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
		alertQueries := []string{}
		for k, v := range data.labels {
			alertQueries = append(alertQueries, fmt.Sprintf("%s=%s", k, v))
		}
		alertQueryStr := strings.Join(alertQueries, ",")
		addr := fmt.Sprintf(
			"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=%s&alertFor=%s&replicas=3",
			expected.ServiceName,
			expected.AlertName,
			data.shortcut,
			expected.AlertFor,
		)
		if len(alertQueries) > 0 {
			addr += fmt.Sprintf("&alertLabels=%s", alertQueryStr)
		}
		req, _ := http.NewRequest("GET", addr, nil)

		serve := New()
		serve.ReconfigureHandler(rwMock, req)

		s.Equal(expected, serve.alerts[expected.AlertNameFormatted])
	}
}

func (s *ServerTestSuite) Test_ReconfigureHandler_DoesNotExpandAnnotations_WhenTheyAreAlreadySet_CompoundOps() {
	testData := struct {
		expected    string
		shortcut    string
		annotations map[string]string
		labels      map[string]string
	}{
		`sum(rate(http_server_resp_time_bucket{job="my-service", le="0.025"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) > 0.75 unless sum(rate(http_server_resp_time_bucket{job="my-service", le="0.1"}[5m])) / sum(rate(http_server_resp_time_count{job="my-service"}[5m])) < 0.99`,
		`@resp_time_below:0.025,5m,0.75_unless_@resp_time_above:0.1,5m,0.99`,
		map[string]string{"summary": "not-again"},
		map[string]string{"receiver": "system", "service": "ugly-service"},
	}
	expected := prometheus.Alert{
		AlertAnnotations:   testData.annotations,
		AlertFor:           "my-for",
		AlertIf:            testData.expected,
		AlertLabels:        testData.labels,
		AlertName:          "my-alert",
		AlertNameFormatted: "myservice_myalert",
		ServiceName:        "my-service",
		Replicas:           3,
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=%s&alertFor=%s&replicas=3&alertAnnotations=summary=not-again&alertLabels=service=ugly-service,receiver=system",
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
			AlertPersistent:    i == 2,
		})
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName.1=%s&alertIf.1=%s&alertFor.1=%s&alertName.2=%s&alertIf.2=%s&alertFor.2=%s&alertAnnotations.1=%s&alertAnnotations.2=%s&alertLabels.1=%s&alertLabels.2=%s&replicas=3&alertPersistent.2=true",
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

func (s *ServerTestSuite) Test_ReconfigureHandler_WithNodeInfo() {
	defer func() {
		os.Unsetenv("DF_SCRAPE_TARGET_LABELS")
	}()
	os.Setenv("DF_SCRAPE_TARGET_LABELS", "env,domain")
	nodeInfo := prometheus.NodeIPSet{}
	nodeInfo.Add("node-1", "1.0.1.1", "id1")
	nodeInfo.Add("node-2", "1.0.1.2", "id2")
	expected := prometheus.Scrape{
		ServiceName: "my-service",
		ScrapePort:  1234,
		ScrapeLabels: &map[string]string{
			"env":    "prod",
			"domain": "frontend",
		},
		NodeInfo: nodeInfo,
	}

	nodeInfoBytes, err := json.Marshal(nodeInfo)
	s.Require().NoError(err)

	rwMock := ResponseWriterMock{}
	addr, err := url.Parse("/v1/docker-flow-monitor")
	s.Require().NoError(err)

	q := addr.Query()
	q.Add("serviceName", expected.ServiceName)
	q.Add("scrapePort", fmt.Sprintf("%d", expected.ScrapePort))
	q.Add("env", (*expected.ScrapeLabels)["env"])
	q.Add("domain", (*expected.ScrapeLabels)["domain"])
	q.Add("extra", "system")
	q.Add("nodeInfo", string(nodeInfoBytes))
	addr.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", addr.String(), nil)
	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	targetScrape := serve.scrapes[expected.ServiceName]
	s.Equal(expected.ServiceName, targetScrape.ServiceName)
	s.Equal(expected.ScrapePort, targetScrape.ScrapePort)
	s.Equal(expected.ScrapeLabels, targetScrape.ScrapeLabels)
	s.Equal("", (*targetScrape.ScrapeLabels)["extra"])

	s.Require().NotNil(targetScrape.NodeInfo)
	s.True(expected.NodeInfo.Equal(targetScrape.NodeInfo))

}

func (s *ServerTestSuite) Test_ReconfigureHandler_WithNodeInfo_NoTargetLabelsDefined() {
	nodeInfo := prometheus.NodeIPSet{}
	nodeInfo.Add("node-1", "1.0.1.1", "id1")
	nodeInfo.Add("node-2", "1.0.1.2", "id2")
	expected := prometheus.Scrape{
		ServiceName:  "my-service",
		ScrapePort:   1234,
		ScrapeLabels: &map[string]string{},
		NodeInfo:     nodeInfo,
	}

	nodeInfoBytes, err := json.Marshal(nodeInfo)
	s.Require().NoError(err)

	rwMock := ResponseWriterMock{}
	addr, err := url.Parse("/v1/docker-flow-monitor")
	s.Require().NoError(err)

	q := addr.Query()
	q.Add("serviceName", expected.ServiceName)
	q.Add("scrapePort", fmt.Sprintf("%d", expected.ScrapePort))
	q.Add("env", "dev")
	q.Add("domain", "frontend")
	q.Add("nodeInfo", string(nodeInfoBytes))
	addr.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", addr.String(), nil)
	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	targetScrape := serve.scrapes[expected.ServiceName]
	s.Equal(expected.ServiceName, targetScrape.ServiceName)
	s.Equal(expected.ScrapePort, targetScrape.ScrapePort)
	s.Equal(expected.ScrapeLabels, targetScrape.ScrapeLabels)

	s.Require().NotNil(targetScrape.NodeInfo)
	s.True(expected.NodeInfo.Equal(targetScrape.NodeInfo))
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
	s.Equal(1, s.reloadCalledNum)
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
  scrape_interval: 15s
  scrape_timeout: 11s
  metrics_path: /metrics
  dns_sd_configs:
  - names:
    - tasks.my-service
    type: A
    port: 1234
`
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service&scrapePort=1234&scrapeInterval=15s&scrapeTimeout=11s&alertName=my-alert&alertIf=my-if&alertFor=my-for"
	req, _ := http.NewRequest("GET", addr, nil)
	fsOrig := prometheus.FS
	defer func() { prometheus.FS = fsOrig }()
	prometheus.FS = afero.NewMemMapFs()

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	actual, _ := afero.ReadFile(prometheus.FS, "/etc/prometheus/prometheus.yml")
	s.Equal(expected, string(actual))
}

func (s *ServerTestSuite) Test_ReconfigureHandler_WithNodeInfo_CallsWriteConfig() {
	fsOrig := prometheus.FS
	defer func() {
		os.Unsetenv("DF_SCRAPE_TARGET_LABELS")
		prometheus.FS = fsOrig
	}()
	prometheus.FS = afero.NewMemMapFs()
	os.Setenv("DF_SCRAPE_TARGET_LABELS", "env,domain")
	expectedConfig := `global:
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
  file_sd_configs:
  - files:
    - /etc/prometheus/file_sd/my-service.json
`
	nodeInfo := prometheus.NodeIPSet{}
	nodeInfo.Add("node-1", "1.0.1.1", "id1")
	nodeInfo.Add("node-2", "1.0.1.2", "id2")
	expected := prometheus.Scrape{
		ServiceName: "my-service",
		ScrapePort:  1234,
		ScrapeLabels: &map[string]string{
			"env":    "prod",
			"domain": "frontend",
		},
		NodeInfo: nodeInfo,
	}

	nodeInfoBytes, err := json.Marshal(nodeInfo)
	s.Require().NoError(err)

	rwMock := ResponseWriterMock{}
	addr, err := url.Parse("/v1/docker-flow-monitor")
	s.Require().NoError(err)

	q := addr.Query()
	q.Add("serviceName", expected.ServiceName)
	q.Add("scrapePort", fmt.Sprintf("%d", expected.ScrapePort))
	q.Add("env", (*expected.ScrapeLabels)["env"])
	q.Add("domain", (*expected.ScrapeLabels)["domain"])
	q.Add("nodeInfo", string(nodeInfoBytes))
	q.Add("alertName", "my-alert")
	q.Add("alertIf", "my-if")
	q.Add("alertFor", "my-for")
	addr.RawQuery = q.Encode()

	req, _ := http.NewRequest("GET", addr.String(), nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	actualPrometheusConfig, err := afero.ReadFile(prometheus.FS, "/etc/prometheus/prometheus.yml")
	s.Require().NoError(err)
	s.Equal(expectedConfig, string(actualPrometheusConfig))

	fileSDConfigByte, err := afero.ReadFile(prometheus.FS, "/etc/prometheus/file_sd/my-service.json")
	s.Require().NoError(err)

	fileSDconfig := prometheus.FileStaticConfig{}
	err = json.Unmarshal(fileSDConfigByte, &fileSDconfig)
	s.Require().NoError(err)

	s.Require().Len(fileSDconfig, 2)

	var targetGroup1 *prometheus.TargetGroup
	var targetGroup2 *prometheus.TargetGroup
	for _, tg := range fileSDconfig {
		if tg == nil {
			continue
		}
		for _, target := range tg.Targets {
			if target == "1.0.1.1:1234" {
				targetGroup1 = tg
				break
			} else if target == "1.0.1.2:1234" {
				targetGroup2 = tg
				break
			}
		}
	}

	s.Require().NotNil(targetGroup1)
	s.Require().NotNil(targetGroup2)

	s.Equal((*expected.ScrapeLabels)["env"], targetGroup1.Labels["env"])
	s.Equal((*expected.ScrapeLabels)["domain"], targetGroup1.Labels["domain"])
	s.Equal("node-1", targetGroup1.Labels["node"])

	s.Equal((*expected.ScrapeLabels)["env"], targetGroup2.Labels["env"])
	s.Equal("node-2", targetGroup2.Labels["node"])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_SendsReloadRequestToPrometheus() {
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service&scrapePort=1234"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(1, s.reloadCalledNum)
}

func (s *ServerTestSuite) Test_ReconfigureHandler_ReturnsNokWhenPrometheusReloadFails() {
	prometheus.Reload = func() error {
		s.reloadCalledNum++
		return errors.New("Prometheus error")
	}
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
	prometheus.Reload = func() error {
		s.reloadCalledNum++
		return errors.New("Prometheus error")
	}
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

func (s *ServerTestSuite) Test_RemoveHandler_KeepsPersistantAlerts() {

	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service-1"
	req, _ := http.NewRequest("DELETE", addr, nil)

	serve := New()
	serve.alerts["myservice1alert1"] = prometheus.Alert{ServiceName: "my-service-1", AlertName: "my-alert-1", AlertPersistent: true}
	serve.alerts["myservice1alert2"] = prometheus.Alert{ServiceName: "my-service-1", AlertName: "my-alert-2"}
	serve.alerts["myservice2alert1"] = prometheus.Alert{ServiceName: "my-service-2", AlertName: "my-alert-1"}
	serve.RemoveHandler(rwMock, req)

	s.Len(serve.alerts, 2)
}

func (s *ServerTestSuite) Test_RemoveHandler_ReturnsJson() {
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
	s.Equal(1, s.reloadCalledNum)
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

func (s *ServerTestSuite) Test_RemoveHandler_WithNodeInfo_CallsWriteConfig() {
	fsOrig := prometheus.FS
	defer func() {
		prometheus.FS = fsOrig
	}()
	prometheus.FS = afero.NewMemMapFs()
	nodeInfo1 := prometheus.NodeIPSet{}
	nodeInfo1.Add("node-1", "1.0.1.1", "id1")
	nodeInfo1.Add("node-2", "1.0.1.2", "id2")
	expected1 := prometheus.Scrape{
		ServiceName:  "my-service1",
		ScrapePort:   1234,
		ScrapeLabels: &map[string]string{},
		NodeInfo:     nodeInfo1,
	}

	nodeInfo2 := prometheus.NodeIPSet{}
	nodeInfo2.Add("node-1", "1.0.2.1", "id1")
	expected2 := prometheus.Scrape{
		ServiceName:  "my-service2",
		ScrapePort:   2341,
		ScrapeLabels: &map[string]string{},
		NodeInfo:     nodeInfo2,
	}

	nodeInfoBytes1, err := json.Marshal(nodeInfo1)
	s.Require().NoError(err)
	nodeInfoBytes2, err := json.Marshal(nodeInfo2)
	s.Require().NoError(err)

	rwMock := ResponseWriterMock{}
	addr, err := url.Parse("/v1/docker-flow-monitor")
	s.Require().NoError(err)

	q1 := addr.Query()
	q1.Add("serviceName", expected1.ServiceName)
	q1.Add("scrapePort", fmt.Sprintf("%d", expected1.ScrapePort))
	q1.Add("nodeInfo", string(nodeInfoBytes1))

	q2 := addr.Query()
	q2.Add("serviceName", expected2.ServiceName)
	q2.Add("scrapePort", fmt.Sprintf("%d", expected2.ScrapePort))
	q2.Add("nodeInfo", string(nodeInfoBytes2))

	serve := New()

	addr.RawQuery = q1.Encode()
	req1, _ := http.NewRequest("GET", addr.String(), nil)
	serve.ReconfigureHandler(rwMock, req1)

	addr.RawQuery = q2.Encode()
	req2, _ := http.NewRequest("GET", addr.String(), nil)
	serve.ReconfigureHandler(rwMock, req2)

	actualPrometheusConfigBytes, err := afero.ReadFile(prometheus.FS, "/etc/prometheus/prometheus.yml")
	s.Require().NoError(err)

	actualPrometheusConfig := prometheus.Config{}
	err = yaml.Unmarshal(actualPrometheusConfigBytes, &actualPrometheusConfig)
	s.Require().NoError(err)
	s.Len(actualPrometheusConfig.ScrapeConfigs, 2)

	var sdConfig1 *prometheus.SDConfig
	var sdConfig2 *prometheus.SDConfig

	for _, sc := range actualPrometheusConfig.ScrapeConfigs {
		if sc.JobName == "my-service1" {
			s.Require().Len(sc.ServiceDiscoveryConfig.FileSDConfigs, 1)
			sdConfig1 = sc.ServiceDiscoveryConfig.FileSDConfigs[0]
		}
		if sc.JobName == "my-service2" {
			s.Require().Len(sc.ServiceDiscoveryConfig.FileSDConfigs, 1)
			sdConfig2 = sc.ServiceDiscoveryConfig.FileSDConfigs[0]
		}
	}
	s.Require().NotNil(sdConfig1)
	s.Require().NotNil(sdConfig2)
	s.Len(sdConfig1.Files, 1)
	s.Len(sdConfig2.Files, 1)
	s.Equal("/etc/prometheus/file_sd/my-service1.json", sdConfig1.Files[0])
	s.Equal("/etc/prometheus/file_sd/my-service2.json", sdConfig2.Files[0])

	// my-service1 has two servies
	fileSDConfigService1Byte, err := afero.ReadFile(prometheus.FS, "/etc/prometheus/file_sd/my-service1.json")
	s.Require().NoError(err)
	fileSDconfig1 := prometheus.FileStaticConfig{}
	err = json.Unmarshal(fileSDConfigService1Byte, &fileSDconfig1)
	s.Require().NoError(err)
	s.Require().Len(fileSDconfig1, 2)

	// my-service2 has one servies
	fileSDConfigService2Byte, err := afero.ReadFile(prometheus.FS, "/etc/prometheus/file_sd/my-service2.json")
	s.Require().NoError(err)
	fileSDconfig2 := prometheus.FileStaticConfig{}
	err = json.Unmarshal(fileSDConfigService2Byte, &fileSDconfig2)
	s.Require().NoError(err)
	s.Require().Len(fileSDconfig2, 1)

	// Delete my-service1
	addrDelete1 := "/v1/docker-flow-monitor?serviceName=my-service1"
	reqDelete1, _ := http.NewRequest("DELETE", addrDelete1, nil)

	serve.RemoveHandler(rwMock, reqDelete1)

	actualConfigBytes, _ := afero.ReadFile(prometheus.FS, "/etc/prometheus/prometheus.yml")
	// Config did not change since there is still a service being scraped
	actualPrometheusConfigAfter := prometheus.Config{}
	err = yaml.Unmarshal(actualConfigBytes, &actualPrometheusConfigAfter)
	s.Require().NoError(err)
	s.Len(actualPrometheusConfigAfter.ScrapeConfigs, 1)

	// my-service1 is gone
	myService1Exists, err := afero.Exists(prometheus.FS, "/etc/prometheus/file_sd/my-service1.json")
	s.Require().NoError(err)
	s.False(myService1Exists)

	fileSDConfigService2Byte, err = afero.ReadFile(prometheus.FS, "/etc/prometheus/file_sd/my-service2.json")
	s.Require().NoError(err)
	fileSDconfig2After := prometheus.FileStaticConfig{}
	err = json.Unmarshal(fileSDConfigService2Byte, &fileSDconfig2After)
	s.Require().NoError(err)

	// my-service2 is still running running
	s.Require().Len(fileSDconfig2After, 1)

	// Delete my-service2
	addrDelete2 := "/v1/docker-flow-monitor?serviceName=my-service2"
	reqDelete2, _ := http.NewRequest("DELETE", addrDelete2, nil)

	serve.RemoveHandler(rwMock, reqDelete2)

	expectedConfigDelete := `global:
  scrape_interval: 5s
alerting:
  alertmanagers:
  - static_configs:
    - targets:
      - alert-manager:9093
    scheme: http
`

	actualConfig, _ := afero.ReadFile(prometheus.FS, "/etc/prometheus/prometheus.yml")
	// Config did not change since there is still a service being scraped
	s.Equal(expectedConfigDelete, string(actualConfig))

	// my-service2 is gone
	myService2Exists, err := afero.Exists(prometheus.FS, "/etc/prometheus/file_sd/my-service2.json")
	s.Require().NoError(err)
	s.False(myService2Exists)
}

func (s *ServerTestSuite) Test_RemoveHandler_SendsReloadRequestToPrometheus() {
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service"
	req, _ := http.NewRequest("DELETE", addr, nil)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer testServer.Close()

	serve := New()
	serve.RemoveHandler(rwMock, req)

	s.Equal(1, s.reloadCalledNum)
}

func (s *ServerTestSuite) Test_RemoveHandler_ReturnsNokWhenPrometheusReloadFails() {

	prometheus.Reload = func() error {
		return errors.New("Prometheus failed loading")
	}
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
	prometheus.Reload = func() error {
		return errors.New("Prometheus failed loading")
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

func (s *ServerTestSuite) Test_InitialConfig_CallsWriteConfig() {
	expected := map[string]map[string]string{
		"node1id": map[string]string{
			"awsregion": "us-east",
			"role":      "worker",
		},
		"node2id": map[string]string{
			"awsregion": "us-west",
			"role":      "manager",
		},
		"node3id": map[string]string{
			"role":     "manager",
			"back_end": "placeholder",
		},
	}

	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		resp := []map[string]string{}
		resp = append(resp, map[string]string{
			"id":           "node1id",
			"hostname":     "node1hostname",
			"address":      "10.0.0.1",
			"versionIndex": "24",
			"state":        "up",
			"role":         "worker",
			"availability": "active",
			"awsregion":    "us-east",
		})
		resp = append(resp, map[string]string{
			"id":           "node2id",
			"hostname":     "node2hostname",
			"address":      "10.0.1.1",
			"versionIndex": "24",
			"state":        "up",
			"role":         "manager",
			"availability": "active",
			"awsregion":    "us-west",
		})

		// Does not have awsregion
		resp = append(resp, map[string]string{
			"id":           "node3id",
			"hostname":     "node3hostname",
			"address":      "10.0.2.1",
			"versionIndex": "24",
			"state":        "up",
			"role":         "manager",
			"availability": "active",
			"back_end":     "placeholder",
		})
		js, _ := json.Marshal(resp)
		w.Write(js)
	}))

	defer func() {
		os.Unsetenv("DF_GET_NODES_URL")
		os.Unsetenv("DF_NODE_TARGET_LABELS")
	}()
	os.Setenv("DF_GET_NODES_URL", testServer.URL)
	os.Setenv("DF_NODE_TARGET_LABELS", "awsregion,role,back_end")

	serve := New()
	err := serve.InitialConfig()
	s.Require().NoError(err)

	s.Equal(expected, serve.nodeLabels)
}

// ReconfigureNodeHandler

func (s *ServerTestSuite) Test_ReconfigureNodeHandler_AddsNodes_WithoutNodeID() {

	actual := 0
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(status int) {
			actual = status
		},
	}
	addr := "/v1/docker-flow-monitor/node/reconfigure?role=worker&hostname=node1hostname&awsregion=us-east1&address=1.0.0.1"
	req, _ := http.NewRequest("GET", addr, nil)
	serve := New()
	serve.ReconfigureNodeHandler(rwMock, req)

	s.Equal(http.StatusBadRequest, actual)
}

func (s *ServerTestSuite) Test_ReconfigureNodeHandler_AddsNodes_CallsWriteConfig() {
	fsOrig := prometheus.FS
	defer func() {
		os.Unsetenv("DF_NODE_TARGET_LABELS")
		prometheus.FS = fsOrig
	}()
	os.Setenv("DF_NODE_TARGET_LABELS", "awsregion,role")
	prometheus.FS = afero.NewMemMapFs()
	nodeInfo := prometheus.NodeIPSet{}
	nodeInfo.Add("node-1", "1.0.1.1", "node1id")
	expectedScrape := prometheus.Scrape{
		ServiceName:  "my-service",
		ScrapePort:   1234,
		ScrapeLabels: &map[string]string{},
		NodeInfo:     nodeInfo,
	}
	expectedNodeLabel := map[string]map[string]string{
		"node1id": map[string]string{
			"awsregion": "us-east1",
			"role":      "worker",
		},
	}

	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor/node/reconfigure?role=worker&id=node1id&hostname=node1hostname&awsregion=us-east1&address=1.0.0.1"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()

	// Insert scrape for testing
	serve.scrapes[expectedScrape.ServiceName] = expectedScrape
	serve.ReconfigureNodeHandler(rwMock, req)

	s.Equal(expectedNodeLabel, serve.nodeLabels)

	// Check static file for node labels
	myServiceExists, err := afero.Exists(prometheus.FS, "/etc/prometheus/file_sd/my-service.json")
	s.Require().NoError(err)
	s.True(myServiceExists)

	fileSDConfigServiceByte, err := afero.ReadFile(prometheus.FS, "/etc/prometheus/file_sd/my-service.json")
	s.Require().NoError(err)
	fileSDconfig := prometheus.FileStaticConfig{}
	err = json.Unmarshal(fileSDConfigServiceByte, &fileSDconfig)
	s.Require().NoError(err)
	s.Require().Len(fileSDconfig, 1)

	var serviceNode1Tg *prometheus.TargetGroup

	for _, tg := range fileSDconfig {
		for _, target := range tg.Targets {
			if target == "1.0.1.1:1234" {
				serviceNode1Tg = tg
				break
			}
		}
	}
	s.Require().NotNil(serviceNode1Tg)

	s.Equal("us-east1", serviceNode1Tg.Labels["awsregion"])
	s.Equal("worker", serviceNode1Tg.Labels["role"])

}

// RemoveNodeHandler

func (s *ServerTestSuite) Test_ReconfigureNodeHandler_RemoveNodes_WithoutNodeID() {

	actual := 0
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(status int) {
			actual = status
		},
	}
	addr := "/v1/docker-flow-monitor/node/reconfigure?role=worker&hostname=node1hostname&awsregion=us-east1&address=1.0.0.1"
	req, _ := http.NewRequest("GET", addr, nil)
	serve := New()
	serve.RemoveNodeHandler(rwMock, req)

	s.Equal(http.StatusBadRequest, actual)
}

func (s *ServerTestSuite) Test_RemoveNodeHandler_RemoveNodes() {

	// Add node
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor/node/reconfigure?role=worker&id=node1id&hostname=node1hostname&awsregion=us-east1&address=1.0.0.1"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureNodeHandler(rwMock, req)

	// Remove node
	addr = "/v1/docker-flow-monitor/node/remove?id=node1id&hostname=node1hostname&awsregion=us-east1"
	req, _ = http.NewRequest("DELETE", addr, nil)

	serve.RemoveNodeHandler(rwMock, req)

	s.Len(serve.nodeLabels, 0)
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
