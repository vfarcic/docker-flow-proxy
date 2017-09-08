package server

import (
	"../proxy"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type CertTestSuite struct {
	suite.Suite
	serviceName string
}

func (s *CertTestSuite) SetupTest() {
}

func TestCertUnitTestSuite(t *testing.T) {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("")
	proxy.Instance = proxyMock

	logPrintfOrig := logPrintf
	defer func() { logPrintf = logPrintfOrig }()
	logPrintf = func(format string, v ...interface{}) {}

	s := new(CertTestSuite)
	s.serviceName = "my-fancy-service"
	suite.Run(t, s)
}

// GetAll

func (s *CertTestSuite) Test_GetAll_SetsContentTypeToJson() {
	var actual string
	orig := httpWriterSetContentType
	defer func() { httpWriterSetContentType = orig }()
	httpWriterSetContentType = func(w http.ResponseWriter, value string) {
		actual = value
	}
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/certs",
		nil,
	)

	c.GetAll(w, req)

	s.Equal("application/json", actual)
}

func (s *CertTestSuite) Test_GetAll_WritesHeaderStatus200() {
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/certs",
		nil,
	)

	c.GetAll(w, req)

	w.AssertCalled(s.T(), "WriteHeader", 200)
}

func (s *CertTestSuite) Test_GetAll_WritesReturnsCert() {
	certs := []cert{}
	proxyCerts := map[string]string{}
	name := "my-service"
	path := fmt.Sprintf("/my/certs/dir/%s", name)
	cert := cert{
		ProxyServiceName: name,
		CertsDir:         "/my/certs/dir",
		CertContent:      "Content of the cert",
	}
	proxyCerts[path] = "Content of the cert"
	proxyCerts["/run/secrets"] = "Content of a cert from secrets. This cert should be ignored."
	certs = append(certs, cert)
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("GetCerts")
	proxyMock.On("GetCerts").Return(proxyCerts)
	proxy.Instance = proxyMock
	expected := CertResponse{
		Status:  "OK",
		Message: "",
		Certs:   certs,
	}
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"GET",
		"http://acme.com/v1/docker-flow-proxy/certs",
		nil,
	)

	actual, _ := c.GetAll(w, req)

	s.EqualValues(expected, actual)
}

// Init

func (s *CertTestSuite) Test_Init_InvokesLookupHost() {
	var actualHost string
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		actualHost = host
		return []string{}, nil
	}
	c := NewCert("../certs")
	c.ProxyServiceName = s.serviceName

	c.Init()

	s.Assert().Equal(fmt.Sprintf("tasks.%s", s.serviceName), actualHost)
}

func (s *CertTestSuite) Test_Init_ReturnsError_WhenLookupHostFails() {
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		return []string{}, fmt.Errorf("This is an LookupHost error")
	}
	c := NewCert("../certs")

	err := c.Init()

	s.Assertions.Error(err)
}

func (s *CertTestSuite) Test_Init_SendsHttpRequestForEachIp() {
	var actualPath string
	testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualPath = r.URL.Path
	}))
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	ip, port, _ := net.SplitHostPort(tsAddr)
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		hostPort := net.JoinHostPort(ip, port)
		return []string{hostPort}, nil
	}
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("")
	proxy.Instance = proxyMock

	c := NewCert("../certs")
	c.ProxyServiceName = s.serviceName

	c.Init()

	s.Assert().Equal("/v1/docker-flow-proxy/certs", actualPath)
}

func (s *CertTestSuite) Test_Init_DoesNotFail_WhenRequestFails() {
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		return []string{"unknown-address"}, nil
	}
	c := NewCert("../certs")
	c.ProxyServiceName = s.serviceName

	err := c.Init()

	s.NoError(err)
}

func (s *CertTestSuite) Test_Init_WritesCertToFile() {
	testServer := s.getCertGetAllMockServer(1, 3)
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	ip, port, _ := net.SplitHostPort(tsAddr)
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		hostPort := net.JoinHostPort(ip, port)
		return []string{hostPort}, nil
	}

	c := NewCert("../certs")
	path := fmt.Sprintf("%s/%s", c.CertsDir, "my-cert-3.pem")
	os.Remove(path)
	c.ProxyServiceName = s.serviceName
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("")
	proxy.Instance = proxyMock

	c.Init()

	actual, err := ioutil.ReadFile(path)

	s.NoError(err)
	s.Equal("Content of my-cert-3.pem", string(actual))
}

