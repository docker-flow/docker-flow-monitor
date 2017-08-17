package prometheus

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"os/exec"
	"testing"
)

type ReloadTestSuite struct {
	suite.Suite
}

func (s *ReloadTestSuite) SetupTest() {
}

func TestReloadUnitTestSuite(t *testing.T) {
	s := new(ReloadTestSuite)
	logPrintlnOrig := logPrintf
	defer func() { logPrintf = logPrintlnOrig }()
	logPrintf = func(format string, v ...interface{}) {}
	suite.Run(t, s)
}

// Reload

func (s *ReloadTestSuite) Test_Reload_ReloadsPrometheus() {
	cmdRunOrig := cmdRun
	defer func() { cmdRun = cmdRunOrig }()
	actualArgs := [][]string{}
	cmdRun = func(cmd *exec.Cmd) error {
		actualArgs = append(actualArgs, cmd.Args)
		return nil
	}

	Reload()

	s.Contains(actualArgs, []string{"pkill", "-HUP", "prometheus"})
}

func (s *ReloadTestSuite) Test_Reload_ReturnsError_WhenReloadFails() {
	cmdRunOrig := cmdRun
	defer func() { cmdRun = cmdRunOrig }()
	actualArgs := [][]string{}
	cmdRun = func(cmd *exec.Cmd) error {
		actualArgs = append(actualArgs, cmd.Args)
		return fmt.Errorf("This is an error")
	}

	err := Reload()

	s.Error(err)
}
