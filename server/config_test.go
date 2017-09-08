package server

import (
	"../proxy"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/suite"
	"net/http"
	"testing"
)

type ConfigTestSuite struct {
	suite.Suite
}

func (s *ConfigTestSuite) SetupTest() {
}

func TestConfigUnitTestSuite(t *testing.T) {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("")
	proxy.Instance = proxyMock
	s := new(ConfigTestSuite)
	suite.Run(t, s)
}

// Get

func (s *ConfigTestSuite) Test_Get_SetsContentTypeToText() {
	var actual string
	orig := httpWriterSetContentType
	defer func() { httpWriterSetContentType = orig }()
	httpWriterSetContentType = func(w http.ResponseWriter, value string) {
		actual = value
	}
	c := NewConfig()
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/config",
		nil,
	)

	c.Get(w, req)

	s.Equal("text/html", actual)
}

func (s *ConfigTestSuite) Test_Get_SetsContentTypeToJson() {
	var actual string
	orig := httpWriterSetContentType
	defer func() { httpWriterSetContentType = orig }()
	httpWriterSetContentType = func(w http.ResponseWriter, value string) {
		actual = value
	}
	c := NewConfig()
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/config?type=json",
		nil,
	)

	c.Get(w, req)

	s.Equal("application/json", actual)
}

func (s *ConfigTestSuite) Test_Get_WritesHeaderStatus500_WhenProxyReadConfigReturnsAnError() {
	c := NewConfig()
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/config",
		nil,
	)
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("ReadConfig")
	proxyMock.On("ReadConfig").Return("", fmt.Errorf("This is an error"))
	proxy.Instance = proxyMock

	c.Get(w, req)

	w.AssertCalled(s.T(), "WriteHeader", 500)
}

func (s *ConfigTestSuite) Test_Get_WritesConfig() {
	c := NewConfig()
	expected := "This is what we expect"
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/config",
		nil,
	)
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("ReadConfig")
	proxyMock.On("ReadConfig").Return(expected, nil)
	proxy.Instance = proxyMock

	c.Get(w, req)

	w.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ConfigTestSuite) Test_Get_WritesServices() {
	c := NewConfig()
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/config?type=json",
		nil,
	)
	services := map[string]proxy.Service{
		"my-service-1": {ServiceName: "my-service-1"},
		"my-service-2": {ServiceName: "my-service-2"},
	}
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("GetServices")
	proxyMock.On("GetServices").Return(services)
	proxy.Instance = proxyMock
	expected, _ := json.Marshal(services)

	c.Get(w, req)

	w.AssertCalled(s.T(), "Write", []byte(expected))
}
