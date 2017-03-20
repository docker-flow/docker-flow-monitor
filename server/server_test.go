package server

import (
	"github.com/stretchr/testify/suite"
	"testing"
"net/http"
	"time"
	"fmt"
	"encoding/json"
)

type ServerTestSuite struct {
	suite.Suite
}

func (s *ServerTestSuite) SetupTest() {
}

func TestServerUnitTestSuite(t *testing.T) {
	s := new(ServerTestSuite)
	suite.Run(t, s)
}

// NewServe

func (s *ServerTestSuite) Test_New_ReturnsServe() {
	serve := New()

	s.NotNil(serve)
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

// AlertHandler

func (s *ServerTestSuite) Test_AlertHandler_ReturnsJson() {
	expected := Response{
		Status: "OK",
		Alert: Alert{
			AlertName: "my-alert",
			AlertIf: "my-if",
			AlertFrom: "my-from",
		},
	}
	actual := Response{}
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(header int) {},
		HeaderMock: func() http.Header {
			return http.Header{}
		},
		WriteMock: func(content []byte) (int, error) {
			json.Unmarshal(content, &actual)
			return 0, nil
		},
	}
	addr := fmt.Sprintf(
		"/v1/docker-flow-monitor/alert?alertName=%s&alertIf=%s&alertFrom=%s",
		expected.AlertName,
		expected.AlertIf,
		expected.AlertFrom,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.AlertHandler(rwMock, req)

	s.Equal(expected, actual)
}

func (s *ServerTestSuite) Test_AlertHandler_SetsContentHeaderToJson() {
	actual := http.Header{}
	rwMock := ResponseWriterMock{
		WriteHeaderMock: func(header int) {},
		HeaderMock: func() http.Header {
			return actual
		},
		WriteMock: func(content []byte) (int, error) {
			return 0, nil
		},
	}
	addr := "/v1/docker-flow-monitor/alert?alertName=my-alert&alertIf=my-if"
	req, _ := http.NewRequest("GET", addr, nil)

	serve := New()
	serve.AlertHandler(rwMock, req)

	s.Equal("application/json", actual.Get("Content-Type"))
}

// Mock

type ResponseWriterMock struct {
	HeaderMock      func() http.Header
	WriteMock       func([]byte) (int, error)
	WriteHeaderMock func(int)
}

func (m ResponseWriterMock) Header() http.Header {
	return m.HeaderMock()
}

func (m ResponseWriterMock) Write(content []byte) (int, error) {
	return m.WriteMock(content)
}

func (m ResponseWriterMock) WriteHeader(header int) {
	m.WriteHeaderMock(header)
}
