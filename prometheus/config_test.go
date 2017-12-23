package prometheus

import (
	"fmt"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"
)

type ConfigTestSuite struct {
	suite.Suite
}

func (s *ConfigTestSuite) SetupTest() {
}

func TestConfigUnitTestSuite(t *testing.T) {
	s := new(ConfigTestSuite)
	logPrintlnOrig := logPrintf
	os.Setenv("ARG_ALERTMANAGER_URL", "http://alert-manager:9093")
	defer func() {
		logPrintf = logPrintlnOrig
		os.Unsetenv("ARG_ALERTMANAGER_URL")
	}()
	logPrintf = func(format string, v ...interface{}) {}
	suite.Run(t, s)
}

// GetAlertManagerConfig

func (s *ConfigTestSuite) Test_GetRemoteConfig_ReturnsAlertManagerTarget() {
	expected := `alerting:
  alertmanagers:
  - scheme: http
    static_configs:
    - targets:
      - alert-manager:9093`
	actual := GetAlertManagerConfig()
	s.Equal(expected, actual)
}

// GetRemoteConfig

func (s *ConfigTestSuite) Test_GetRemoteConfig_ReturnsEmptyString_WhenEnvVarsAreNotSet() {
	actual := GetRemoteConfig()

	s.Empty(actual)
}

func (s *ConfigTestSuite) Test_GetRemoteConfig_ReturnsRemoteWriteUrl() {
	defer func() {
		os.Unsetenv("REMOTE_WRITE_URL")
		os.Unsetenv("REMOTE_WRITE_REMOTE_TIMEOUT")
	}()
	os.Setenv("REMOTE_WRITE_URL", "http://acme.com/write")
	os.Setenv("REMOTE_WRITE_REMOTE_TIMEOUT", "30s")
	expected_1 := `remote_write:
  - url: http://acme.com/write
    remote_timeout: 30s`
	expected_2 := `remote_write:
  - remote_timeout: 30s
    url: http://acme.com/write`
	actual := GetRemoteConfig()

	s.Contains([]string{expected_1, expected_2}, actual)
}

func (s *ConfigTestSuite) Test_GetRemoteConfig_ReturnsRemoteReadUrl() {
	defer func() {
		os.Unsetenv("REMOTE_READ_URL")
		os.Unsetenv("REMOTE_READ_REMOTE_TIMEOUT")
	}()
	os.Setenv("REMOTE_READ_URL", "http://acme.com/read")
	os.Setenv("REMOTE_READ_REMOTE_TIMEOUT", "30s")
	expected_1 := `remote_read:
  - url: http://acme.com/read
    remote_timeout: 30s`
	expected_2 := `remote_read:
  - remote_timeout: 30s
    url: http://acme.com/read`
	actual := GetRemoteConfig()

	s.Contains([]string{expected_1, expected_2}, actual)
}

// GetGlobalConfig

func (s *ConfigTestSuite) Test_GlobalConfig_ReturnsConfigWithData() {
	scrapeIntervalOrig := os.Getenv("GLOBAL_SCRAPE_INTERVAL")
	defer func() { os.Setenv("GLOBAL_SCRAPE_INTERVAL", scrapeIntervalOrig) }()
	os.Setenv("GLOBAL_SCRAPE_INTERVAL", "123s")
	expected := `global:
  scrape_interval: 123s`

	actual := GetGlobalConfig()
	s.Equal(expected, actual)
}

func (s *ConfigTestSuite) Test_GlobalConfig_AllowsNestedEntries() {
	scrapeIntervalOrig := os.Getenv("GLOBAL_SCRAPE_INTERVAL")
	defer func() {
		os.Setenv("GLOBAL_SCRAPE_INTERVAL", scrapeIntervalOrig)
		os.Unsetenv("GLOBAL_EXTERNAL_LABELS-CLUSTER")
		os.Unsetenv("GLOBAL_EXTERNAL_LABELS-TYPE")
	}()
	os.Setenv("GLOBAL_SCRAPE_INTERVAL", "123s")
	os.Setenv("GLOBAL_EXTERNAL_LABELS-CLUSTER", "swarm")
	os.Setenv("GLOBAL_EXTERNAL_LABELS-TYPE", "production")
	expected := `global:
  scrape_interval: 123s
  external_labels:
    cluster: swarm
    type: production`
	actual := ""

	// Because of ordering, the config is not always the same so we're repeating a failure for a few times.
	for i := 0; i < 5; i++ {
		actual = GetGlobalConfig()
		if actual == expected {
			return
		}
	}
	s.Equal(expected, actual)
}

// GetScrapeConfig

func (s *ConfigTestSuite) Test_GetScrapeConfig_ReturnsConfigWithData() {
	expected := `scrape_configs:
  - job_name: "service-1"
    metrics_path: /metrics
    dns_sd_configs:
      - names: ["tasks.service-1"]
        type: A
        port: 1234
  - job_name: "service-2"
    metrics_path: /metrics
    dns_sd_configs:
      - names: ["tasks.service-2"]
        type: A
        port: 5678
  - job_name: "service-3"
    metrics_path: /something
    static_configs:
      - targets:
        - service-3:4321`
	scrapes := map[string]Scrape{
		"service-1": {ServiceName: "service-1", ScrapePort: 1234},
		"service-2": {ServiceName: "service-2", ScrapePort: 5678},
		"service-3": {ServiceName: "service-3", ScrapePort: 4321, ScrapeType: "static_configs", MetricsPath: "/something"},
	}

	actual := GetScrapeConfig(scrapes)

	s.Equal(expected, actual)
}

