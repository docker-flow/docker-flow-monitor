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
	"net/url"
)

type ServerTestSuite struct {
	suite.Suite
}

func (s *ServerTestSuite) SetupTest() {
}

func TestServerUnitTestSuite(t *testing.T) {
	s := new(ServerTestSuite)
	logPrintlnOrig := logPrintf
	defer func() { logPrintf = logPrintlnOrig }()
	logPrintf = func(format string, v ...interface{}) {}
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer testServer.Close()
	prometheusAddrOrig := prometheusAddr
	defer func() { prometheusAddr = prometheusAddrOrig }()
	prometheusAddr = testServer.URL
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
	fsOrig := fs
	defer func() { fs = fsOrig }()
	fs = afero.NewMemMapFs()

	serve := New()
	serve.Execute()

	actual, _ := afero.ReadFile(fs, "/etc/prometheus/prometheus.yml")
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
	cmdRunOrig := cmdRun
	defer func() { cmdRun = cmdRunOrig }()
	cmdRun = func(cmd *exec.Cmd) error {
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
	expected := Alert{
		ServiceName: "my-service",
		AlertName: "my-alert",
		AlertIf: "a>b",
		AlertFrom: "my-from",
		AlertNameFormatted: "myservicemyalert",
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=%s&alertFrom=%s",
		expected.ServiceName,
		expected.AlertName,
		url.QueryEscape(expected.AlertIf),
		expected.AlertFrom,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, serve.Alerts[expected.AlertNameFormatted])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_RemovesOldAlerts() {
	expected := Alert{
		ServiceName: "my-service",
		AlertName: "my-alert",
		AlertIf: "a>b",
		AlertFrom: "my-from",
		AlertNameFormatted: "myservicemyalert",
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName=%s&alertIf=%s&alertFrom=%s",
		expected.ServiceName,
		expected.AlertName,
		url.QueryEscape(expected.AlertIf),
		expected.AlertFrom,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.Alerts["myservicesomeotheralert"] = Alert{
		ServiceName: "my-service",
		AlertName: "some-other-alert",
	}
	serve.Alerts["anotherservicemyalert"] = Alert{
		ServiceName: "another-service",
		AlertName: "my-alert",
	}
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(2, len(serve.Alerts))
	s.Equal(expected, serve.Alerts[expected.AlertNameFormatted])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_AddsMultipleAlerts() {
	expected := []Alert{}
	for i:=1; i <=2; i++ {
		expected = append(expected, Alert{
			ServiceName: "my-service",
			AlertName: fmt.Sprintf("my-alert-%d", i),
			AlertIf: fmt.Sprintf("my-if-%d", i),
			AlertFrom: fmt.Sprintf("my-from-%d", i),
			AlertNameFormatted: fmt.Sprintf("myservicemyalert%d", i),
		})
	}
	rwMock := ResponseWriterMock{}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor?serviceName=%s&alertName.1=%s&alertIf.1=%s&alertFrom.1=%s&alertName.2=%s&alertIf.2=%s&alertFrom.2=%s",
		expected[0].ServiceName,
		expected[0].AlertName,
		expected[0].AlertIf,
		expected[0].AlertFrom,
		expected[1].AlertName,
		expected[1].AlertIf,
		expected[1].AlertFrom,
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
	expected := Alert{
		AlertName: "my-alert",
		AlertIf: "my-if",
		AlertFrom: "my-from",
		AlertNameFormatted: "myalert",
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
	serve.ReconfigureHandler(rwMock, req)

	s.Equal(expected, serve.Alerts["myalert"])
}

func (s *ServerTestSuite) Test_ReconfigureHandler_ReturnsJson() {
	cmdRunOrig := cmdRun
	defer func() { cmdRun = cmdRunOrig }()
	cmdRun = func(cmd *exec.Cmd) error {
		return nil
	}
	expected := Response{
		Status: http.StatusOK,
		Alerts: []Alert{Alert{
			ServiceName: "my-service",
			AlertName: "myalert",
			AlertIf: "my-if",
			AlertFrom: "my-from",
			AlertNameFormatted: "myservicemyalert",
		}},
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
		expected.Alerts[0].AlertName,
		expected.Alerts[0].AlertIf,
		expected.Alerts[0].AlertFrom,
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
	addr := "/v1/docker-flow-monitor?serviceName=my-service&scrapePort=1234&alertName=my-alert&alertIf=my-if&alertFrom=my-from"
	req, _ := http.NewRequest("GET", addr, nil)
	fsOrig := fs
	defer func() { fs = fsOrig }()
	fs = afero.NewMemMapFs()

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	actual, _ := afero.ReadFile(fs, "/etc/prometheus/prometheus.yml")
	s.Equal(expected, string(actual))
}

func (s *ServerTestSuite) Test_ReconfigureHandler_SendsReloadRequestToPrometheus() {
	cmdRunOrig := cmdRun
	defer func() { cmdRun = cmdRunOrig }()
	actualArgs := [][]string{}
	cmdRun = func(cmd *exec.Cmd) error {
		actualArgs = append(actualArgs, cmd.Args)
		return nil
	}
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service&scrapePort=1234"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	s.Contains(actualArgs, []string{"pkill", "-HUP", "prometheus"})
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
	prometheusAddrOrig := prometheusAddr
	defer func() { prometheusAddr = prometheusAddrOrig }()
	prometheusAddr = "this-url-does-not-exist"

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
	serve.Scrapes["my-service-1"] = Scrape{ServiceName: "my-service-1", ScrapePort: 1111}
	serve.Scrapes["my-service-2"] = Scrape{ServiceName: "my-service-2", ScrapePort: 2222}
	serve.RemoveHandler(rwMock, req)

	s.Len(serve.Scrapes, 1)
}

func (s *ServerTestSuite) Test_RemoveHandler_RemovesAlerts() {
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service-1"
	req, _ := http.NewRequest("DELETE", addr, nil)

	serve := New()
	serve.Alerts["myservice1alert1"] = Alert{ServiceName: "my-service-1", AlertName: "my-alert-1"}
	serve.Alerts["myservice1alert2"] = Alert{ServiceName: "my-service-1", AlertName: "my-alert-1"}
	serve.Alerts["myservice2alert1"] = Alert{ServiceName: "my-service-2", AlertName: "my-alert-1"}
	serve.RemoveHandler(rwMock, req)

	s.Len(serve.Alerts, 1)
}

func (s *ServerTestSuite) Test_RemoveHandler_ReturnsJson() {
	cmdRunOrig := cmdRun
	defer func() { cmdRun = cmdRunOrig }()
	cmdRun = func(cmd *exec.Cmd) error {
		return nil
	}
	expected := Response{
		Status: http.StatusOK,
		Alerts: []Alert{
			Alert{ServiceName: "my-service", AlertName: "my-alert"},
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
	addr := "/v1/docker-flow-monitor?serviceName=my-service"
	req, _ := http.NewRequest("DELETE", addr, nil)

	serve := New()
	serve.Scrapes[expected.Scrape.ServiceName] = expected.Scrape
	alertKey := serve.getAlertNameFormatted(expected.Alerts[0].ServiceName, expected.Alerts[0].AlertName)
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
	fsOrig := fs
	defer func() { fs = fsOrig }()
	fs = afero.NewMemMapFs()

	serve := New()
	serve.ReconfigureHandler(rwMock, req)

	actual, _ := afero.ReadFile(fs, "/etc/prometheus/prometheus.yml")
	s.Equal(expectedAfterGet, string(actual))

	expectedAfterDelete := `
global:
  scrape_interval: 5s
`
	addr = "/v1/docker-flow-monitor?serviceName=my-service"
	req, _ = http.NewRequest("DELETE", addr, nil)

	serve.RemoveHandler(rwMock, req)

	actual, _ = afero.ReadFile(fs, "/etc/prometheus/prometheus.yml")
	s.Equal(expectedAfterDelete, string(actual))
}

func (s *ServerTestSuite) Test_RemoveHandler_SendsReloadRequestToPrometheus() {
	cmdRunOrig := cmdRun
	defer func() { cmdRun = cmdRunOrig }()
	actualArgs := [][]string{}
	cmdRun = func(cmd *exec.Cmd) error {
		actualArgs = append(actualArgs, cmd.Args)
		return nil
	}
	rwMock := ResponseWriterMock{}
	addr := "/v1/docker-flow-monitor?serviceName=my-service"
	req, _ := http.NewRequest("DELETE", addr, nil)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer testServer.Close()
	prometheusAddrOrig := prometheusAddr
	defer func() { prometheusAddr = prometheusAddrOrig }()
	prometheusAddr = testServer.URL

	serve := New()
	serve.RemoveHandler(rwMock, req)

	s.Contains(actualArgs, []string{"pkill", "-HUP", "prometheus"})
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
	gc := serve.GetGlobalConfig()
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

func (s *ServerTestSuite) Test_WriteConfig_WritesAlerts() {
	fsOrig := fs
	defer func() { fs = fsOrig }()
	fs = afero.NewMemMapFs()
	serve := New()
	serve.Alerts["myalert"] = Alert{
		ServiceName: "my-service",
		AlertName: "alert-name",
		AlertNameFormatted: "myservicealertname",
		AlertIf: "a>b",
	}
	gc := serve.GetGlobalConfig()
	expectedConfig := fmt.Sprintf(`%s

rule_files:
  - 'alert.rules'
`,
		gc,
	)
	expectedAlerts := serve.GetAlertConfig()

	serve.WriteConfig()

	actualConfig, _ := afero.ReadFile(fs, "/etc/prometheus/prometheus.yml")
	s.Equal(expectedConfig, string(actualConfig))
	actualAlerts, _ := afero.ReadFile(fs, "/etc/prometheus/alert.rules")
	s.Equal(expectedAlerts, string(actualAlerts))
}

// GetGlobalConfig

func (s *ServerTestSuite) Test_GlobalConfig_ReturnsConfigWithData() {
	scrapeIntervalOrig := os.Getenv("GLOBAL_SCRAPE_INTERVAL")
	defer func() { os.Setenv("GLOBAL_SCRAPE_INTERVAL", scrapeIntervalOrig) }()
	os.Setenv("GLOBAL_SCRAPE_INTERVAL", "123s")
	serve := New()
	expected := `
global:
  scrape_interval: 123s`

	actual := serve.GetGlobalConfig()
	s.Equal(expected, actual)
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
	expected := ""
	for _, i := range []int{1, 2} {
		expected += fmt.Sprintf(`
ALERT alertNameFormatted%d
  IF alert-if-%d
  FROM alert-from-%d
`, i, i, i)
		serve.Alerts[fmt.Sprintf("alert-name-%d", i)] = Alert{
			AlertNameFormatted: fmt.Sprintf("alertNameFormatted%d", i),
			ServiceName: fmt.Sprintf("my-service-%d", i),
			AlertName: fmt.Sprintf("alert-name-%d", i),
			AlertIf: fmt.Sprintf("alert-if-%d", i),
			AlertFrom: fmt.Sprintf("alert-from-%d", i),
		}
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

func (s *ServerTestSuite) Test_RunPrometheus_AddsRoutePrefix() {
	cmdRunOrig := cmdRun
	defer func() {
		cmdRun = cmdRunOrig
		os.Unsetenv("ARG_WEB_ROUTE-PREFIX")
	}()
	os.Setenv("ARG_WEB_ROUTE-PREFIX", "/something")
	actualArgs := []string{}
	cmdRun = func(cmd *exec.Cmd) error {
		actualArgs = cmd.Args
		return nil
	}

	serve := New()
	serve.RunPrometheus()

	s.Equal([]string{"/bin/sh", "-c", "prometheus -config.file=/etc/prometheus/prometheus.yml -storage.local.path=/prometheus -web.console.libraries=/usr/share/prometheus/console_libraries -web.console.templates=/usr/share/prometheus/consoles -web.route-prefix=/something"}, actualArgs)
}

func (s *ServerTestSuite) Test_RunPrometheus_AddsExternalUrl() {
	cmdRunOrig := cmdRun
	defer func() {
		cmdRun = cmdRunOrig
		os.Unsetenv("ARG_WEB_EXTERNAL-URL")
	}()
	os.Setenv("ARG_WEB_EXTERNAL-URL", "/something")
	actualArgs := []string{}
	cmdRun = func(cmd *exec.Cmd) error {
		actualArgs = cmd.Args
		return nil
	}

	serve := New()
	serve.RunPrometheus()

	s.Equal([]string{"/bin/sh", "-c", "prometheus -config.file=/etc/prometheus/prometheus.yml -storage.local.path=/prometheus -web.console.libraries=/usr/share/prometheus/console_libraries -web.console.templates=/usr/share/prometheus/consoles -web.external-url=/something"}, actualArgs)
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
