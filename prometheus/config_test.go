package prometheus

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/spf13/afero"
	"github.com/stretchr/testify/suite"

	"gopkg.in/yaml.v2"
)

type ConfigTestSuite struct {
	suite.Suite
}

func TestConfigUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (s *ConfigTestSuite) Test_RuleFiles() {
	c := new(Config)
	c.InsertEnv("RULE_FILES_1", "one_rule")
	c.InsertEnv("RULE_FILES_2", "two_rule")

	s.Len(c.RuleFiles, 2)
	s.Equal(c.RuleFiles[0], "one_rule")
	s.Equal(c.RuleFiles[1], "two_rule")
}

func (s *ConfigTestSuite) Test_GlobalConfig() {
	c := new(Config)
	c.InsertEnv("GLOBAL__SCRAPE_INTERVAL", "20s")
	c.InsertEnv("GLOBAL__SCRAPE_TIMEOUT", "10s")
	c.InsertEnv("GLOBAL__EVALUATION_INTERVAL", "15s")
	c.InsertEnv("GLOBAL__EXTERNAL_LABELS", "akey=avalue")
	c.InsertEnv("GLOBAL__EXTERNAL_LABELS", "bkey=bvalue")

	s.Equal("20s", c.GlobalConfig.ScrapeInterval)
	s.Equal("10s", c.GlobalConfig.ScrapeTimeout)
	s.Equal("15s", c.GlobalConfig.EvaluationInterval)
	s.Equal("avalue", c.GlobalConfig.ExternalLabels["akey"])
	s.Equal("bvalue", c.GlobalConfig.ExternalLabels["bkey"])
}

func (s *ConfigTestSuite) Test_AlertConfig() {
	c := new(Config)
	c.InsertEnv("ALERTING__ALERT_RELABEL_CONFIGS_1__SOURCE_LABELS_1", "sourcelabel1")
	c.InsertEnv("ALERTING__ALERT_RELABEL_CONFIGS_1__SOURCE_LABELS_2", "sourcelabel2")
	c.InsertEnv("ALERTING__ALERT_RELABEL_CONFIGS_1__SEPARATOR", ",")
	c.InsertEnv("ALERTING__ALERT_RELABEL_CONFIGS_1__REGEX", "regex")
	c.InsertEnv("ALERTING__ALERT_RELABEL_CONFIGS_1__MODULUS", "10")
	c.InsertEnv("ALERTING__ALERT_RELABEL_CONFIGS_1__TARGET_LABEL", "atarget")
	c.InsertEnv("ALERTING__ALERT_RELABEL_CONFIGS_1__REPLACEMENT", "areplacement")
	c.InsertEnv("ALERTING__ALERT_RELABEL_CONFIGS_1__ACTION", "anaction")

	c.InsertEnv("ALERTING__ALERTMANAGERS_1__SCHEME", "http")
	c.InsertEnv("ALERTING__ALERTMANAGERS_1__PATH_PREFIX", "/hello1")
	c.InsertEnv("ALERTING__ALERTMANAGERS_1__TIMEOUT", "10s")
	c.InsertEnv("ALERTING__ALERTMANAGERS_1__RELABEL_CONFIGS_1__TARGET_LABEL", "what")
	c.InsertEnv("ALERTING__ALERTMANAGERS_1__STATIC_CONFIGS_1__TARGETS_1", "target1")
	c.InsertEnv("ALERTING__ALERTMANAGERS_1__STATIC_CONFIGS_1__TARGETS_2", "target2")
	c.InsertEnv("ALERTING__ALERTMANAGERS_1__STATIC_CONFIGS_1__LABELS", "label1=value1")
	c.InsertEnv("ALERTING__ALERTMANAGERS_1__STATIC_CONFIGS_1__LABELS", "label2=value2")
	c.InsertEnv("ALERTING__ALERTMANAGERS_1__STATIC_CONFIGS_1__SOURCE", "asource")

	c.InsertEnv("ALERTING__ALERTMANAGERS_2__SCHEME", "https")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__PATH_PREFIX", "/hello2")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__TIMEOUT", "15s")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__RELABEL_CONFIGS_1__TARGET_LABEL", "what1")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__RELABEL_CONFIGS_2__TARGET_LABEL", "what2")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__DNS_SD_CONFIGS_1__NAMES_1", "name1")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__DNS_SD_CONFIGS_1__NAMES_2", "name2")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__DNS_SD_CONFIGS_1__REFRESH_INTERVAL", "24s")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__DNS_SD_CONFIGS_1__TYPE", "A")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__DNS_SD_CONFIGS_1__PORT", "1234")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__DNS_SD_CONFIGS_2__NAMES_1", "name3")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__DNS_SD_CONFIGS_2__NAMES_2", "name4")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__DNS_SD_CONFIGS_2__REFRESH_INTERVAL", "29s")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__DNS_SD_CONFIGS_2__TYPE", "A")
	c.InsertEnv("ALERTING__ALERTMANAGERS_2__DNS_SD_CONFIGS_2__PORT", "234")

	s.Require().Len(c.AlertingConfig.AlertRelabelConfigs, 1)
	arc := c.AlertingConfig.AlertRelabelConfigs[0]
	s.Require().Len(arc.SourceLabels, 2)
	s.Equal("sourcelabel1", arc.SourceLabels[0])
	s.Equal("sourcelabel2", arc.SourceLabels[1])
	s.Equal(",", arc.Separator)
	s.Equal("regex", arc.Regex)
	s.Equal(uint64(10), arc.Modulus)
	s.Equal("atarget", arc.TargetLabel)
	s.Equal("areplacement", arc.Replacement)
	s.Equal("anaction", arc.Action)

	s.Require().Len(c.AlertingConfig.AlertmanagerConfigs, 2)
	am1 := c.AlertingConfig.AlertmanagerConfigs[0]
	s.Equal(am1.Scheme, "http")
	s.Equal(am1.PathPrefix, "/hello1")
	s.Equal("10s", am1.Timeout)

	s.Require().Len(am1.RelabelConfigs, 1)
	s.Equal("what", am1.RelabelConfigs[0].TargetLabel)
	s.Require().Len(am1.ServiceDiscoveryConfig.StaticConfigs, 1)
	sc1 := am1.ServiceDiscoveryConfig.StaticConfigs[0]
	s.Require().Len(sc1.Targets, 2)
	s.Equal("target1", sc1.Targets[0])
	s.Equal("target2", sc1.Targets[1])
	s.Equal("value1", sc1.Labels["label1"])
	s.Equal("value2", sc1.Labels["label2"])
	s.Equal("asource", sc1.Source)

	am2 := c.AlertingConfig.AlertmanagerConfigs[1]
	s.Equal("https", am2.Scheme)
	s.Equal("/hello2", am2.PathPrefix)
	s.Equal("15s", am2.Timeout)

	s.Require().Len(am2.RelabelConfigs, 2)
	s.Equal("what1", am2.RelabelConfigs[0].TargetLabel)
	s.Equal("what2", am2.RelabelConfigs[1].TargetLabel)

	s.Require().Len(am2.ServiceDiscoveryConfig.DNSSDConfigs, 2)
	dnsc1 := am2.ServiceDiscoveryConfig.DNSSDConfigs[0]
	s.Require().Len(dnsc1.Names, 2)
	s.Equal("name1", dnsc1.Names[0])
	s.Equal("name2", dnsc1.Names[1])
	s.Equal("24s", dnsc1.RefreshInterval)
	s.Equal("A", dnsc1.Type)
	s.Equal(1234, dnsc1.Port)

	dnsc2 := am2.ServiceDiscoveryConfig.DNSSDConfigs[1]
	s.Require().Len(dnsc2.Names, 2)
	s.Equal("name3", dnsc2.Names[0])
	s.Equal("name4", dnsc2.Names[1])
	s.Equal("29s", dnsc2.RefreshInterval)
	s.Equal("A", dnsc2.Type)
	s.Equal(234, dnsc2.Port)

}