func (s *CertTestSuite) Test_Init_InvokesProxyCreateConfigFromTemplates() {
	testServer := s.getCertGetAllMockServer(1, 3)
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	ip, port, _ := net.SplitHostPort(tsAddr)
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		hostPort := net.JoinHostPort(ip, port)
		return []string{hostPort}, nil
	}
	c := NewCert("../certs")
	c.ProxyServiceName = s.serviceName
	c.ServicePort = port
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("")
	proxy.Instance = proxyMock

	c.Init()

	proxyMock.AssertCalled(s.T(), "CreateConfigFromTemplates")
}

func (s *CertTestSuite) Test_Init_InvokesProxyReload() {
	testServer := s.getCertGetAllMockServer(1, 3)
	defer func() { testServer.Close() }()
	tsAddr := strings.Replace(testServer.URL, "http://", "", -1)
	ip, port, _ := net.SplitHostPort(tsAddr)
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		hostPort := net.JoinHostPort(ip, port)
		return []string{hostPort}, nil
	}
	c := NewCert("../certs")
	c.ProxyServiceName = s.serviceName
	c.ServicePort = port
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("")
	proxy.Instance = proxyMock

	c.Init()

	proxyMock.AssertCalled(s.T(), "Reload")
}

func (s *CertTestSuite) Test_Init_WritesCertToFile_WhenItComesFromTheBiggestResponse() {
	testServer1 := s.getCertGetAllMockServer(1, 2)
	testServer2 := s.getCertGetAllMockServer(3, 5)
	defer func() {
		testServer1.Close()
		testServer2.Close()
	}()
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		tsAddr1 := strings.Replace(testServer1.URL, "http://", "", -1)
		ip1, port1, _ := net.SplitHostPort(tsAddr1)
		hostPort1 := net.JoinHostPort(ip1, port1)
		tsAddr2 := strings.Replace(testServer2.URL, "http://", "", -1)
		ip2, port2, _ := net.SplitHostPort(tsAddr2)
		hostPort2 := net.JoinHostPort(ip2, port2)
		return []string{hostPort1, hostPort2}, nil
	}
	c := NewCert("../certs")
	path2 := fmt.Sprintf("%s/%s", c.CertsDir, "my-cert-2.pem")
	os.Remove(path2)
	path3 := fmt.Sprintf("%s/%s", c.CertsDir, "my-cert-3.pem")
	os.Remove(path3)
	c.ProxyServiceName = s.serviceName
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("")
	proxy.Instance = proxyMock

	c.Init()

	_, err := ioutil.ReadFile(path2)
	s.Error(err)

	_, err = ioutil.ReadFile(path3)
	s.NoError(err)
}

func (s *CertTestSuite) getCertGetAllMockServer(from, to int) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		certs := []cert{}
		for i := from; i <= to; i++ {
			cert := cert{
				ProxyServiceName: fmt.Sprintf("my-cert-%d.pem", i),
				CertContent:      fmt.Sprintf("Content of my-cert-%d.pem", i),
			}
			certs = append(certs, cert)
		}
		msg := CertResponse{Status: "OK", Message: "", Certs: certs}
		httpWriterSetContentType(w, "application/json")
		w.WriteHeader(http.StatusOK)
		js, _ := json.Marshal(msg)
		w.Write(js)
	}))
}

// Put

func (s *CertTestSuite) Test_Put_SavesBodyAsFile() {
	c := NewCert("../certs")
	certName := "test.pem"
	expected := "THIS IS A CERTIFICATE"
	path := fmt.Sprintf("%s/%s", c.CertsDir, certName)
	os.Remove(path)
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		fmt.Sprintf("http://acme.com/v1/docker-flow-proxy/cert?certName=%s", certName),
		strings.NewReader(expected),
	)

	c.Put(w, req)
	actual, err := ioutil.ReadFile(path)

	s.NoError(err)
	s.Equal(expected, string(actual))
}

