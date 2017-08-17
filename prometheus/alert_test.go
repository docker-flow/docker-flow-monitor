package prometheus

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"testing"
)

type AlertTestSuite struct {
	suite.Suite
}

func (s *AlertTestSuite) SetupTest() {
}

func TestAlertUnitTestSuite(t *testing.T) {
	s := new(AlertTestSuite)
	logPrintlnOrig := logPrintf
	defer func() { logPrintf = logPrintlnOrig }()
	logPrintf = func(format string, v ...interface{}) {}
	suite.Run(t, s)
}

// GetAlertConfig

func (s *AlertTestSuite) Test_GetAlertConfig_ReturnsConfigWithData() {
	expected := ""
	alerts := s.getTestAlerts()
	for _, i := range []int{1, 2} {
		expected += fmt.Sprintf(`
ALERT alertNameFormatted%d
  IF alert-if-%d
  FOR alert-for-%d
`, i, i, i)
	}

	actual := GetAlertConfig(alerts)

	s.Equal(expected, actual)
}

func (s *AlertTestSuite) Test_GetAlertConfig_ReturnsConfigWithLabels_WhenPresent() {
	expected := ""
	alerts := s.getTestAlerts()
	for _, i := range []int{1, 2} {
		expected += fmt.Sprintf(`
ALERT alertNameFormatted%d
  IF alert-if-%d
  FOR alert-for-%d
  LABELS {
    alert-label-%d-1 = "alert-label-value-%d-1",
    alert-label-%d-2 = "alert-label-value-%d-2",
  }
`, i, i, i, i, i, i, i)
		key := fmt.Sprintf("alert-name-%d", i)
		alert := alerts[key]
		alert.AlertLabels = map[string]string{
			fmt.Sprintf("alert-label-%d-1", i): fmt.Sprintf("alert-label-value-%d-1", i),
			fmt.Sprintf("alert-label-%d-2", i): fmt.Sprintf("alert-label-value-%d-2", i),
		}
		alerts[key] = alert
	}

	actual := GetAlertConfig(alerts)

	s.Equal(expected, actual)
}

func (s *AlertTestSuite) Test_GetAlertConfig_ReturnsConfigWithAnnotations_WhenPresent() {
	expected := ""
	alerts := s.getTestAlerts()
	for _, i := range []int{1, 2} {
		expected += fmt.Sprintf(`
ALERT alertNameFormatted%d
  IF alert-if-%d
  FOR alert-for-%d
  ANNOTATIONS {
    alert-annotation-%d-1 = "alert-annotation-value-%d-1",
    alert-annotation-%d-2 = "alert-annotation-value-%d-2",
  }
`, i, i, i, i, i, i, i)
		key := fmt.Sprintf("alert-name-%d", i)
		alert := alerts[key]
		alert.AlertAnnotations = map[string]string{
			fmt.Sprintf("alert-annotation-%d-1", i): fmt.Sprintf("alert-annotation-value-%d-1", i),
			fmt.Sprintf("alert-annotation-%d-2", i): fmt.Sprintf("alert-annotation-value-%d-2", i),
		}
		alerts[key] = alert
	}

	actual := GetAlertConfig(alerts)

	s.Equal(expected, actual)
}

// Util

func (s *AlertTestSuite) getTestAlerts() map[string]Alert {
	alerts := map[string]Alert{}
	for _, i := range []int{1, 2} {
		alerts[fmt.Sprintf("alert-name-%d", i)] = Alert{
			AlertNameFormatted: fmt.Sprintf("alertNameFormatted%d", i),
			ServiceName:        fmt.Sprintf("my-service-%d", i),
			AlertName:          fmt.Sprintf("alert-name-%d", i),
			AlertIf:            fmt.Sprintf("alert-if-%d", i),
			AlertFor:           fmt.Sprintf("alert-for-%d", i),
		}
	}
	return alerts
}