func (s *ConfigTestSuite) Test_ScrapeConfigs() {
	c := &Config{}

	c.InsertEnv("SCRAPE_CONFIGS_1__JOB_NAME", "jobname1")
	c.InsertEnv("SCRAPE_CONFIGS_1__HONOR_LABELS", "true")
	c.InsertEnv("SCRAPE_CONFIGS_1__PARAMS", "key_1=first")
	c.InsertEnv("SCRAPE_CONFIGS_1__PARAMS", "key_2=second")
	c.InsertEnv("SCRAPE_CONFIGS_1__SCRAPE_INTERVAL", "13s")
	c.InsertEnv("SCRAPE_CONFIGS_1__SCRAPE_TIMEOUT", "17s")
	c.InsertEnv("SCRAPE_CONFIGS_1__METRICS_PATH", "/metrics1")
	c.InsertEnv("SCRAPE_CONFIGS_1__SCHEME", "http")
	c.InsertEnv("SCRAPE_CONFIGS_1__SAMPLE_LIMIT", "10")
	c.InsertEnv("SCRAPE_CONFIGS_1__DNS_SD_CONFIGS_1__NAMES_1", "hello")
	c.InsertEnv("SCRAPE_CONFIGS_1__DNS_SD_CONFIGS_1__REFRESH_INTERVAL", "18s")
	c.InsertEnv("SCRAPE_CONFIGS_1__DNS_SD_CONFIGS_1__TYPE", "A")
	c.InsertEnv("SCRAPE_CONFIGS_1__DNS_SD_CONFIGS_1__PORT", "123")
	c.InsertEnv("SCRAPE_CONFIGS_1__BASIC_AUTH__USERNAME", "username1")
	c.InsertEnv("SCRAPE_CONFIGS_1__BASIC_AUTH__PASSWORD", "password1")
	c.InsertEnv("SCRAPE_CONFIGS_1__BEARER_TOKEN", "btoken")
	c.InsertEnv("SCRAPE_CONFIGS_1__PROXY_URL", "localhost1")
	c.InsertEnv("SCRAPE_CONFIGS_1__TLS_CONFIG__CA_FILE", "cafile")
	c.InsertEnv("SCRAPE_CONFIGS_1__TLS_CONFIG__CERT_FILE", "certfile")
	c.InsertEnv("SCRAPE_CONFIGS_1__TLS_CONFIG__KEY_FILE", "keyfile")
	c.InsertEnv("SCRAPE_CONFIGS_1__TLS_CONFIG__SERVER_NAME", "servicename")
	c.InsertEnv("SCRAPE_CONFIGS_1__TLS_CONFIG__INSECURE_SKIP_VERIFY", "true")
	c.InsertEnv("SCRAPE_CONFIGS_1__RELABEL_CONFIGS_1__TARGET_LABEL", "target1")
	c.InsertEnv("SCRAPE_CONFIGS_1__RELABEL_CONFIGS_2__TARGET_LABEL", "target2")
	c.InsertEnv("SCRAPE_CONFIGS_1__METRIC_RELABEL_CONFIGS_1__SOURCE_LABELS_1", "label1")
	c.InsertEnv("SCRAPE_CONFIGS_1__METRIC_RELABEL_CONFIGS_1__SOURCE_LABELS_2", "label2")

	c.InsertEnv("SCRAPE_CONFIGS_2__JOB_NAME", "jobname2")
	c.InsertEnv("SCRAPE_CONFIGS_2__DNS_SD_CONFIGS_1__NAMES_1", "hello2")
	c.InsertEnv("SCRAPE_CONFIGS_2__DNS_SD_CONFIGS_1__TYPE", "A")
	c.InsertEnv("SCRAPE_CONFIGS_2__DNS_SD_CONFIGS_1__PORT", "1233")

	s.Require().Len(c.ScrapeConfigs, 2)
	s1 := c.ScrapeConfigs[0]
	s.Equal("jobname1", s1.JobName)
	s.Equal(true, s1.HonorLabels)

	s.Require().Len(s1.Params["key"], 2)
	s.Equal("first", s1.Params["key"][0])
	s.Equal("second", s1.Params["key"][1])
	s.Equal("13s", s1.ScrapeInterval)
	s.Equal("17s", s1.ScrapeTimeout)
	s.Equal("/metrics1", s1.MetricsPath)
	s.Equal("http", s1.Scheme)
	s.Equal(uint(10), s1.SampleLimit)

	s.Require().Len(s1.ServiceDiscoveryConfig.DNSSDConfigs, 1)
	s1dnsc := s1.ServiceDiscoveryConfig.DNSSDConfigs[0]
	s.Equal("hello", s1dnsc.Names[0])
	s.Equal("18s", s1dnsc.RefreshInterval)
	s.Equal("A", s1dnsc.Type)
	s.Equal(123, s1dnsc.Port)

	s1hcc := s1.HTTPClientConfig
	s.Equal("username1", s1hcc.BasicAuth.Username)
	s.Equal("password1", s1hcc.BasicAuth.Password)
	s.Equal("btoken", s1hcc.BearerToken)
	s.Equal("localhost1", s1hcc.ProxyURL)
	s.Equal("cafile", s1hcc.TLSConfig.CAFile)
	s.Equal("certfile", s1hcc.TLSConfig.CertFile)
	s.Equal("keyfile", s1hcc.TLSConfig.KeyFile)
	s.Equal("servicename", s1hcc.TLSConfig.ServerName)
	s.Equal(true, s1hcc.TLSConfig.InsecureSkipVerify)

	s.Require().Len(s1.RelabelConfigs, 2)
	s.Equal("target1", s1.RelabelConfigs[0].TargetLabel)
	s.Equal("target2", s1.RelabelConfigs[1].TargetLabel)

	s.Require().Len(s1.MetricRelabelConfigs, 1)
	s1sl := s1.MetricRelabelConfigs[0].SourceLabels
	s.Require().Len(s1sl, 2)
	s.Equal("label1", s1sl[0])
	s.Equal("label2", s1sl[1])

	s2 := c.ScrapeConfigs[1]
	s.Equal("jobname2", s2.JobName)

	s2dnsc := s2.ServiceDiscoveryConfig.DNSSDConfigs
	s.Require().Len(s2dnsc, 1)
	s.Equal("hello2", s2dnsc[0].Names[0])
	s.Equal("A", s2dnsc[0].Type)
	s.Equal(1233, s2dnsc[0].Port)
}