func (s *ConfigTestSuite) Test_GetScrapeConfig_ReturnsConfigWithDataAndSecrets() {
	fsOrig := FS
	defer func() { FS = fsOrig }()
	FS = afero.NewMemMapFs()
	job2 := `  - job_name: "service-2"
    metrics_path: /metrics
    dns_sd_configs:
      - names: ["tasks.service-2"]
        type: A
        port: 5678`
	job3 := `  - job_name: "service-3"
    metrics_path: /metrics
    dns_sd_configs:
      - names: ["tasks.service-3"]
        port: 9999`
	expected := fmt.Sprintf(`scrape_configs:
  - job_name: "service-1"
    metrics_path: /metrics
    dns_sd_configs:
      - names: ["tasks.service-1"]
        type: A
        port: 1234
%s
%s`,
		job2,
		job3,
	)
	scrapes := map[string]Scrape{
		"service-1": {ServiceName: "service-1", ScrapePort: 1234},
	}
	afero.WriteFile(FS, "/run/secrets/scrape_job2", []byte(job2), 0644)
	afero.WriteFile(FS, "/run/secrets/scrape_job3", []byte(job3), 0644)

	actual := GetScrapeConfig(scrapes)

	s.Equal(expected, actual)
}

func (s *ConfigTestSuite) Test_GetScrapeConfig_ReturnsOnlySecretsWithScrapePrefix() {
	fsOrig := FS
	defer func() { FS = fsOrig }()
	FS = afero.NewMemMapFs()
	job := `  - job_name: "my-service"
    dns_sd_configs:
      - names: ["tasks.my-service"]
        type: A
        port: 5678`
	expected := fmt.Sprintf(`scrape_configs:
%s`,
		job,
	)
	scrapes := map[string]Scrape{}
	afero.WriteFile(FS, "/run/secrets/scrape_job", []byte(job), 0644)
	afero.WriteFile(FS, "/run/secrets/job_without_scrape_prefix", []byte("something silly"), 0644)

	actual := GetScrapeConfig(scrapes)

	s.Equal(expected, actual)
}

func (s *ConfigTestSuite) Test_GetScrapeConfig_ReturnsConfigsFromCustomDirectory() {
	fsOrig := FS
	defer func() { FS = fsOrig }()
	FS = afero.NewMemMapFs()
	defer func() { os.Unsetenv("CONFIGS_DIR") }()
	os.Setenv("CONFIGS_DIR", "/tmp")
	job := `  - job_name: "my-service"
    dns_sd_configs:
      - names: ["tasks.my-service"]
        type: A
        port: 5678`
	expected := fmt.Sprintf(`scrape_configs:
%s`,
		job,
	)
	scrapes := map[string]Scrape{}
	afero.WriteFile(FS, "/tmp/scrape_job", []byte(job), 0644)

	actual := GetScrapeConfig(scrapes)

	s.Equal(expected, actual)
}

func (s *ConfigTestSuite) Test_GetScrapeConfig_ReturnsEmptyString_WhenNoData() {
	actual := GetScrapeConfig(map[string]Scrape{})

	s.Empty(actual)
}

// WriteConfig

func (s *ConfigTestSuite) Test_WriteConfig_WritesConfig() {
	fsOrig := FS
	defer func() {
		FS = fsOrig
		os.Unsetenv("REMOTE_WRITE_URL")
		os.Unsetenv("REMOTE_READ_URL")
	}()
	os.Setenv("REMOTE_WRITE_URL", "http://acme.com/write")
	os.Setenv("REMOTE_READ_URL", "http://acme.com/read")
	FS = afero.NewMemMapFs()
	scrapes := map[string]Scrape{
		"service-1": {ServiceName: "service-1", ScrapePort: 1234},
		"service-2": {ServiceName: "service-2", ScrapePort: 5678},
	}
	gc := GetGlobalConfig()
	sc := GetScrapeConfig(scrapes)
	rc := GetRemoteConfig()
	acm := GetAlertManagerConfig()
	expected := fmt.Sprintf(`%s
%s
%s
%s`,
		gc,
		sc,
		rc,
		acm,
	)
	println("000")
	println(rc)
	println("111")

	WriteConfig(scrapes, map[string]Alert{})

	actual, _ := afero.ReadFile(FS, "/etc/prometheus/prometheus.yml")
	s.Equal(expected, string(actual))
	//	s.Fail(string(actual))
}

func (s *ConfigTestSuite) Test_WriteConfig_WritesAlerts() {
	fsOrig := FS
	defer func() { FS = fsOrig }()
	FS = afero.NewMemMapFs()
	alerts := map[string]Alert{}
	alerts["myalert"] = Alert{
		ServiceName:        "my-service",
		AlertName:          "alert-name",
		AlertNameFormatted: "myservicealertname",
		AlertIf:            "a>b",
	}
	gc := GetGlobalConfig()
	acm := GetAlertManagerConfig()
	expectedConfig := fmt.Sprintf(`%s
rule_files:
  - 'alert.rules'
%s`, gc, acm)
	expectedAlerts := GetAlertConfig(alerts)

	WriteConfig(map[string]Scrape{}, alerts)

	actualConfig, _ := afero.ReadFile(FS, "/etc/prometheus/prometheus.yml")
	s.Equal(expectedConfig, string(actualConfig))
	actualAlerts, _ := afero.ReadFile(FS, "/etc/prometheus/alert.rules")
	s.Equal(expectedAlerts, string(actualAlerts))
}