func (s *CertTestSuite) Test_Put_SetsContentTypeToJson() {
	var actual string
	orig := httpWriterSetContentType
	defer func() { httpWriterSetContentType = orig }()
	httpWriterSetContentType = func(w http.ResponseWriter, value string) {
		actual = value
	}
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		"http://acme.com/v1/docker-flow-proxy/cert?certName=my-cert.pem",
		strings.NewReader("cert content"),
	)

	c.Put(w, req)

	s.Equal("application/json", actual)
}

func (s *CertTestSuite) Test_Put_WritesHeaderStatus200() {
	expected, _ := json.Marshal(CertResponse{
		Status: "OK",
	})
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		"http://acme.com/v1/docker-flow-proxy/cert?certName=my-cert.pem",
		strings.NewReader("cert content"),
	)

	c.Put(w, req)

	w.AssertCalled(s.T(), "WriteHeader", 200)
	w.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *CertTestSuite) Test_Put_SendsDistributeRequests_WhenDistruibuteParamIsPresent() {
	expectedServiceName := "my-proxy-service"
	serviceNameOrig := os.Getenv("SERVICE_NAME")
	defer func() { os.Setenv("SERVICE_NAME", serviceNameOrig) }()
	os.Setenv("SERVICE_NAME", expectedServiceName)
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		"http://acme.com:1234/v1/docker-flow-proxy/cert?certName=my-cert.pem&distribute=true",
		strings.NewReader("cert content"),
	)
	actualServiceName := ""
	actualPort := ""
	sendDistributeRequestsOrig := sendDistributeRequests
	defer func() { sendDistributeRequests = sendDistributeRequestsOrig }()
	sendDistributeRequests = func(req *http.Request, port, serviceName string) (status int, err error) {
		actualServiceName = serviceName
		actualPort = port
		return 0, nil
	}
	c.Put(w, req)

	s.Equal(expectedServiceName, actualServiceName)
	s.Equal("1234", actualPort)
}

func (s *CertTestSuite) Test_Put_ReturnsError_WhenCertNameIsNotPresent() {
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		"http://acme.com:1234/v1/docker-flow-proxy/cert",
		strings.NewReader("cert content"),
	)

	_, err := c.Put(w, req)

	s.Error(err)
}

func (s *CertTestSuite) Test_Put_SendsDistributeRequestsToPort8080_WhenPortIsNotAvailable() {
	expectedServiceName := "my-proxy-service"
	serviceNameOrig := os.Getenv("SERVICE_NAME")
	defer func() { os.Setenv("SERVICE_NAME", serviceNameOrig) }()
	os.Setenv("SERVICE_NAME", expectedServiceName)
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		"http://acme.com/v1/docker-flow-proxy/cert?certName=my-cert.pem&distribute=true",
		strings.NewReader("cert content"),
	)
	actualServiceName := ""
	actualPort := ""
	sendDistributeRequestsOrig := sendDistributeRequests
	defer func() { sendDistributeRequests = sendDistributeRequestsOrig }()
	sendDistributeRequests = func(req *http.Request, port, serviceName string) (status int, err error) {
		actualServiceName = serviceName
		actualPort = port
		return 0, nil
	}

	c.Put(w, req)

	s.Equal("8080", actualPort)
	s.Equal(expectedServiceName, actualServiceName)
}

func (s *CertTestSuite) Test_Put_ReturnsError_WhenSendDistributeRequestsReturnsError() {
	serviceName := "my-proxy-service"
	serviceNameOrig := os.Getenv("SERVICE_NAME")
	defer func() { os.Setenv("SERVICE_NAME", serviceNameOrig) }()
	os.Setenv("SERVICE_NAME", serviceName)
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		"http://acme.com/v1/docker-flow-proxy/cert?certName=my-cert.pem&distribute=true",
		strings.NewReader("cert content"),
	)
	sendDistributeRequestsOrig := sendDistributeRequests
	defer func() { sendDistributeRequests = sendDistributeRequestsOrig }()
	sendDistributeRequests = func(req *http.Request, port, serviceName string) (status int, err error) {
		return 0, fmt.Errorf("This is an error")
	}

	_, err := c.Put(w, req)

	s.Error(err)
}