func (s *ConfigTestSuite) Test_RemoteReadConfigs() {
	c := &Config{}

	c.InsertEnv("REMOTE_READ_1__URL", "localhost")
	c.InsertEnv("REMOTE_READ_1__REMOTE_TIMEOUT", "10s")
	c.InsertEnv("REMOTE_READ_1__READ_RECENT", "true")
	c.InsertEnv("REMOTE_READ_1__BASIC_AUTH__USERNAME", "username")
	c.InsertEnv("REMOTE_READ_1__BASIC_AUTH__PASSWORD", "password")
	c.InsertEnv("REMOTE_READ_1__REQUIRED_MATCHERS", "key1=value1")
	c.InsertEnv("REMOTE_READ_1__REQUIRED_MATCHERS", "key2=value2")

	c.InsertEnv("REMOTE_READ_2__URL", "localhost2")

	s.Require().Len(c.RemoteReadConfigs, 2)
	rrc := c.RemoteReadConfigs[0]
	s.Equal("localhost", rrc.URL)
	s.Equal("10s", rrc.RemoteTimeout)
	s.Equal("username", rrc.HTTPClientConfig.BasicAuth.Username)
	s.Equal("password", rrc.HTTPClientConfig.BasicAuth.Password)
	s.Equal("value1", rrc.RequiredMatchers["key1"])
	s.Equal("value2", rrc.RequiredMatchers["key2"])

	s.Equal("localhost2", c.RemoteReadConfigs[1].URL)
}

