package server

import (
	"../proxy"
	"github.com/stretchr/testify/suite"
	//	"net/http"
	"testing"
	//	"fmt"
	//	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
)

type MetricsTestSuite struct {
	suite.Suite
}

func (s *MetricsTestSuite) SetupTest() {
}

func TestMetricsUnitTestSuite(t *testing.T) {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("")
	proxy.Instance = proxyMock

	s := new(MetricsTestSuite)
	suite.Run(t, s)
}

// NewMetrics

func (s *MetricsTestSuite) Test_NewMetrics_SetsMetricsUrl() {
	expected := "http://acme.com"
	m := NewMetrics(expected)

	s.Equal(expected, m.getMetricsUrl())
}

func (s *MetricsTestSuite) Test_NewMetrics_SetsMetricsUrl_WhenNotPresent() {
	expected := "http://localhost/admin?stats;csv"
	m := NewMetrics("")

	s.Equal(expected, m.getMetricsUrl())
}

func (s *MetricsTestSuite) Test_NewMetrics_SetsMetricsUrlWithCredentials() {
	defer func() {
		os.Unsetenv("STATS_USER")
		os.Unsetenv("STATS_USER_ENV")
		os.Unsetenv("STATS_PASS")
		os.Unsetenv("STATS_PASS_ENV")
		os.Unsetenv("STATS_URI")
		os.Unsetenv("STATS_URI_ENV")
		os.Unsetenv("STATS_PORT")
	}()
	os.Setenv("STATS_USER_ENV", "STATS_USER")
	os.Setenv("STATS_USER", "my-user")
	os.Setenv("STATS_PASS_ENV", "STATS_PASS")
	os.Setenv("STATS_PASS", "my-pass")
	os.Setenv("STATS_URI", "/myStats")
	os.Setenv("STATS_URI_ENV", "STATS_URI")
	os.Setenv("STATS_PORT", "81")
	expected := "http://my-user:my-pass@localhost:81/myStats;csv"
	m := NewMetrics("")

	s.Equal(expected, m.getMetricsUrl())
}

func (s *MetricsTestSuite) Test_NewMetrics_SetsMetricsUrlWithoutCredentials_WhenNone() {
	defer func() {
		os.Unsetenv("STATS_USER")
		os.Unsetenv("STATS_USER_ENV")
		os.Unsetenv("STATS_PASS")
		os.Unsetenv("STATS_PASS_ENV")
	}()
	os.Setenv("STATS_USER_ENV", "STATS_USER")
	os.Setenv("STATS_USER", "none")
	os.Setenv("STATS_PASS_ENV", "STATS_PASS")
	os.Setenv("STATS_PASS", "none")
	expected := "http://localhost/admin?stats;csv"
	m := NewMetrics("")

	s.Equal(expected, m.getMetricsUrl())
}

// Get

func (s *MetricsTestSuite) Test_Get_SetsContentTypeToText() {
	var actual string
	orig := httpWriterSetContentType
	defer func() { httpWriterSetContentType = orig }()
	httpWriterSetContentType = func(w http.ResponseWriter, value string) {
		actual = value
	}
	m := metrics{}
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/metrics",
		nil,
	)

	m.Get(w, req)

	s.Equal("text/html", actual)
}

func (s *MetricsTestSuite) Test_Get_WritesHeaderStatus500_WhenLookupHostFails() {
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		return []string{}, fmt.Errorf("This is an LookupHost error")
	}
	m := metrics{}
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/metrics?distribute=true",
		nil,
	)

	m.Get(w, req)

	w.AssertCalled(s.T(), "WriteHeader", 500)
}

func (s *MetricsTestSuite) Test_Get_SendsRequestsToAllReplicas_WhenDistributeIsTrue() {
	m := metrics{}
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/metrics?distribute=true",
		nil,
	)
	var actualPath, actualQuery string
	hits := 0
	testServer1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualPath = r.URL.Path
		actualQuery = r.URL.RawQuery
		hits++
	}))
	defer func() { testServer1.Close() }()
	testServer2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
	}))
	defer func() { testServer2.Close() }()
	tsAddr1 := strings.Replace(testServer1.URL, "http://", "", -1)
	tsAddr2 := strings.Replace(testServer1.URL, "http://", "", -1)
	ip1, port1, _ := net.SplitHostPort(tsAddr1)
	ip2, port2, _ := net.SplitHostPort(tsAddr2)
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		hostPort1 := net.JoinHostPort(ip1, port1)
		hostPort2 := net.JoinHostPort(ip2, port2)
		return []string{hostPort1, hostPort2}, nil
	}

	m.Get(w, req)

	s.Equal("/v1/docker-flow-proxy/metrics", actualPath)
	s.Equal("distribute=false", actualQuery)
	s.Equal(2, hits)
}

func (s *MetricsTestSuite) Test_Get_ReturnsMetricsFromAllReplicas_WhenDistributeIsTrue() {
	m := metrics{}
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/metrics?distribute=true",
		nil,
	)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("This is a set of metrics"))
	}))
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	ip, port, _ := net.SplitHostPort(tsAddr)
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		hostPort := net.JoinHostPort(ip, port)
		return []string{hostPort, hostPort, hostPort}, nil
	}

	m.Get(w, req)

	w.AssertCalled(s.T(), "Write", []byte("This is a set of metrics\nThis is a set of metrics\nThis is a set of metrics\n"))
}

func (s *MetricsTestSuite) Test_Get_WritesHeaderStatus500_WhenMetricsCanNotBeFetchedAndDistributeIsTrue() {
	m := metrics{}
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/metrics?distribute=true",
		nil,
	)
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		return []string{"http://this_url_does_not_exist:8080"}, nil
	}

	m.Get(w, req)

	w.AssertCalled(s.T(), "WriteHeader", 500)
}

func (s *MetricsTestSuite) Test_Get_WritesHeaderStatus500_WhenRequestFailsAndDistributeIsTrue() {
	m := metrics{}
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/metrics?distribute=true",
		nil,
	)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	ip, port, _ := net.SplitHostPort(tsAddr)
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		hostPort := net.JoinHostPort(ip, port)
		return []string{hostPort, hostPort, hostPort}, nil
	}

	m.Get(w, req)

	w.AssertCalled(s.T(), "WriteHeader", http.StatusInternalServerError)
}

func (s *MetricsTestSuite) Test_Get_ReturnsMetrics_WhenDistributeIsFalse() {
	w := getResponseWriterMock()
	expected := []byte("This is a set of metrics from a single replica")
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/metrics?distribute=false",
		nil,
	)
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(expected)
	}))
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	addr := fmt.Sprintf("http://admin:admin@%s/admin?stats;csv", tsAddr)
	m := NewMetrics(addr)

	m.Get(w, req)

	w.AssertCalled(s.T(), "Write", expected)
}

func (s *MetricsTestSuite) Test_Get_WritesHeaderStatus500_WhenMetricsRetrievalFailsDistributeIsFalse() {
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/metrics?distribute=false",
		nil,
	)
	m := NewMetrics("http://this_url_does_not_exist")

	m.Get(w, req)

	w.AssertCalled(s.T(), "WriteHeader", http.StatusInternalServerError)
}
