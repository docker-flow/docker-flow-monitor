package prometheus

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"os"
	"github.com/spf13/afero"
	"fmt"
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

	actual := GetGlobalConfig()
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
`
	scrapes := map[string]Scrape {
		"service-1": Scrape{ ServiceName: "service-1", ScrapePort: 1234 },
		"service-2": Scrape{ ServiceName: "service-2", ScrapePort: 5678 },
	}

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
	defer func() { FS = fsOrig }()
	FS = afero.NewMemMapFs()
	scrapes := map[string]Scrape {
		"service-1": Scrape{ ServiceName: "service-1", ScrapePort: 1234 },
		"service-2": Scrape{ ServiceName: "service-2", ScrapePort: 5678 },
	}
	gc := GetGlobalConfig()
	sc := GetScrapeConfig(scrapes)
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
		ServiceName: "my-service",
		AlertName: "alert-name",
		AlertNameFormatted: "myservicealertname",
		AlertIf: "a>b",
	}
	gc := GetGlobalConfig()
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