func (s *ConfigTestSuite) Test_RemoteWriteConfigs() {
	c := &Config{}

	c.InsertEnv("REMOTE_WRITE_1__URL", "localhost")
	c.InsertEnv("REMOTE_WRITE_1__REMOTE_TIMEOUT", "14s")
	c.InsertEnv("REMOTE_WRITE_1__WRITE_RELABEL_CONFIGS_1__SOURCE_LABELS_1", "label1")
	c.InsertEnv("REMOTE_WRITE_1__WRITE_RELABEL_CONFIGS_1__SOURCE_LABELS_2", "label2")
	c.InsertEnv("REMOTE_WRITE_1__PROXY_URL", "proxy_url")

	c.InsertEnv("REMOTE_WRITE_2__URL", "localhost2")
	c.InsertEnv("REMOTE_WRITE_2__REMOTE_TIMEOUT", "20s")
	c.InsertEnv("REMOTE_WRITE_2__QUEUE_CONFIG__CAPACITY", "10")
	c.InsertEnv("REMOTE_WRITE_2__QUEUE_CONFIG__MAX_SHARDS", "2")
	c.InsertEnv("REMOTE_WRITE_2__QUEUE_CONFIG__MAX_SAMPLES_PER_SEND", "4")
	c.InsertEnv("REMOTE_WRITE_2__QUEUE_CONFIG__BATCH_SEND_DEADLINE", "10s")
	c.InsertEnv("REMOTE_WRITE_2__QUEUE_CONFIG__MAX_RETRIES", "5")
	c.InsertEnv("REMOTE_WRITE_2__QUEUE_CONFIG__MIN_BACKOFF", "5s")
	c.InsertEnv("REMOTE_WRITE_2__QUEUE_CONFIG__MAX_BACKOFF", "6s")

	s.Require().Len(c.RemoteWriteConfigs, 2)
	rcc1 := c.RemoteWriteConfigs[0]
	s.Equal("localhost", rcc1.URL)
	s.Equal("14s", rcc1.RemoteTimeout)

	rcc1Wrc := rcc1.WriteRelabelConfigs
	s.Require().Len(rcc1Wrc, 1)
	s.Equal("label1", rcc1Wrc[0].SourceLabels[0])
	s.Equal("label2", rcc1Wrc[0].SourceLabels[1])
	s.Equal("proxy_url", rcc1.HTTPClientConfig.ProxyURL)

	rcc2 := c.RemoteWriteConfigs[1]
	s.Equal("localhost2", rcc2.URL)
	s.Equal("20s", rcc2.RemoteTimeout)

	rcc2qc := rcc2.QueueConfig
	s.Equal(10, rcc2qc.Capacity)
	s.Equal(2, rcc2qc.MaxShards)
	s.Equal(4, rcc2qc.MaxSamplesPerSend)
	s.Equal("10s", rcc2qc.BatchSendDeadline)
	s.Equal(5, rcc2qc.MaxRetries)
	s.Equal("5s", rcc2qc.MinBackoff)
	s.Equal("6s", rcc2qc.MaxBackoff)

}

func (s *ConfigTestSuite) Test_BackwardsEnvVars() {
	c := &Config{}

	c.InsertEnv("REMOTE_WRITE_URL", "http://acme.com/write")
	c.InsertEnv("REMOTE_WRITE_REMOTE_TIMEOUT", "30s")
	c.InsertEnv("REMOTE_READ_URL", "http://acme.com/read")
	c.InsertEnv("REMOTE_READ_REMOTE_TIMEOUT", "30s")
	c.InsertEnv("GLOBAL_SCRAPE_INTERVAL", "123s")
	c.InsertEnv("GLOBAL_EXTERNAL_LABELS-CLUSTER", "swarm")
	c.InsertEnv("GLOBAL_EXTERNAL_LABELS-TYPE", "production")

	s.Equal("http://acme.com/write", c.RemoteWriteConfigs[0].URL)
	s.Equal("30s", c.RemoteWriteConfigs[0].RemoteTimeout)
	s.Equal("http://acme.com/read", c.RemoteReadConfigs[0].URL)
	s.Equal("30s", c.RemoteReadConfigs[0].RemoteTimeout)
	s.Equal("123s", c.GlobalConfig.ScrapeInterval)
	s.Equal("swarm", c.GlobalConfig.ExternalLabels["cluster"])
	s.Equal("production", c.GlobalConfig.ExternalLabels["type"])

	cYAML, err := yaml.Marshal(c)
	s.Require().NoError(err)

	expected := `global:
  scrape_interval: 123s
  external_labels:
    cluster: swarm
    type: production
remote_write:
- url: http://acme.com/write
  remote_timeout: 30s
remote_read:
- url: http://acme.com/read
  remote_timeout: 30s
`

	s.Equal(expected, string(cYAML))

}

