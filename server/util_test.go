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
	baseUrl        string
	reconfigureUrl string
	serviceName    string
	dnsIps         []string
	server         *httptest.Server
}

func (s *UtilTestSuite) SetupTest() {
}

func TestUtilUnitTestSuite(t *testing.T) {
	s := new(UtilTestSuite)
	s.baseUrl = "/v1/docker-flow-proxy/reconfigure"
	s.serviceName = "my-fancy-service"
	s.reconfigureUrl = fmt.Sprintf(
		"%s?serviceName=%s&serviceColor=pink&servicePath=/path/to/my/service/api&serviceDomain=my-domain.com",
		s.baseUrl,
		s.serviceName,
	)
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		return s.dnsIps, nil
	}
	s.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	addr := strings.Replace(s.server.URL, "http://", "", -1)
	s.dnsIps = []string{strings.Split(addr, ":")[0]}
	suite.Run(t, s)
}

// SendDistributeRequests

func (s *UtilTestSuite) Test_SendDistributeRequests_InvokesLookupHost() {
	var actualHost string
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		actualHost = host
		return []string{}, nil
	}
	req, _ := http.NewRequest("GET", s.reconfigureUrl, nil)

	sendDistributeRequests(req, "8080", s.serviceName)

	s.Assert().Equal(fmt.Sprintf("tasks.%s", s.serviceName), actualHost)
}

func (s *UtilTestSuite) Test_SendDistributeRequests_ReturnsError_WhenLookupHostFails() {
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		return []string{}, fmt.Errorf("This is an LookupHost error")
	}
	req, _ := http.NewRequest("GET", s.reconfigureUrl, nil)

	status, err := sendDistributeRequests(req, "8080", s.serviceName)

	s.Assertions.Equal(http.StatusBadRequest, status)
	s.Assertions.Error(err)
}

func (s *UtilTestSuite) Test_SendDistributeRequests_SendsHttpRequestForEachIp() {
	var actualPath string
	var actualQuery url.Values
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualQuery = r.URL.Query()
		actualPath = r.URL.Path
	}))
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	dnsIpsOrig := s.dnsIps
	defer func() { s.dnsIps = dnsIpsOrig }()
	s.dnsIps = []string{strings.Split(tsAddr, ":")[0]}
	port := strings.Split(tsAddr, ":")[1]

	addr := fmt.Sprintf("http://initial-proxy-address:%s%s&distribute=true", port, s.reconfigureUrl)
	req, _ := http.NewRequest("GET", addr, nil)

	sendDistributeRequests(req, port, s.serviceName)

	s.Assert().Equal(s.baseUrl, actualPath)
	s.Assert().Equal("false", actualQuery.Get("distribute"))
}

func (s *UtilTestSuite) Test_SendDistributeRequests_SendsHttpRequestForEachIpWithTheCorrectMethod() {
	actualProtocol := "GET"
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualProtocol = r.Method
	}))
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	dnsIpsOrig := s.dnsIps
	defer func() { s.dnsIps = dnsIpsOrig }()
	s.dnsIps = []string{strings.Split(tsAddr, ":")[0]}
	port := strings.Split(tsAddr, ":")[1]

	addr := fmt.Sprintf("http://initial-proxy-address:%s%s&distribute=true", port, s.reconfigureUrl)
	req, _ := http.NewRequest("PUT", addr, nil)

	sendDistributeRequests(req, port, s.serviceName)

	s.Assert().Equal("PUT", actualProtocol)
}

func (s *UtilTestSuite) Test_SendDistributeRequests_SendsHttpRequestForEachIpWithTheBody() {
	actualBody := ""
	expectedBody := "THIS IS BODY"
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		body, _ := ioutil.ReadAll(req.Body)
		actualBody = string(body)
	}))
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	dnsIpsOrig := s.dnsIps
	defer func() { s.dnsIps = dnsIpsOrig }()
	s.dnsIps = []string{strings.Split(tsAddr, ":")[0]}
	port := strings.Split(tsAddr, ":")[1]

	addr := fmt.Sprintf("http://initial-proxy-address:%s%s&distribute=true", port, s.reconfigureUrl)
	req, _ := http.NewRequest("PUT", addr, strings.NewReader(expectedBody))

	sendDistributeRequests(req, port, s.serviceName)

	s.Assert().Equal(expectedBody, actualBody)
}

func (s *UtilTestSuite) Test_SendDistributeRequests_ReturnsError_WhenRequestFail() {
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer func() { testServer.Close() }()

	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	dnsIpsOrig := s.dnsIps
	defer func() { s.dnsIps = dnsIpsOrig }()
	s.dnsIps = []string{strings.Split(tsAddr, ":")[0]}
	port := strings.Split(tsAddr, ":")[1]

	addr := fmt.Sprintf("http://initial-proxy-address:%s%s&distribute=true", port, s.reconfigureUrl)
	req, _ := http.NewRequest("GET", addr, nil)

	actualStatus, err := sendDistributeRequests(req, port, s.serviceName)

	s.Assertions.Equal(http.StatusBadRequest, actualStatus)
	s.Assertions.Error(err)
}

func (s *UtilTestSuite) Test_SendDistributeRequests_ReturnsError_WhenProxyIPsAreNotFound() {
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		return []string{}, nil
	}
	req, _ := http.NewRequest("GET", s.reconfigureUrl, nil)

	actualStatus, err := sendDistributeRequests(req, "1234", s.serviceName)

	s.Assertions.Equal(http.StatusBadRequest, actualStatus)
	s.Assertions.Error(err)
}
