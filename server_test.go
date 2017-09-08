// +build !integration

package main

import (
	"./actions"
	"./proxy"
	"./server"
	"fmt"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

type ServerTestSuite struct {
	suite.Suite
	proxy.Service
	BaseUrl            string
	ReconfigureBaseUrl string
	RemoveBaseUrl      string
	ReconfigureUrl     string
	RemoveUrl          string
	ConfigUrl          string
	CertUrl            string
	CertsUrl           string
	ResponseWriter     *ResponseWriterMock
	RequestReconfigure *http.Request
	RequestRemove      *http.Request
	InstanceName       string
	DnsIps             []string
	Server             *httptest.Server
	sd                 proxy.ServiceDest
}

func TestServerUnitTestSuite(t *testing.T) {
	s := new(ServerTestSuite)
	retryIntervalOrig := os.Getenv("RELOAD_INTERVAL")
	defer func() { os.Setenv("RELOAD_INTERVAL", retryIntervalOrig) }()
	os.Setenv("RELOAD_INTERVAL", "1")
	suite.Run(t, s)
}

func (s *ServerTestSuite) SetupTest() {
	s.sd = proxy.ServiceDest{
		ReqMode:       "http",
		ServiceDomain: []string{"my-domain.com"},
		ServicePath:   []string{"/path/to/my/service/api", "/path/to/my/other/service/api"},
	}
	s.Service.ServiceDest = []proxy.ServiceDest{s.sd}
	s.InstanceName = "proxy-test-instance"
	s.ServiceName = "myService"
	s.OutboundHostname = "machine-123.my-company.com"
	s.BaseUrl = "/v1/docker-flow-proxy"
	s.ReconfigureBaseUrl = fmt.Sprintf("%s/reconfigure", s.BaseUrl)
	s.RemoveBaseUrl = fmt.Sprintf("%s/remove", s.BaseUrl)
	s.ReconfigureUrl = fmt.Sprintf(
		"%s?serviceName=%s&servicePath=%s&serviceDomain=%s&outboundHostname=%s",
		s.ReconfigureBaseUrl,
		s.ServiceName,
		strings.Join(s.sd.ServicePath, ","),
		strings.Join(s.sd.ServiceDomain, ","),
		s.OutboundHostname,
	)
	s.RemoveUrl = fmt.Sprintf("%s?serviceName=%s", s.RemoveBaseUrl, s.ServiceName)
	s.CertUrl = fmt.Sprintf("%s/cert?my-cert.pem", s.BaseUrl)
	s.CertsUrl = fmt.Sprintf("%s/certs", s.BaseUrl)
	s.ConfigUrl = "/v1/docker-flow-proxy/config"
	s.ResponseWriter = getResponseWriterMock()
	s.RequestReconfigure, _ = http.NewRequest("GET", s.ReconfigureUrl, nil)
	s.RequestRemove, _ = http.NewRequest("GET", s.RemoveUrl, nil)
	httpListenAndServe = func(addr string, handler http.Handler) error {
		return nil
	}
	serverImpl = serve{
		BaseReconfigure: actions.BaseReconfigure{
			InstanceName: s.InstanceName,
		},
	}
	logPrintfOrig := logPrintf
	defer func() { logPrintf = logPrintfOrig }()
	logPrintf = func(format string, v ...interface{}) {}
}

// Execute

func (s *ServerTestSuite) Test_Execute_InvokesHTTPListenAndServe() {
	serverImpl := serve{
		IP:   "myIp",
		Port: "1234",
	}
	var actual string
	expected := fmt.Sprintf("%s:%s", serverImpl.IP, serverImpl.Port)
	httpListenAndServe = func(addr string, handler http.Handler) error {
		actual = addr
		return nil
	}

	serverImpl.Execute([]string{})
	time.Sleep(1 * time.Millisecond)

	s.Equal(expected, actual)
}

func (s *ServerTestSuite) Test_Execute_ReturnsError_WhenHTTPListenAndServeFails() {
	orig := httpListenAndServe
	defer func() {
		httpListenAndServe = orig
	}()
	httpListenAndServe = func(addr string, handler http.Handler) error {
		return fmt.Errorf("This is an error")
	}

	actual := serverImpl.Execute([]string{})

	s.Error(actual)
}

func (s *ServerTestSuite) Test_Execute_InvokesRunExecute() {
	orig := newRun
	defer func() {
		newRun = orig
	}()
	mockObj := getRunMock("")
	newRun = func() runner {
		return mockObj
	}

	serverImpl.Execute([]string{})

	mockObj.AssertCalled(s.T(), "Execute", []string{})
}

func (s *ServerTestSuite) Test_Execute_InvokesCertInit() {
	invoked := false
	err := serverImpl.Execute([]string{})
	certOrig := cert
	defer func() { cert = certOrig }()
	cert = CertMock{
		GetInitMock: func() error {
			invoked = true
			return nil
		},
	}
	serverImpl.Execute([]string{})

	s.NoError(err)
	s.True(invoked)
}

func (s *ServerTestSuite) Test_Execute_InvokesReconfigureExecuteForEachServiceDefinedInEnvVars() {
	called := 0
	defer MockServer(ServerMock{
		GetServicesFromEnvVarsMock: func() *[]proxy.Service { return &[]proxy.Service{{}, {}} },
	})()
	defer MockFetch(FetchMock{})()
	defer MockReconfigure(ReconfigureMock{
		ExecuteMock: func(reloadAfter bool) error {
			called++
			return nil
		},
	})()

	serverImpl.Execute([]string{})
	s.Equal(2, called)
}

func (s *ServerTestSuite) Test_Execute_InvokesReloadAllServicesWithListenerAddress() {
	expectedListenerAddress := "swarm-listener"
	actualListenerAddressChan := make(chan string)
	defer MockFetch(FetchMock{
		ReloadConfigMock: func(baseData actions.BaseReconfigure, listenerAddr string) error {
			actualListenerAddressChan <- listenerAddr
			return nil
		},
	})()
	serverImpl.ListenerAddress = expectedListenerAddress

	serverImpl.Execute([]string{})

	actualListenerAddress, _ := <-actualListenerAddressChan
	close(actualListenerAddressChan)
	s.Equal(fmt.Sprintf("http://%s:8080", expectedListenerAddress), actualListenerAddress)
}

func (s *ServerTestSuite) Test_Execute_RetriesContactingSwarmListenerAddress_WhenError() {
	expectedListenerAddress := "swarm-listener"
	actualListenerAddressChan := make(chan string)
	callNum := 0
	defer MockFetch(FetchMock{
		ReloadConfigMock: func(baseData actions.BaseReconfigure, listenerAddr string) error {
			callNum = callNum + 1
			actualListenerAddressChan <- fmt.Sprintf("%s-%d", listenerAddr, callNum)
			if callNum == 2 {
				close(actualListenerAddressChan)
				return nil
			}
			return fmt.Errorf("On iteration %d", callNum)
		},
	})()
	defer func() {
		os.Unsetenv("LISTENER_ADDRESS")
	}()
	serverImpl.ListenerAddress = expectedListenerAddress

	serverImpl.Execute([]string{})

	actualListenerAddress1, chok1 := <-actualListenerAddressChan
	s.True(chok1)
	actualListenerAddress2, _ := <-actualListenerAddressChan
	_, chok2 := <-actualListenerAddressChan

	s.False(chok2)

	s.Equal(fmt.Sprintf("http://%s:8080-1", expectedListenerAddress), actualListenerAddress1)
	s.Equal(fmt.Sprintf("http://%s:8080-2", expectedListenerAddress), actualListenerAddress2)
}

func (s *ServerTestSuite) Test_Execute_RepeatsContactingSwarmListenerAddress() {
	defer os.Unsetenv("REPEAT_RELOAD")
	os.Setenv("REPEAT_RELOAD", "true")
	expectedListenerAddress := "swarm-listener"
	actualListenerAddressChan := make(chan string)
	callNum := 0
	defer MockFetch(FetchMock{
		ReloadConfigMock: func(baseData actions.BaseReconfigure, listenerAddr string) error {
			callNum = callNum + 1
			if callNum <= 2 {
				actualListenerAddressChan <- fmt.Sprintf("%s-%d", listenerAddr, callNum)
				if callNum == 2 {
					close(actualListenerAddressChan)
				}
			}
			return nil
		},
	})()
	defer func() {
		os.Unsetenv("LISTENER_ADDRESS")
	}()
	serverImpl.ListenerAddress = expectedListenerAddress

	serverImpl.Execute([]string{})

	actualListenerAddress1, chok1 := <-actualListenerAddressChan
	s.True(chok1)
	actualListenerAddress2, _ := <-actualListenerAddressChan
	_, chok2 := <-actualListenerAddressChan

	s.False(chok2)

	s.Equal(fmt.Sprintf("http://%s:8080-1", expectedListenerAddress), actualListenerAddress1)
	s.Equal(fmt.Sprintf("http://%s:8080-2", expectedListenerAddress), actualListenerAddress2)
}

// CertPutHandler

func (s *ServerTestSuite) Test_CertPutHandler_InvokesCertPut_WhenUrlIsCert() {
	invoked := false
	certOrig := cert
	defer func() { cert = certOrig }()
	cert = CertMock{
		PutMock: func(http.ResponseWriter, *http.Request) (string, error) {
			invoked = true
			return "", nil
		},
	}
	req, _ := http.NewRequest("PUT", s.CertUrl, nil)

	srv := serve{}
	srv.certPutHandler(s.ResponseWriter, req)

	s.Assert().True(invoked)
}

// CertsHandler

func (s *ServerTestSuite) Test_CertsHandler_InvokesCertGetAll_WhenUrlIsCerts() {
	invoked := false
	certOrig := cert
	defer func() { cert = certOrig }()
	cert = CertMock{
		GetAllMock: func(http.ResponseWriter, *http.Request) (server.CertResponse, error) {
			invoked = true
			return server.CertResponse{}, nil
		},
	}
	req, _ := http.NewRequest("GET", s.CertsUrl, nil)

	srv := serve{}
	srv.certsHandler(s.ResponseWriter, req)

	s.Assert().True(invoked)
}

// Mock

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

type CertMock struct {
	PutMock     func(http.ResponseWriter, *http.Request) (string, error)
	PutCertMock func(certName string, certContent []byte) (string, error)
	GetAllMock  func(w http.ResponseWriter, req *http.Request) (server.CertResponse, error)
	GetInitMock func() error
}

func (m CertMock) Put(w http.ResponseWriter, req *http.Request) (string, error) {
	return m.PutMock(w, req)
}

func (m CertMock) PutCert(certName string, certContent []byte) (string, error) {
	return m.PutCertMock(certName, certContent)
}

func (m CertMock) GetAll(w http.ResponseWriter, req *http.Request) (server.CertResponse, error) {
	return m.GetAllMock(w, req)
}

func (m CertMock) Init() error {
	return m.GetInitMock()
}

type ServerMock struct {
	GetServicesFromEnvVarsMock func() *[]proxy.Service
	GetServiceFromUrlMock      func(req *http.Request) *proxy.Service
	PingHandlerMock            func(w http.ResponseWriter, req *http.Request)
	ReconfigureHandlerMock     func(w http.ResponseWriter, req *http.Request)
	ReloadHandlerMock          func(w http.ResponseWriter, req *http.Request)
	RemoveHandlerMock          func(w http.ResponseWriter, req *http.Request)
	Test1HandlerMock           func(w http.ResponseWriter, req *http.Request)
	Test2HandlerMock           func(w http.ResponseWriter, req *http.Request)
}

func MockServer(mock ServerMock) func() {
	newServerOrig := server.NewServer
	server.NewServer = func(listenerAddr, port, serviceName, configsPath, templatesPath string, cert server.Certer) server.Server {
		return mock
	}
	return func() { server.NewServer = newServerOrig }
}

func (m ServerMock) GetServiceFromUrl(req *http.Request) *proxy.Service {
	return m.GetServiceFromUrlMock(req)
}

func (m ServerMock) Test1Handler(w http.ResponseWriter, req *http.Request) {
	m.Test1HandlerMock(w, req)
}

func (m ServerMock) Test2Handler(w http.ResponseWriter, req *http.Request) {
	m.Test2HandlerMock(w, req)
}

func (m ServerMock) ReconfigureHandler(w http.ResponseWriter, req *http.Request) {
	m.ReconfigureHandlerMock(w, req)
}

func (m ServerMock) PingHandler(w http.ResponseWriter, req *http.Request) {
	m.PingHandlerMock(w, req)
}

func (m ServerMock) ReloadHandler(w http.ResponseWriter, req *http.Request) {
	m.ReloadHandlerMock(w, req)
}

func (m ServerMock) RemoveHandler(w http.ResponseWriter, req *http.Request) {
	m.RemoveHandlerMock(w, req)
}

func (m ServerMock) GetServicesFromEnvVars() *[]proxy.Service {
	return m.GetServicesFromEnvVarsMock()
}

type RunMock struct {
	mock.Mock
}

func (m *RunMock) Execute(args []string) error {
	params := m.Called(args)
	return params.Error(0)
}

func getRunMock(skipMethod string) *RunMock {
	mockObj := new(RunMock)
	if skipMethod != "Execute" {
		mockObj.On("Execute", mock.Anything).Return(nil)
	}
	return mockObj
}

type ReconfigureMock struct {
	ExecuteMock      func(reloadAfter bool) error
	GetDataMock      func() (actions.BaseReconfigure, proxy.Service)
	GetTemplatesMock func() (front, back string, err error)
}

func MockReconfigure(mock ReconfigureMock) func() {
	newReconfigureOrig := actions.NewReconfigure
	actions.NewReconfigure = func(baseData actions.BaseReconfigure, serviceData proxy.Service) actions.Reconfigurable {
		return mock
	}
	return func() { actions.NewReconfigure = newReconfigureOrig }
}

func (m ReconfigureMock) Execute(reloadAfter bool) error {
	return m.ExecuteMock(reloadAfter)
}

func (m ReconfigureMock) GetData() (actions.BaseReconfigure, proxy.Service) {
	return m.GetDataMock()
}

func (m ReconfigureMock) GetTemplates() (front, back string, err error) {
	return m.GetTemplatesMock()
}

type ReloadMock struct {
	ExecuteMock func(recreate bool) error
}

func (m ReloadMock) Execute(recreate bool) error {
	return m.ExecuteMock(recreate)
}

type FetchMock struct {
	ReloadClusterConfigMock func(listenerAddr string) error
	ReloadConfigMock        func(baseData actions.BaseReconfigure, listenerAddr string) error
}

func (m FetchMock) ReloadClusterConfig(listenerAddr string) error {
	return m.ReloadClusterConfigMock(listenerAddr)
}
func (m FetchMock) ReloadConfig(baseData actions.BaseReconfigure, listenerAddr string) error {
	return m.ReloadConfigMock(baseData, listenerAddr)
}
func MockFetch(mock FetchMock) func() {
	newFetchOrig := actions.NewFetch
	actions.NewFetch = func(baseData actions.BaseReconfigure) actions.Fetchable {
		return &mock
	}
	return func() {
		actions.NewFetch = newFetchOrig
	}
}