func (s *CertTestSuite) Test_Put_ReturnsError_WhenSendDistributeRequestsReturnsNon200Status() {
	serviceName := "my-proxy-service"
	serviceNameOrig := os.Getenv("SERVICE_NAME")
	defer func() { os.Setenv("SERVICE_NAME", serviceNameOrig) }()
	os.Setenv("SERVICE_NAME", serviceName)
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		"http://acme.com/v1/docker-flow-proxy/cert?certName=my-cert.pem&distribute=true",
		strings.NewReader("cert content"),
	)
	sendDistributeRequestsOrig := sendDistributeRequests
	defer func() { sendDistributeRequests = sendDistributeRequestsOrig }()
	sendDistributeRequests = func(req *http.Request, port, serviceName string) (status int, err error) {
		return 400, nil
	}

	_, err := c.Put(w, req)

	s.Error(err)
}

func (s *CertTestSuite) Test_Put_ReturnsError_WhenDirectoryDoesNotExist() {
	c := NewCert("THIS_PATH_DOES_NOT_EXIST")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		"http://acme.com/v1/docker-flow-proxy/cert?certName=test.pem",
		strings.NewReader("cert content"),
	)

	_, err := c.Put(w, req)

	s.Error(err)
}

func (s *CertTestSuite) Test_Put_WritesHeaderStatus400_WhenDirectoryDoesNotExist() {
	c := NewCert("THIS_PATH_DOES_NOT_EXIST")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		"http://acme.com/v1/docker-flow-proxy/cert?certName=test.pem",
		strings.NewReader("cert content"),
	)

	c.Put(w, req)

	w.AssertCalled(s.T(), "WriteHeader", 400)
}

func (s *CertTestSuite) Test_Put_ReturnsError_WhenCannotReadBody() {
	c := NewCert("../certs")
	w := getResponseWriterMock()
	r := ReaderMock{
		ReadMock: func([]byte) (int, error) { return 0, fmt.Errorf("This is an error") },
	}
	req, _ := http.NewRequest("PUT", "http://acme.com/v1/docker-flow-proxy/cert?certName=test.pem", r)

	_, err := c.Put(w, req)

	s.Error(err)
}

func (s *CertTestSuite) Test_Put_WritesHeaderStatus40_WhenCannotReadBody() {
	c := NewCert("../certs")
	w := getResponseWriterMock()
	r := ReaderMock{
		ReadMock: func([]byte) (int, error) { return 0, fmt.Errorf("This is an error") },
	}
	req, _ := http.NewRequest("PUT", "http://acme.com/v1/docker-flow-proxy/cert?certName=test.pem", r)

	c.Put(w, req)

	w.AssertCalled(s.T(), "WriteHeader", 400)
}

func (s *CertTestSuite) Test_Put_ReturnsCertPath() {
	c := NewCert("../certs")
	certName := "test.pem"
	expected, _ := filepath.Abs(fmt.Sprintf("%s/%s", c.CertsDir, certName))
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		fmt.Sprintf("http://acme.com/v1/docker-flow-proxy/cert?certName=%s", certName),
		strings.NewReader("cert content"),
	)

	actual, _ := c.Put(w, req)

	s.Equal(expected, actual)
}

func (s *CertTestSuite) Test_Put_ReturnsError_WhenCertNameDoesNotExist() {
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		fmt.Sprintf("http://acme.com/v1/docker-flow-proxy/cert"),
		strings.NewReader("cert content"),
	)

	_, err := c.Put(w, req)

	s.Error(err)
}

func (s *CertTestSuite) Test_Put_ReturnsError_WhenBodyIsEmpty() {
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		fmt.Sprintf("http://acme.com/v1/docker-flow-proxy/cert?certName=my-cert.pem"),
		strings.NewReader(""),
	)

	_, err := c.Put(w, req)

	s.Error(err)
}

func (s *CertTestSuite) Test_Put_InvokesProxyCreateConfigFromTemplates() {
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		"http://acme.com/v1/docker-flow-proxy/cert?certName=my-cert.pem",
		strings.NewReader("cert content"),
	)
	proxyMock := getProxyMock("")
	proxy.Instance = proxyMock

	c.Put(w, req)

	proxyMock.AssertCalled(s.T(), "CreateConfigFromTemplates")
}

func (s *CertTestSuite) Test_Put_InvokesProxyReload() {
	c := NewCert("../certs")
	w := getResponseWriterMock()
	req, _ := http.NewRequest(
		"PUT",
		"http://acme.com/v1/docker-flow-proxy/cert?certName=my-cert.pem",
		strings.NewReader("cert content"),
	)
	proxyMock := getProxyMock("")
	proxy.Instance = proxyMock

	c.Put(w, req)

	proxyMock.AssertCalled(s.T(), "Reload")
}