func (s *ConfigTestSuite) Test_InsertAlertManagerURL() {
	c := &Config{}

	err := c.InsertAlertManagerURL("http://alert-manager:9093")
	s.Require().NoError(err)

	s.Require().Len(c.AlertingConfig.AlertmanagerConfigs, 1)
	acc := c.AlertingConfig.AlertmanagerConfigs[0]
	s.Equal("http", acc.Scheme)

	s.Require().Len(acc.ServiceDiscoveryConfig.StaticConfigs, 1)
	sc := acc.ServiceDiscoveryConfig.StaticConfigs[0]
	s.Equal("alert-manager:9093", sc.Targets[0])

	cYAML, err := yaml.Marshal(c)
	s.Require().NoError(err)

	expected := `alerting:
  alertmanagers:
  - static_configs:
    - targets:
      - alert-manager:9093
    scheme: http
`

	s.Equal(expected, string(cYAML))

}

func (s *ConfigTestSuite) Test_InsertScrape_ConfigWithData() {

	scrapes := map[string]Scrape{
		"service-1": {ServiceName: "service-1", ScrapePort: 1234},
		"service-2": {
			ServiceName:    "service-2",
			ScrapePort:     5678,
			ScrapeInterval: "32s",
			ScrapeTimeout:  "11s",
		},
		"service-3": {
			ServiceName:    "service-3",
			ScrapeInterval: "23s",
			ScrapeTimeout:  "21s",
			ScrapePort:     4321,
			ScrapeType:     "static_configs",
			MetricsPath:    "/something",
		},
	}

	c := &Config{}
	c.InsertScrapes(scrapes)
	s.Require().Len(c.ScrapeConfigs, 3)

	for _, sc := range c.ScrapeConfigs {
		expectedC := scrapes[sc.JobName]
		s.Equal(expectedC.ServiceName, sc.JobName)
		if expectedC.ScrapeType != "static_configs" {
			s.Equal(expectedC.ScrapePort, sc.ServiceDiscoveryConfig.DNSSDConfigs[0].Port)
			s.Equal("A", sc.ServiceDiscoveryConfig.DNSSDConfigs[0].Type)
			s.Equal("/metrics", sc.MetricsPath)
			s.Equal(
				fmt.Sprintf("tasks.%s", expectedC.ServiceName),
				sc.ServiceDiscoveryConfig.DNSSDConfigs[0].Names[0],
			)
			s.Equal(expectedC.ScrapeInterval, sc.ScrapeInterval)
			s.Equal(expectedC.ScrapeTimeout, sc.ScrapeTimeout)
		} else {
			s.Equal(expectedC.ServiceName, sc.JobName)
			s.Equal(expectedC.MetricsPath, sc.MetricsPath)
			s.Equal(
				fmt.Sprintf("%s:%d", expectedC.ServiceName, expectedC.ScrapePort),
				sc.ServiceDiscoveryConfig.StaticConfigs[0].Targets[0],
			)
			s.Equal(expectedC.ScrapeInterval, sc.ScrapeInterval)
			s.Equal(expectedC.ScrapeTimeout, sc.ScrapeTimeout)
		}
	}
}

func (s *ConfigTestSuite) Test_InsertScrape_ConfigWithDataAndSecrets() {
	fsOrig := FS
	defer func() { FS = fsOrig }()
	FS = afero.NewMemMapFs()

	// Old style scrape secret file
	job2 := `  - job_name: "service-2"
    metrics_path: /metrics
    dns_sd_configs:
      - names: ["tasks.service-2"]
        type: A
        port: 5678`

	// New style scrape secret file
	job3 := `- job_name: "service-3"
  metrics_path: /metrics
  dns_sd_configs:
    - names: ["tasks.service-3"]
      port: 9999`
	scrapes := map[string]Scrape{
		"service-1": {ServiceName: "service-1", ScrapePort: 1234},
	}
	afero.WriteFile(FS, "/run/secrets/scrape_job2", []byte(job2), 0644)
	afero.WriteFile(FS, "/run/secrets/scrape_job3", []byte(job3), 0644)

	c := &Config{}
	c.InsertScrapes(scrapes)
	c.InsertScrapesFromDir("/run/secrets")

	s.Require().Len(c.ScrapeConfigs, 3)

	s1 := c.ScrapeConfigs[0]
	s.Equal("service-1", s1.JobName)
	s.Equal(1234, s1.ServiceDiscoveryConfig.DNSSDConfigs[0].Port)
	s.Equal("A", s1.ServiceDiscoveryConfig.DNSSDConfigs[0].Type)
	s.Equal("/metrics", s1.MetricsPath)
	s.Equal("tasks.service-1", s1.ServiceDiscoveryConfig.DNSSDConfigs[0].Names[0])

	s2 := c.ScrapeConfigs[1]
	s.Equal("service-2", s2.JobName)
	s.Equal(5678, s2.ServiceDiscoveryConfig.DNSSDConfigs[0].Port)
	s.Equal("A", s2.ServiceDiscoveryConfig.DNSSDConfigs[0].Type)
	s.Equal("/metrics", s2.MetricsPath)
	s.Equal("tasks.service-2", s2.ServiceDiscoveryConfig.DNSSDConfigs[0].Names[0])

	s3 := c.ScrapeConfigs[2]
	s.Equal("service-3", s3.JobName)
	s.Equal(9999, s3.ServiceDiscoveryConfig.DNSSDConfigs[0].Port)
	s.Equal("/metrics", s3.MetricsPath)
	s.Equal("tasks.service-3", s3.ServiceDiscoveryConfig.DNSSDConfigs[0].Names[0])
}

