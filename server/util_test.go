package server

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type UtilTestSuite struct {
	suite.Suite
}

func (s *UtilTestSuite) SetupTest() {
}

func TestUtilUnitTestSuite(t *testing.T) {
	s := new(UtilTestSuite)
	suite.Run(t, s)
}

// SendDistributeRequests

func (s *ServerTestSuite) Test_SendDistributeRequests_InvokesLookupHost() {
	var actualHost string
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		actualHost = host
		return []string{}, nil
	}
	req, _ := http.NewRequest("GET", s.ReconfigureUrl, nil)

	sendDistributeRequests(req, "8080", s.ServiceName)

	s.Assert().Equal(fmt.Sprintf("tasks.%s", s.ServiceName), actualHost)
}

func (s *ServerTestSuite) Test_SendDistributeRequests_ReturnsError_WhenLookupHostFails() {
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		return []string{}, fmt.Errorf("This is an LookupHost error")
	}
	req, _ := http.NewRequest("GET", s.ReconfigureUrl, nil)

	status, err := sendDistributeRequests(req, "8080", s.ServiceName)

	s.Assertions.Equal(http.StatusBadRequest, status)
	s.Assertions.Error(err)
}

func (s *ServerTestSuite) Test_SendDistributeRequests_SendsHttpRequestForEachIp() {
	var actualPath string
	var actualQuery url.Values
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualQuery = r.URL.Query()
		actualPath = r.URL.Path
	}))
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	dnsIpsOrig := s.DnsIps
	defer func() { s.DnsIps = dnsIpsOrig }()
	s.DnsIps = []string{strings.Split(tsAddr, ":")[0]}
	port := strings.Split(tsAddr, ":")[1]

	addr := fmt.Sprintf("http://initial-proxy-address:%s%s&distribute=true", port, s.ReconfigureUrl)
	req, _ := http.NewRequest("GET", addr, nil)

	sendDistributeRequests(req, port, s.ServiceName)

	s.Assert().Equal(s.BaseUrl, actualPath)
	s.Assert().Equal("false", actualQuery.Get("distribute"))
}

func (s *ServerTestSuite) Test_SendDistributeRequests_SendsHttpRequestForEachIpWithTheCorrectMethod() {
	actualProtocol := "GET"
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualProtocol = r.Method
	}))
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	dnsIpsOrig := s.DnsIps
	defer func() { s.DnsIps = dnsIpsOrig }()
	s.DnsIps = []string{strings.Split(tsAddr, ":")[0]}
	port := strings.Split(tsAddr, ":")[1]

	addr := fmt.Sprintf("http://initial-proxy-address:%s%s&distribute=true", port, s.ReconfigureUrl)
	req, _ := http.NewRequest("PUT", addr, nil)

	sendDistributeRequests(req, port, s.ServiceName)

	s.Assert().Equal("PUT", actualProtocol)
}

func (s *ServerTestSuite) Test_SendDistributeRequests_SendsHttpRequestForEachIpWithTheBody() {
	actualBody := ""
	expectedBody := "THIS IS BODY"
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := ioutil.ReadAll(req.Body)
		actualBody = string(body)
	}))
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	dnsIpsOrig := s.DnsIps
	defer func() { s.DnsIps = dnsIpsOrig }()
	s.DnsIps = []string{strings.Split(tsAddr, ":")[0]}
	port := strings.Split(tsAddr, ":")[1]

	addr := fmt.Sprintf("http://initial-proxy-address:%s%s&distribute=true", port, s.ReconfigureUrl)
	req, _ := http.NewRequest("PUT", addr, strings.NewReader(expectedBody))

	sendDistributeRequests(req, port, s.ServiceName)

	s.Assert().Equal(expectedBody, actualBody)
}

func (s *ServerTestSuite) Test_SendDistributeRequests_ReturnsError_WhenRequestFail() {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer func() { testServer.Close() }()

	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	dnsIpsOrig := s.DnsIps
	defer func() { s.DnsIps = dnsIpsOrig }()
	s.DnsIps = []string{strings.Split(tsAddr, ":")[0]}
	port := strings.Split(tsAddr, ":")[1]

	addr := fmt.Sprintf("http://initial-proxy-address:%s%s&distribute=true", port, s.ReconfigureUrl)
	req, _ := http.NewRequest("GET", addr, nil)

	actualStatus, err := sendDistributeRequests(req, port, s.ServiceName)

	s.Assertions.Equal(http.StatusBadRequest, actualStatus)
	s.Assertions.Error(err)
}

func (s *ServerTestSuite) Test_SendDistributeRequests_ReturnsError_WhenProxyIPsAreNotFound() {
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		return []string{}, nil
	}
	req, _ := http.NewRequest("GET", s.ReconfigureUrl, nil)

	actualStatus, err := sendDistributeRequests(req, "1234", s.ServiceName)

	s.Assertions.Equal(http.StatusBadRequest, actualStatus)
	s.Assertions.Error(err)
}