// NewCert

func (s *CertTestSuite) Test_NewCert_SetsCertsDir() {
	expected := "../certs"
	cert := NewCert(expected)

	s.Equal(expected, cert.CertsDir)
}

func (s *CertTestSuite) Test_NewCert_SetsServiceName() {
	serviceName := "my-proxy-service"
	serviceNameOrig := os.Getenv("SERVICE_NAME")
	defer func() { os.Setenv("SERVICE_NAME", serviceNameOrig) }()
	os.Setenv("SERVICE_NAME", serviceName)

	cert := NewCert("../certs")

	s.Equal(serviceName, cert.ProxyServiceName)
}

// Mock

// ReaderMock

type ReaderMock struct {
	ReadMock func([]byte) (int, error)
}

func (m ReaderMock) Read(p []byte) (int, error) {
	return m.ReadMock(p)
}

// ResponseWriterMock

type ResponseWriterMock struct {
	mock.Mock
}

func (m *ResponseWriterMock) Header() http.Header {
	m.Called()
	return make(map[string][]string)
}

func (m *ResponseWriterMock) Write(data []byte) (int, error) {
	params := m.Called(data)
	return params.Int(0), params.Error(1)
}

func (m *ResponseWriterMock) WriteHeader(header int) {
	m.Called(header)
}

func getResponseWriterMock() *ResponseWriterMock {
	mockObj := new(ResponseWriterMock)
	mockObj.On("Header").Return(nil)
	mockObj.On("Write", mock.Anything).Return(0, nil)
	mockObj.On("WriteHeader", mock.Anything)
	return mockObj
}

type ProxyMock struct {
	mock.Mock
}

func (m *ProxyMock) RunCmd(extraArgs []string) error {
	params := m.Called(extraArgs)
	return params.Error(0)
}

func (m *ProxyMock) CreateConfigFromTemplates() error {
	params := m.Called()
	return params.Error(0)
}

func (m *ProxyMock) ReadConfig() (string, error) {
	params := m.Called()
	return params.String(0), params.Error(1)
}

func (m *ProxyMock) Reload() error {
	params := m.Called()
	return params.Error(0)
}

func (m *ProxyMock) AddCert(certName string) {
	m.Called(certName)
}

func (m *ProxyMock) GetCerts() map[string]string {
	params := m.Called()
	return params.Get(0).(map[string]string)
}

func (m *ProxyMock) AddService(service proxy.Service) {
	m.Called(service)
}

func (m *ProxyMock) RemoveService(service string) {
	m.Called(service)
}

func (m *ProxyMock) GetServices() map[string]proxy.Service {
	params := m.Called()
	return params.Get(0).(map[string]proxy.Service)
}

func (m *ProxyMock) GetCertPaths() []string {
	params := m.Called()
	return params.Get(0).([]string)
}

func getProxyMock(skipMethod string) *ProxyMock {
	mockObj := new(ProxyMock)
	if skipMethod != "RunCmd" {
		mockObj.On("RunCmd", mock.Anything).Return(nil)
	}
	if skipMethod != "CreateConfigFromTemplates" {
		mockObj.On("CreateConfigFromTemplates").Return(nil)
	}
	if skipMethod != "ReadConfig" {
		mockObj.On("ReadConfig").Return("", nil)
	}
	if skipMethod != "Reload" {
		mockObj.On("Reload").Return(nil)
	}
	if skipMethod != "AddCert" {
		mockObj.On("AddCert", mock.Anything).Return(nil)
	}
	if skipMethod != "GetCerts" {
		mockObj.On("GetCerts").Return(map[string]string{})
	}
	if skipMethod != "AddService" {
		mockObj.On("AddService", mock.Anything)
	}
	if skipMethod != "RemoveService" {
		mockObj.On("RemoveService", mock.Anything)
	}
	if skipMethod != "GetServices" {
		mockObj.On("GetServices").Return(map[string]proxy.Service{})
	}
	if skipMethod != "GetCertPaths" {
		mockObj.On("GetCertPaths")
	}
	return mockObj
}
