package prometheus

import (
	"fmt"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"
	"os"
	"testing"
)

type ConfigTestSuite struct {
	suite.Suite
}

func (s *ConfigTestSuite) SetupTest() {
}

func TestConfigUnitTestSuite(t *testing.T) {
	s := new(ConfigTestSuite)
	logPrintlnOrig := LogPrintf
	defer func() { LogPrintf = logPrintlnOrig }()
	LogPrintf = func(format string, v ...interface{}) {}
	suite.Run(t, s)
}

// GetGlobalConfig

func (s *ConfigTestSuite) Test_GlobalConfig_ReturnsConfigWithData() {
	scrapeIntervalOrig := os.Getenv("GLOBAL_SCRAPE_INTERVAL")
	defer func() { os.Setenv("GLOBAL_SCRAPE_INTERVAL", scrapeIntervalOrig) }()
	os.Setenv("GLOBAL_SCRAPE_INTERVAL", "123s")
	expected := `
global:
  scrape_interval: 123s`

	actual := getGlobalConfig()
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
	expected := `
global:
  scrape_interval: 123s
  external_labels:
    cluster: swarm
    type: production`
	actual := ""

	// Because of ordering, the config is not always the same so we're repeating a failure for a few times.
	for i := 0; i < 5; i++ {
		actual = getGlobalConfig()
		if actual == expected {
			return
		}
	}
	s.Equal(expected, actual)
}

// GetScrapeConfig

func (s *ConfigTestSuite) Test_GetScrapeConfig_ReturnsConfigWithData() {
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
  - job_name: "service-3"
    static_configs:
      - targets:
        - service-3:4321
`
	scrapes := map[string]Scrape{
		"service-1": {ServiceName: "service-1", ScrapePort: 1234},
		"service-2": {ServiceName: "service-2", ScrapePort: 5678},
		"service-3": {ServiceName: "service-3", ScrapePort: 4321, ScrapeType: "static_configs"},
	}

	actual := getScrapeConfig(scrapes)

	s.Equal(expected, actual)
}

func (s *ConfigTestSuite) Test_GetScrapeConfig_ReturnsConfigWithDataAndSecrets() {
	fsOrig := FS
	defer func() { FS = fsOrig }()
	FS = afero.NewMemMapFs()
	job2 := `  - job_name: "service-2"
    dns_sd_configs:
      - names: ["tasks.service-2"]
        type: A
        port: 5678`
	job3 := `  - job_name: "service-3"
    dns_sd_configs:
      - names: ["tasks.service-3"]
        port: 9999
`
	expected := fmt.Sprintf(`
scrape_configs:
  - job_name: "service-1"
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

	actual := getScrapeConfig(scrapes)

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
	expected := fmt.Sprintf(`
scrape_configs:
%s
`,
		job,
	)
	scrapes := map[string]Scrape{}
	afero.WriteFile(FS, "/run/secrets/scrape_job", []byte(job), 0644)
	afero.WriteFile(FS, "/run/secrets/job_without_scrape_prefix", []byte("something silly"), 0644)

	actual := getScrapeConfig(scrapes)

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
	expected := fmt.Sprintf(`
scrape_configs:
%s
`,
		job,
	)
	scrapes := map[string]Scrape{}
	afero.WriteFile(FS, "/tmp/scrape_job", []byte(job), 0644)

	actual := getScrapeConfig(scrapes)

	s.Equal(expected, actual)
}

func (s *ConfigTestSuite) Test_GetScrapeConfig_ReturnsEmptyString_WhenNoData() {
	actual := getScrapeConfig(map[string]Scrape{})

	s.Empty(actual)
}

// WriteConfig

func (s *ConfigTestSuite) Test_WriteConfig_WritesConfig() {
	fsOrig := FS
	defer func() { FS = fsOrig }()
	FS = afero.NewMemMapFs()
	scrapes := map[string]Scrape{
		"service-1": {ServiceName: "service-1", ScrapePort: 1234},
		"service-2": {ServiceName: "service-2", ScrapePort: 5678},
	}
	gc := getGlobalConfig()
	sc := getScrapeConfig(scrapes)
	expected := fmt.Sprintf(`%s
%s`,
		gc,
		sc,
	)

	WriteConfig(scrapes, map[string]Alert{})

	actual, _ := afero.ReadFile(FS, "/etc/prometheus/prometheus.yml")
	s.Equal(expected, string(actual))
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
	gc := getGlobalConfig()
	expectedConfig := fmt.Sprintf(`%s

rule_files:
  - 'alert.rules'
`,
		gc,
	)
	expectedAlerts := GetAlertConfig(alerts)

	WriteConfig(map[string]Scrape{}, alerts)

	actualConfig, _ := afero.ReadFile(FS, "/etc/prometheus/prometheus.yml")
	s.Equal(expectedConfig, string(actualConfig))
	actualAlerts, _ := afero.ReadFile(FS, "/etc/prometheus/alert.rules")
	s.Equal(expectedAlerts, string(actualAlerts))
}
