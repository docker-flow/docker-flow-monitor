package prometheus

import (
	"testing"
	"github.com/stretchr/testify/suite"
	"os/exec"
	"os"
)

type RunTestSuite struct {
	suite.Suite
}

func (s *RunTestSuite) SetupTest() {
}

func TestRunUnitTestSuite(t *testing.T) {
	s := new(RunTestSuite)
	logPrintlnOrig := logPrintf
	defer func() { logPrintf = logPrintlnOrig }()
	logPrintf = func(format string, v ...interface{}) {}
	os.Setenv("GLOBAL_SCRAPE_INTERVAL", "5s")
	os.Setenv("ARG_CONFIG_FILE", "/etc/prometheus/prometheus.yml")
	os.Setenv("ARG_STORAGE_LOCAL_PATH", "/prometheus")
	os.Setenv("ARG_WEB_CONSOLE_LIBRARIES", "/usr/share/prometheus/console_libraries")
	os.Setenv("ARG_WEB_CONSOLE_TEMPLATES", "/usr/share/prometheus/consoles")
	suite.Run(t, s)
}

// Run

func (s *RunTestSuite) Test_Run_ExecutesPrometheus() {
	cmdRunOrig := cmdRun
	defer func() { cmdRun = cmdRunOrig }()
	actualArgs := []string{}
	cmdRun = func(cmd *exec.Cmd) error {
		actualArgs = cmd.Args
		return nil
	}

	Run()

	s.Equal([]string{"/bin/sh", "-c", "prometheus -config.file=/etc/prometheus/prometheus.yml -storage.local.path=/prometheus -web.console.libraries=/usr/share/prometheus/console_libraries -web.console.templates=/usr/share/prometheus/consoles"}, actualArgs)
}

func (s *RunTestSuite) Test_Run_AddsRoutePrefix() {
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

	Run()

	s.Equal([]string{"/bin/sh", "-c", "prometheus -config.file=/etc/prometheus/prometheus.yml -storage.local.path=/prometheus -web.console.libraries=/usr/share/prometheus/console_libraries -web.console.templates=/usr/share/prometheus/consoles -web.route-prefix=/something"}, actualArgs)
}

func (s *RunTestSuite) Test_Run_AddsExternalUrl() {
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

	Run()

	s.Equal([]string{"/bin/sh", "-c", "prometheus -config.file=/etc/prometheus/prometheus.yml -storage.local.path=/prometheus -web.console.libraries=/usr/share/prometheus/console_libraries -web.console.templates=/usr/share/prometheus/consoles -web.external-url=/something"}, actualArgs)
}

func (s *RunTestSuite) Test_Run_ReturnsError() {
	// Assumes that `prometheus` does not exist
	err := Run()

	s.Error(err)
}