func (s *ConfigTestSuite) Test_InsertScrape_ConfigWithOnlySecrets_WithoutScrapePrefix() {
	fsOrig := FS
	defer func() { FS = fsOrig }()
	FS = afero.NewMemMapFs()
	job := `- job_name: "my-service"
  dns_sd_configs:
    - names: ["tasks.my-service"]
      type: A
      port: 5678`

	scrapes := map[string]Scrape{}
	afero.WriteFile(FS, "/run/secrets/scrape_job", []byte(job), 0644)
	afero.WriteFile(FS, "/run/secrets/job_without_scrape_prefix", []byte("something silly"), 0644)

	c := &Config{}
	c.InsertScrapes(scrapes)
	c.InsertScrapesFromDir("/run/secrets")

	s.Require().Len(c.ScrapeConfigs, 1)

	s1 := c.ScrapeConfigs[0]
	s.Equal("my-service", s1.JobName)
	s.Equal(5678, s1.ServiceDiscoveryConfig.DNSSDConfigs[0].Port)
	s.Equal("A", s1.ServiceDiscoveryConfig.DNSSDConfigs[0].Type)
	s.Equal("tasks.my-service", s1.ServiceDiscoveryConfig.DNSSDConfigs[0].Names[0])
}

func (s *ConfigTestSuite) Test_InsertScrape_ConfiFromCustomDirectory() {
	fsOrig := FS
	defer func() { FS = fsOrig }()
	FS = afero.NewMemMapFs()
	job := `- job_name: "my-service"
  dns_sd_configs:
    - names: ["tasks.my-service"]
      type: A
      port: 5678`
	afero.WriteFile(FS, "/tmp/scrape_job", []byte(job), 0644)

	c := &Config{}
	c.InsertScrapesFromDir("/tmp")

	s.Require().Len(c.ScrapeConfigs, 1)

	s1 := c.ScrapeConfigs[0]
	s.Equal("my-service", s1.JobName)
	s.Equal(5678, s1.ServiceDiscoveryConfig.DNSSDConfigs[0].Port)
	s.Equal("A", s1.ServiceDiscoveryConfig.DNSSDConfigs[0].Type)
	s.Equal("tasks.my-service", s1.ServiceDiscoveryConfig.DNSSDConfigs[0].Names[0])

}

func (s *ConfigTestSuite) Test_Writeconfig_WritesConfig() {
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
	alerts := map[string]Alert{}

	c := Config{}
	c.InsertEnv("REMOTE_WRITE_URL", "http://acme.com/write")
	c.InsertEnv("REMOTE_READ_URL", "http://acme.com/read")
	c.InsertScrapes(scrapes)

	nodeLabels := map[string]map[string]string{}
	WriteConfig("/etc/prometheus/prometheus.yml", scrapes, alerts, nodeLabels)
	actual, _ := afero.ReadFile(FS, "/etc/prometheus/prometheus.yml")

	actualConfig := Config{}
	err := yaml.Unmarshal(actual, &actualConfig)
	s.Require().NoError(err)

	s.Equal(c.RemoteReadConfigs[0].URL, actualConfig.RemoteReadConfigs[0].URL)
	s.Equal(c.RemoteWriteConfigs[0].URL, actualConfig.RemoteWriteConfigs[0].URL)

	// Order of scrapes can be different
	s.Contains(actualConfig.ScrapeConfigs, c.ScrapeConfigs[0])
	s.Contains(actualConfig.ScrapeConfigs, c.ScrapeConfigs[1])
}

