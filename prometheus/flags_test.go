package prometheus

import (
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type FlagsTestSuite struct {
	suite.Suite
}

func TestFlagsUnitTestSuite(t *testing.T) {
	suite.Run(t, new(FlagsTestSuite))
}

func (s *FlagsTestSuite) Test_FlagsPrometheusV1() {
	envMap := []struct {
		key      string
		value    string
		expected string
	}{
		{"ARG_CONFIG_FILE", "/etc/prometheus/prometheus.yml",
			"--config.file=\"/etc/prometheus/prometheus.yml\""},
		{"ARG_WEB_LISTEN-ADDRESS", "0.0.0.0:9090",
			"--web.listen-address=\"0.0.0.0:9090\""},
		{"ARG_WEB_READ-TIMEOUT", "5m",
			"--web.read-timeout=\"5m\""},
		{"ARG_WEB_MAX-CONNECTIONS", "512",
			"--web.max-connections=\"512\""},
		{"ARG_WEB_EXTERNAL-URL", "/something",
			"--web.external-url=\"/something\""},
		{"ARG_WEB_ROUTE-PREFIX", "/monitor",
			"--web.route-prefix=\"/monitor\""},
		{"ARG_WEB_USER-ASSETS", "/assets",
			"--web.user-assets=\"/assets\""},
		{"ARG_WEB_ENABLE-REMOTE-SHUTDOWN", "true",
			"--web.enable-lifecycle"},
		{"ARG_WEB_CONSOLE_TEMPLATES", "consoles",
			"--web.console.templates=\"consoles\""},
		{"ARG_WEB_CONSOLE_LIBRARIES", "console_libraries",
			"--web.console.libraries=\"console_libraries\""},
		{"ARG_STORAGE_LOCAL_PATH", "/data",
			"--storage.tsdb.path=\"/data\""},
		{"ARG_STORAGE_LOCAL_RETENTION", "15d",
			"--storage.tsdb.retention=\"15d\""},
		{"ARG_ALERTMANAGER_NOTIFICATION-QUEUE-CAPACITY", "10000",
			"--alertmanager.notification-queue-capacity=\"10000\""},
		{"ARG_ALERTMANAGER_TIMEOUT", "10s",
			"--alertmanager.timeout=\"10s\""},
		{"ARG_QUERY_STALENESS-DELTA", "5m",
			"--query.lookback-delta=\"5m\""},
		{"ARG_QUERY_TIMEOUT", "2m",
			"--query.timeout=\"2m\""},
		{"ARG_QUERY_MAX-CONCURRENCY", "20",
			"--query.max-concurrency=\"20\""},
		{"ARG_LOG_LEVEL", "info",
			"--log.level=\"info\""},
	}
	os.Setenv("ARG_ALERTMANAGER_URL", "http://alert-manager:9093")
	defer func() {
		for _, envItem := range envMap {
			os.Unsetenv(envItem.key)
		}
		os.Unsetenv("ARG_ALERTMANAGER_URL")
	}()
	for _, envItem := range envMap {
		os.Setenv(envItem.key, envItem.value)
	}

	actFlags := EnvToPrometheusFlags("ARG")
	for _, envItem := range envMap {
		s.Contains(actFlags, envItem.expected)
	}
	s.NotContains(actFlags, "--alertmanager.url=\"http://alert-manager:9093\"")
}

func (s *FlagsTestSuite) Test_FlagsPrometheusV2() {
	envMap := []struct {
		key      string
		value    string
		expected string
	}{
		{"ARG_CONFIG_FILE", "/etc/prometheus/prometheus.yml",
			"--config.file=\"/etc/prometheus/prometheus.yml\""},
		{"ARG_WEB_LISTEN-ADDRESS", "0.0.0.0:9090",
			"--web.listen-address=\"0.0.0.0:9090\""},
		{"ARG_WEB_READ-TIMEOUT", "5m",
			"--web.read-timeout=\"5m\""},
		{"ARG_WEB_MAX-CONNECTIONS", "512",
			"--web.max-connections=\"512\""},
		{"ARG_WEB_EXTERNAL-URL", "/something",
			"--web.external-url=\"/something\""},
		{"ARG_WEB_ROUTE-PREFIX", "/monitor",
			"--web.route-prefix=\"/monitor\""},
		{"ARG_WEB_USER-ASSETS", "/assets",
			"--web.user-assets=\"/assets\""},
		{"ARG_WEB_ENABLE-LIFECYCLE", "",
			"--web.enable-lifecycle"},
		{"ARG_WEB_ENABLE-ADMIN-API", "",
			"--web.enable-admin-api"},
		{"ARG_WEB_CONSOLE_TEMPLATES", "consoles",
			"--web.console.templates=\"consoles\""},
		{"ARG_WEB_CONSOLE_LIBRARIES", "console_libraries",
			"--web.console.libraries=\"console_libraries\""},
		{"ARG_STORAGE_TSDB_PATH", "/data",
			"--storage.tsdb.path=\"/data\""},
		{"ARG_STORAGE_TSDB_MIN-BLOCK-DURATION", "2h",
			"--storage.tsdb.min-block-duration=\"2h\""},
		{"ARG_STORAGE_TSDB_MAX-BLOCK-DURATION", "1d",
			"--storage.tsdb.max-block-duration=\"1d\""},
		{"ARG_STORAGE_TSDB_RETENTION", "15d",
			"--storage.tsdb.retention=\"15d\""},
		{"ARG_STORAGE_TSDB_NO-LOCKFILE", "",
			"--storage.tsdb.no-lockfile"},
		{"ARG_ALERTMANAGER_NOTIFICATION-QUEUE-CAPACITY", "10000",
			"--alertmanager.notification-queue-capacity=\"10000\""},
		{"ARG_ALERTMANAGER_TIMEOUT", "10s",
			"--alertmanager.timeout=\"10s\""},
		{"ARG_QUERY_STALENESS-DELTA", "5m",
			"--query.lookback-delta=\"5m\""},
		{"ARG_QUERY_TIMEOUT", "2m",
			"--query.timeout=\"2m\""},
		{"ARG_QUERY_MAX-CONCURRENCY", "20",
			"--query.max-concurrency=\"20\""},
		{"ARG_LOG_LEVEL", "info",
			"--log.level=\"info\""},
	}
	defer func() {
		for _, envItem := range envMap {
			os.Unsetenv(envItem.key)
		}
	}()
	for _, envItem := range envMap {
		os.Setenv(envItem.key, envItem.value)
	}

	actFlags := EnvToPrometheusFlags("ARG")
	for _, envItem := range envMap {
		s.Contains(actFlags, envItem.expected)
	}
}
