package prometheus

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"fmt"
)

type AlertTestSuite struct {
	suite.Suite
}

func (s *AlertTestSuite) SetupTest() {
}

func TestAlertUnitTestSuite(t *testing.T) {
	s := new(ConfigTestSuite)
	logPrintlnOrig := LogPrintf
	defer func() { LogPrintf = logPrintlnOrig }()
	LogPrintf = func(format string, v ...interface{}) {}
	suite.Run(t, s)
}

// GetAlertConfig

func (s *AlertTestSuite) Test_GetAlertConfig_ReturnsConfigWithData() {
	expected := ""
	alerts := map[string]Alert{}
	for _, i := range []int{1, 2} {
		expected += fmt.Sprintf(`
ALERT alertNameFormatted%d
  IF alert-if-%d
  FOR alert-for-%d
`, i, i, i)
		alerts[fmt.Sprintf("alert-name-%d", i)] = Alert{
			AlertNameFormatted: fmt.Sprintf("alertNameFormatted%d", i),
			ServiceName: fmt.Sprintf("my-service-%d", i),
			AlertName: fmt.Sprintf("alert-name-%d", i),
			AlertIf: fmt.Sprintf("alert-if-%d", i),
			AlertFor: fmt.Sprintf("alert-for-%d", i),
		}
	}

	actual := GetAlertConfig(alerts)

	s.Equal(expected, actual)
}