func (s *ConfigTestSuite) Test_Writeconfig_WithNodeInfoAndNodes_WritesConfig() {
	fsOrg := FS
	defer func() {
		FS = fsOrg
	}()
	FS = afero.NewMemMapFs()

	nodeInfo1 := NodeIPSet{}
	nodeInfo1.Add("node-1", "1.0.1.1", "nodeid1")
	nodeInfo1.Add("node-2", "1.0.1.2", "nodeid2")
	serviceLabels1 := map[string]string{
		"env":    "prod",
		"domain": "frontend",
	}

	scrapes := map[string]Scrape{
		"service-1": {
			ServiceName:  "service-1",
			ScrapePort:   1234,
			ScrapeLabels: &serviceLabels1,
			NodeInfo:     nodeInfo1,
		},
	}
	alerts := map[string]Alert{}

	nodeLabels := map[string]map[string]string{
		"nodeid1": map[string]string{
			"awsregion": "us-east",
			"role":      "manager",
		},
		"nodeid2": map[string]string{
			"awsregion": "us-west",
			"role":      "worker",
		},
	}
	WriteConfig("/etc/prometheus/prometheus.yml", scrapes, alerts, nodeLabels)
	actual, err := afero.ReadFile(FS, "/etc/prometheus/prometheus.yml")
	s.Require().NoError(err)

	actualConfig := Config{}
	err = yaml.Unmarshal(actual, &actualConfig)
	s.Require().NoError(err)

	s.Require().Len(actualConfig.ScrapeConfigs, 1)

	var service1ScrapeConfig *ScrapeConfig

	for _, sc := range actualConfig.ScrapeConfigs {
		if sc.JobName == "service-1" {
			service1ScrapeConfig = sc
		}
	}
	s.Require().NotNil(service1ScrapeConfig)

	s.Require().Len(service1ScrapeConfig.ServiceDiscoveryConfig.FileSDConfigs, 1)

	service1FileScrape := service1ScrapeConfig.ServiceDiscoveryConfig.FileSDConfigs[0]

	s.Equal("/etc/prometheus/file_sd/service-1.json", service1FileScrape.Files[0])

	actualSDService1Bytes, err := afero.ReadFile(FS, "/etc/prometheus/file_sd/service-1.json")
	s.Require().NoError(err)
	fsc1 := FileStaticConfig{}
	err = json.Unmarshal(actualSDService1Bytes, &fsc1)
	s.Require().NoError(err)

	var tgService1Node1 *TargetGroup
	var tgService1Node2 *TargetGroup

	for _, tg := range fsc1 {
		for _, target := range tg.Targets {
			if target == "1.0.1.1:1234" {
				tgService1Node1 = tg
				break
			} else if target == "1.0.1.2:1234" {
				tgService1Node2 = tg
				break
			}
		}
	}
	s.Require().NotNil(tgService1Node1)
	s.Require().NotNil(tgService1Node2)

	s.Equal("prod", tgService1Node1.Labels["env"])
	s.Equal("frontend", tgService1Node1.Labels["domain"])
	s.Equal("service-1", tgService1Node1.Labels["service"])
	s.Equal("us-east", tgService1Node1.Labels["awsregion"])
	s.Equal("manager", tgService1Node1.Labels["role"])

	s.Equal("prod", tgService1Node2.Labels["env"])
	s.Equal("frontend", tgService1Node2.Labels["domain"])
	s.Equal("service-1", tgService1Node2.Labels["service"])
	s.Equal("us-west", tgService1Node2.Labels["awsregion"])
	s.Equal("worker", tgService1Node2.Labels["role"])
}

