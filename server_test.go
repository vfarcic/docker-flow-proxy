// +build !integration

package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"./actions"
	"./proxy"
	"./server"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"time"
)

type ServerTestSuite struct {
	suite.Suite
	proxy.Service
	ConsulAddress      string
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

func (s *ServerTestSuite) SetupTest() {
	s.sd = proxy.ServiceDest{
		ServicePath: []string{"/path/to/my/service/api", "/path/to/my/other/service/api"},
	}
	s.Service.ServiceDest = []proxy.ServiceDest{s.sd}
	s.InstanceName = "proxy-test-instance"
	s.ConsulAddress = "http://1.2.3.4:1234"
	s.ServiceName = "myService"
	s.ServiceColor = "pink"
	s.ServiceDomain = []string{"my-domain.com"}
	s.OutboundHostname = "machine-123.my-company.com"
	s.BaseUrl = "/v1/docker-flow-proxy"
	s.ReconfigureBaseUrl = fmt.Sprintf("%s/reconfigure", s.BaseUrl)
	s.RemoveBaseUrl = fmt.Sprintf("%s/remove", s.BaseUrl)
	s.ReconfigureUrl = fmt.Sprintf(
		"%s?serviceName=%s&serviceColor=%s&servicePath=%s&serviceDomain=%s&outboundHostname=%s",
		s.ReconfigureBaseUrl,
		s.ServiceName,
		s.ServiceColor,
		strings.Join(s.sd.ServicePath, ","),
		strings.Join(s.ServiceDomain, ","),
		s.OutboundHostname,
	)
	s.ReqMode = "http"
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
	serverImpl = Serve{
		BaseReconfigure: actions.BaseReconfigure{
			ConsulAddresses: []string{s.ConsulAddress},
			InstanceName:    s.InstanceName,
		},
	}
	logPrintfOrig := logPrintf
	defer func() { logPrintf = logPrintfOrig }()
	logPrintf = func(format string, v ...interface{}) {}
}

// Execute

func (s *ServerTestSuite) Test_Execute_InvokesHTTPListenAndServe() {
	serverImpl := Serve{
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
	orig := NewRun
	defer func() {
		NewRun = orig
	}()
	mockObj := getRunMock("")
	NewRun = func() Executable {
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

func (s *ServerTestSuite) Test_Execute_InvokesReloadAllServices() {
	actualAddresses := []string{}
	actualInstanceName := ""
	defer MockFetch(FetchMock{
		ReloadServicesFromRegistryMock: func(addresses []string, instanceName, mode string) error {
			actualAddresses = addresses
			actualInstanceName = instanceName
			return nil
		},
	})()

	consulAddressesOrig := []string{s.ConsulAddress}
	defer func() {
		os.Unsetenv("CONSUL_ADDRESS")
		serverImpl.ConsulAddresses = consulAddressesOrig
	}()
	os.Setenv("CONSUL_ADDRESS", s.ConsulAddress)

	serverImpl.Execute([]string{})

	s.Equal([]string{s.ConsulAddress}, actualAddresses)
	s.Equal(s.InstanceName, actualInstanceName)
}

func (s *ServerTestSuite) Test_Execute_InvokesReconfigureExecuteForEachServiceDefinedInEnvVars() {
	called := 0
	defer MockServer(ServerMock{
		GetServicesFromEnvVarsMock: func() *[]proxy.Service { return &[]proxy.Service{{}, {}} },
	})()
	defer MockFetch(FetchMock{
		ReloadServicesFromRegistryMock: func(addresses []string, instanceName, mode string) error { return nil },
	})()
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
	reloadFromRegistryCalled := false
	actualListenerAddressChan := make(chan string)
	defer MockReconfigure(ReconfigureMock{
		ExecuteMock: func(reloadAfter bool) error {
			return nil
		},
	})()
	defer MockFetch(FetchMock{
		ReloadServicesFromRegistryMock: func(addresses []string, instanceName, mode string) error {
			reloadFromRegistryCalled = true
			return nil
		},
		ReloadConfigMock: func(baseData actions.BaseReconfigure, mode string, listenerAddr string) error {
			actualListenerAddressChan <- listenerAddr
			return nil
		},
	})()
	consulAddressesOrig := []string{s.ConsulAddress}
	defer func() {
		os.Unsetenv("CONSUL_ADDRESS")
		os.Unsetenv("LISTENER_ADDRESS")
		serverImpl.ConsulAddresses = consulAddressesOrig
	}()
	os.Setenv("CONSUL_ADDRESS", s.ConsulAddress)
	serverImpl.ListenerAddress = expectedListenerAddress

	serverImpl.Execute([]string{})

	actualListenerAddress, _ := <-actualListenerAddressChan
	close(actualListenerAddressChan)
	s.Equal(fmt.Sprintf("http://%s:8080", expectedListenerAddress), actualListenerAddress)
}

func (s *ServerTestSuite) Test_Execute_RetriesContactingSwarmListenerAddress() {
	expectedListenerAddress := "swarm-listener"
	actualListenerAddressChan := make(chan string)
	callNum := 0
	defer MockFetch(FetchMock{
		ReloadServicesFromRegistryMock: func(addresses []string, instanceName, mode string) error {
			return nil
		},
		ReloadConfigMock: func(baseData actions.BaseReconfigure, mode string, listenerAddr string) error {
			callNum = callNum + 1
			actualListenerAddressChan <- fmt.Sprintf("%s-%d", listenerAddr, callNum)
			if callNum == 2 {
				close(actualListenerAddressChan)
				return nil
			} else {
				return fmt.Errorf("On iteration %d", callNum)
			}
		},
	})()
	consulAddressesOrig := []string{s.ConsulAddress}
	defer func() {
		os.Unsetenv("CONSUL_ADDRESS")
		os.Unsetenv("LISTENER_ADDRESS")
		serverImpl.ConsulAddresses = consulAddressesOrig
	}()
	os.Setenv("CONSUL_ADDRESS", s.ConsulAddress)
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

func (s *ServerTestSuite) Test_Execute_ReturnsError_WhenReloadAllServicesFails() {
	defer MockFetch(FetchMock{
		ReloadServicesFromRegistryMock: func(addresses []string, instanceName, mode string) error {
			return fmt.Errorf("This is an error")
		},
	})()
	actual := serverImpl.Execute([]string{})
	s.Error(actual)
}

func (s *ServerTestSuite) Test_Execute_SetsConsulAddressesToEmptySlice_WhenEnvVarIsNotset() {
	srv := Serve{}

	srv.Execute([]string{})

	s.Equal([]string{}, srv.ConsulAddresses)
}

func (s *ServerTestSuite) Test_Execute_SetsConsulAddresses() {
	expected := "http://my-consul"
	consulAddressesOrig := serverImpl.ConsulAddresses
	defer func() {
		os.Unsetenv("CONSUL_ADDRESS")
		serverImpl.ConsulAddresses = consulAddressesOrig
	}()
	os.Setenv("CONSUL_ADDRESS", expected)
	srv := Serve{}

	srv.Execute([]string{})

	s.Equal([]string{expected}, srv.ConsulAddresses)
}

func (s *ServerTestSuite) Test_Execute_SetsMultipleConsulAddresseses() {
	expected := []string{"http://my-consul-1", "http://my-consul-2"}
	consulAddressesOrig := serverImpl.ConsulAddresses
	defer func() {
		os.Unsetenv("CONSUL_ADDRESS")
		serverImpl.ConsulAddresses = consulAddressesOrig
	}()
	os.Setenv("CONSUL_ADDRESS", strings.Join(expected, ","))
	srv := Serve{}

	srv.Execute([]string{})

	s.Equal(expected, srv.ConsulAddresses)
}

func (s *ServerTestSuite) Test_Execute_AddsHttpToConsulAddresses() {
	expected := []string{"http://my-consul-1", "http://my-consul-2"}
	consulAddressesOrig := serverImpl.ConsulAddresses
	defer func() {
		os.Unsetenv("CONSUL_ADDRESS")
		serverImpl.ConsulAddresses = consulAddressesOrig
	}()
	os.Setenv("CONSUL_ADDRESS", "my-consul-1,my-consul-2")
	srv := Serve{}

	srv.Execute([]string{})

	s.Equal(expected, srv.ConsulAddresses)
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

	srv := Serve{}
	srv.CertPutHandler(s.ResponseWriter, req)

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

	srv := Serve{}
	srv.CertsHandler(s.ResponseWriter, req)

	s.Assert().True(invoked)
}

// ServeHTTP > Config

func (s *ServerTestSuite) Test_ConfigHandler_SetsContentTypeToText_WhenUrlIsConfig() {
	var actual string
	httpWriterSetContentType = func(w http.ResponseWriter, value string) {
		actual = value
	}
	req, _ := http.NewRequest("GET", s.ConfigUrl, nil)

	srv := Serve{}
	srv.ConfigHandler(s.ResponseWriter, req)

	s.Equal("text/html", actual)
}

func (s *ServerTestSuite) Test_ConfigHandler_ReturnsConfig_WhenUrlIsConfig() {
	expected := "some text"
	readFileOrig := proxy.ReadFile
	defer func() { proxy.ReadFile = readFileOrig }()
	proxy.ReadFile = func(filename string) ([]byte, error) {
		return []byte(expected), nil
	}

	req, _ := http.NewRequest("GET", s.ConfigUrl, nil)
	srv := Serve{}
	srv.ConfigHandler(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ConfigHandler_ReturnsStatus500_WhenReadFileFails() {
	readFileOrig := readFile
	defer func() { readFile = readFileOrig }()
	readFile = func(filename string) ([]byte, error) {
		return []byte(""), fmt.Errorf("This is an error")
	}

	req, _ := http.NewRequest("GET", s.ConfigUrl, nil)
	srv := Serve{}
	srv.ConfigHandler(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 500)
}

// Suite

// TODO: Review whether everything is needed
func TestServerUnitTestSuite(t *testing.T) {
	s := new(ServerTestSuite)
	logPrintf = func(format string, v ...interface{}) {}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualPath := r.URL.Path
		if r.Method == "GET" {
			switch actualPath {
			case "/v1/docker-flow-proxy/reconfigure":
				if strings.EqualFold(r.URL.Query().Get("returnError"), "true") {
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					w.WriteHeader(http.StatusOK)
					w.Header().Set("Content-Type", "application/json")
				}
			case "/v1/docker-flow-proxy/remove":
				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "application/json")
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}
	}))
	defer func() { s.Server.Close() }()
	addr := strings.Replace(s.Server.URL, "http://", "", -1)
	s.DnsIps = []string{strings.Split(addr, ":")[0]}

	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		return s.DnsIps, nil
	}
	sd := proxy.ServiceDest{
		Port: strings.Split(addr, ":")[1],
	}
	s.ServiceDest = []proxy.ServiceDest{sd}

	suite.Run(t, s)
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
	GetServiceFromUrlMock      func(req *http.Request) *proxy.Service
	TestHandlerMock            func(w http.ResponseWriter, req *http.Request)
	ReconfigureHandlerMock     func(w http.ResponseWriter, req *http.Request)
	ReloadHandlerMock          func(w http.ResponseWriter, req *http.Request)
	RemoveHandlerMock          func(w http.ResponseWriter, req *http.Request)
	GetServicesFromEnvVarsMock func() *[]proxy.Service
}

func MockServer(mock ServerMock) func() {
	newServerOrig := server.NewServer
	server.NewServer = func(listenerAddr, mode, port, serviceName, configsPath, templatesPath string, consulAddresses []string, cert server.Certer) server.Server {
		return mock
	}
	return func() { server.NewServer = newServerOrig }
}

func (m ServerMock) GetServiceFromUrl(req *http.Request) *proxy.Service {
	return m.GetServiceFromUrlMock(req)
}

func (m ServerMock) TestHandler(w http.ResponseWriter, req *http.Request) {
	m.TestHandlerMock(w, req)
}

func (m ServerMock) ReconfigureHandler(w http.ResponseWriter, req *http.Request) {
	m.ReconfigureHandlerMock(w, req)
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
	actions.NewReconfigure = func(baseData actions.BaseReconfigure, serviceData proxy.Service, mode string) actions.Reconfigurable {
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

func MockReload(mock ReloadMock) func() {
	newFetchOrig := actions.NewReload
	actions.NewReload = func() actions.Reloader {
		return mock
	}
	return func() {
		actions.NewReload = newFetchOrig
	}
}

func (m ReloadMock) Execute(recreate bool) error {
	return m.ExecuteMock(recreate)
}

type FetchMock struct {
	ReloadServicesFromRegistryMock func(addresses []string, instanceName, mode string) error
	ReloadClusterConfigMock        func(listenerAddr string) error
	ReloadConfigMock               func(baseData actions.BaseReconfigure, mode string, listenerAddr string) error
}

func (m *FetchMock) ReloadServicesFromRegistry(addresses []string, instanceName, mode string) error {
	return m.ReloadServicesFromRegistryMock(addresses, instanceName, mode)
}
func (m FetchMock) ReloadClusterConfig(listenerAddr string) error {
	return m.ReloadClusterConfigMock(listenerAddr)
}
func (m FetchMock) ReloadConfig(baseData actions.BaseReconfigure, mode string, listenerAddr string) error {
	return m.ReloadConfigMock(baseData, mode, listenerAddr)
}
func MockFetch(mock FetchMock) func() {
	newFetchOrig := actions.NewFetch
	actions.NewFetch = func(baseData actions.BaseReconfigure, mode string) actions.Fetchable {
		return &mock
	}
	return func() {
		actions.NewFetch = newFetchOrig
	}
}