func (s *ConfigTestSuite) Test_Writeconfig_WithNodeInfo_WritesConfig() {
	fsOrg := FS
	defer func() {
		FS = fsOrg
	}()
	FS = afero.NewMemMapFs()

	nodeInfo1 := NodeIPSet{}
	nodeInfo1.Add("node-1", "1.0.1.1", "id1")
	nodeInfo1.Add("node-2", "1.0.1.2", "id2")
	serviceLabels1 := map[string]string{
		"env":    "prod",
		"domain": "frontend",
	}

	nodeInfo2 := NodeIPSet{}
	nodeInfo2.Add("node-1", "1.0.2.1", "id1")
	nodeInfo2.Add("node-1", "1.0.2.2", "id1")
	serviceLabels2 := map[string]string{
		"env":    "dev",
		"domain": "backend",
	}

	scrapes := map[string]Scrape{
		"service-1": {
			ServiceName:  "service-1",
			ScrapePort:   1234,
			ScrapeLabels: &serviceLabels1,
			NodeInfo:     nodeInfo1,
		},
		"service-2": {
			ServiceName:  "service-2",
			ScrapePort:   5678,
			ScrapeLabels: &serviceLabels2,
			NodeInfo:     nodeInfo2,
		},
		"service-3": {
			ServiceName: "service-3",
			ScrapePort:  5432,
		},
	}
	alerts := map[string]Alert{}

	nodeLabels := map[string]map[string]string{}
	WriteConfig("/etc/prometheus/prometheus.yml", scrapes, alerts, nodeLabels)
	actual, err := afero.ReadFile(FS, "/etc/prometheus/prometheus.yml")
	s.Require().NoError(err)

	actualConfig := Config{}
	err = yaml.Unmarshal(actual, &actualConfig)
	s.Require().NoError(err)

	s.Require().Len(actualConfig.ScrapeConfigs, 3)

	var service1ScrapeConfig *ScrapeConfig
	var service2ScrapeConfig *ScrapeConfig
	var service3ScrapeConfig *ScrapeConfig

	for _, sc := range actualConfig.ScrapeConfigs {
		if sc.JobName == "service-1" {
			service1ScrapeConfig = sc
		} else if sc.JobName == "service-2" {
			service2ScrapeConfig = sc
		} else if sc.JobName == "service-3" {
			service3ScrapeConfig = sc
		}
	}
	s.Require().NotNil(service1ScrapeConfig)
	s.Require().NotNil(service2ScrapeConfig)
	s.Require().NotNil(service3ScrapeConfig)

	s.Require().Len(service1ScrapeConfig.ServiceDiscoveryConfig.FileSDConfigs, 1)
	s.Require().Len(service2ScrapeConfig.ServiceDiscoveryConfig.FileSDConfigs, 1)
	s.Require().Len(service3ScrapeConfig.ServiceDiscoveryConfig.DNSSDConfigs, 1)

	service1FileScrape := service1ScrapeConfig.ServiceDiscoveryConfig.FileSDConfigs[0]
	service2FileScrape := service2ScrapeConfig.ServiceDiscoveryConfig.FileSDConfigs[0]
	service3DNSScrape := service3ScrapeConfig.ServiceDiscoveryConfig.DNSSDConfigs[0]

	s.Equal("/etc/prometheus/file_sd/service-1.json", service1FileScrape.Files[0])
	s.Equal("/etc/prometheus/file_sd/service-2.json", service2FileScrape.Files[0])

	s.Require().Len(service3DNSScrape.Names, 1)
	s.Equal("tasks.service-3", service3DNSScrape.Names[0])
	s.Equal(5432, service3DNSScrape.Port)
	s.Equal("A", service3DNSScrape.Type)

	actualSDService1Bytes, err := afero.ReadFile(FS, "/etc/prometheus/file_sd/service-1.json")
	s.Require().NoError(err)
	fsc1 := FileStaticConfig{}
	err = json.Unmarshal(actualSDService1Bytes, &fsc1)
	s.Require().NoError(err)

	actualSDService2Bytes, err := afero.ReadFile(FS, "/etc/prometheus/file_sd/service-2.json")
	s.Require().NoError(err)
	fsc2 := FileStaticConfig{}
	err = json.Unmarshal(actualSDService2Bytes, &fsc2)
	s.Require().NoError(err)

	var tgService1Node1 *TargetGroup
	var tgService1Node2 *TargetGroup
	var tgService2Node1 *TargetGroup
	var tgService2Node2 *TargetGroup

	for _, tg := range fsc1 {
		for _, target := range tg.Targets {
			if target == "1.0.1.1:1234" {
				tgService1Node1 = tg
				break
			} else if target == "1.0.1.2:1234" {
				tgService1Node2 = tg
				break
			}
		}
	}
	for _, tg := range fsc2 {
		for _, target := range tg.Targets {
			if target == "1.0.2.1:5678" {
				tgService2Node1 = tg
				break
			} else if target == "1.0.2.2:5678" {
				tgService2Node2 = tg
			}
		}
	}
	s.Require().NotNil(tgService1Node1)
	s.Require().NotNil(tgService1Node2)
	s.Require().NotNil(tgService2Node1)
	s.Require().NotNil(tgService2Node2)

	s.Equal("prod", tgService1Node1.Labels["env"])
	s.Equal("frontend", tgService1Node1.Labels["domain"])
	s.Equal("service-1", tgService1Node1.Labels["service"])

	s.Equal("prod", tgService1Node2.Labels["env"])
	s.Equal("frontend", tgService1Node2.Labels["domain"])
	s.Equal("service-1", tgService1Node2.Labels["service"])

	s.Equal("dev", tgService2Node1.Labels["env"])
	s.Equal("backend", tgService2Node1.Labels["domain"])
	s.Equal("service-2", tgService2Node1.Labels["service"])

	s.Equal("dev", tgService2Node2.Labels["env"])
	s.Equal("backend", tgService2Node2.Labels["domain"])
	s.Equal("service-2", tgService2Node2.Labels["service"])
}
func (s *ConfigTestSuite) Test_WriteConfig_WriteAlerts() {
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

	c := &Config{}
	c.RuleFiles = []string{"alert.rules"}
	cYAML, _ := yaml.Marshal(c)
	expectedAlerts := GetAlertConfig(alerts)

	nodeLabels := map[string]map[string]string{}
	WriteConfig("/etc/prometheus/prometheus.yml", map[string]Scrape{}, alerts, nodeLabels)

	actualConfig, _ := afero.ReadFile(FS, "/etc/prometheus/prometheus.yml")
	s.Equal(cYAML, actualConfig)
	actualAlerts, _ := afero.ReadFile(FS, "/etc/prometheus/alert.rules")
	s.Equal(expectedAlerts, string(actualAlerts))
}
