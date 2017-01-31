// +build !integration

package main

import (
	"encoding/json"
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
	actions.NewReconfigure = func(baseData actions.BaseReconfigure, serviceData proxy.Service, mode string) actions.Reconfigurable {
		return getReconfigureMock("")
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
	mockObj := getReconfigureMock("")
	actions.NewReconfigure = func(baseData actions.BaseReconfigure, serviceData proxy.Service, mode string) actions.Reconfigurable {
		return mockObj
	}
	consulAddressesOrig := []string{s.ConsulAddress}
	defer func() {
		os.Unsetenv("CONSUL_ADDRESS")
		serverImpl.ConsulAddresses = consulAddressesOrig
	}()
	os.Setenv("CONSUL_ADDRESS", s.ConsulAddress)

	serverImpl.Execute([]string{})

	mockObj.AssertCalled(s.T(), "ReloadAllServices", []string{s.ConsulAddress}, s.InstanceName, "", "")
}

func (s *ServerTestSuite) Test_Execute_InvokesReloadAllServicesWithListenerAddress() {
	listenerAddress := "swarm-listener"
	mockObj := getReconfigureMock("")
	actions.NewReconfigure = func(baseData actions.BaseReconfigure, serviceData proxy.Service, mode string) actions.Reconfigurable {
		return mockObj
	}
	consulAddressesOrig := []string{s.ConsulAddress}
	defer func() {
		os.Unsetenv("CONSUL_ADDRESS")
		os.Unsetenv("LISTENER_ADDRESS")
		serverImpl.ConsulAddresses = consulAddressesOrig
	}()
	os.Setenv("CONSUL_ADDRESS", s.ConsulAddress)
	serverImpl.ListenerAddress = listenerAddress

	serverImpl.Execute([]string{})

	mockObj.AssertCalled(
		s.T(),
		"ReloadAllServices",
		[]string{s.ConsulAddress},
		s.InstanceName,
		"",
		fmt.Sprintf("http://%s:8080", listenerAddress),
	)
}

func (s *ServerTestSuite) Test_Execute_DoesNotInvokeReloadAllServices_WhenModeIsService() {
	serverImpl.Mode = "seRviCe"
	mockObj := getReconfigureMock("")
	actions.NewReconfigure = func(baseData actions.BaseReconfigure, serviceData proxy.Service, mode string) actions.Reconfigurable {
		return mockObj
	}

	serverImpl.Execute([]string{})

	mockObj.AssertNotCalled(s.T(), "ReloadAllServices", s.ConsulAddress, s.InstanceName, "")
}

func (s *ServerTestSuite) Test_Execute_DoesNotInvokeReloadAllServices_WhenModeIsSwarm() {
	serverImpl.Mode = "SWarM"
	mockObj := getReconfigureMock("")
	actions.NewReconfigure = func(baseData actions.BaseReconfigure, serviceData proxy.Service, mode string) actions.Reconfigurable {
		return mockObj
	}

	serverImpl.Execute([]string{})

	mockObj.AssertNotCalled(s.T(), "ReloadAllServices", s.ConsulAddress, s.InstanceName, "")
}

func (s *ServerTestSuite) Test_Execute_ReturnsError_WhenReloadAllServicesFails() {
	mockObj := getReconfigureMock("ReloadAllServices")
	mockObj.On("ReloadAllServices", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("This is an error"))
	actions.NewReconfigure = func(baseData actions.BaseReconfigure, serviceData proxy.Service, mode string) actions.Reconfigurable {
		return mockObj
	}

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

// ServeHTTP

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatus404WhenURLIsUnknown() {
	req, _ := http.NewRequest("GET", "/this/url/does/not/exist", nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 404)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatus200WhenUrlIsTest() {
	for ver := 1; ver <= 2; ver++ {
		rw := getResponseWriterMock()
		req, _ := http.NewRequest("GET", fmt.Sprintf("/v%d/test", ver), nil)

		srv := Serve{}
		srv.ServeHTTP(rw, req)

		rw.AssertCalled(s.T(), "WriteHeader", 200)
	}
}

// ServeHTTP > Cert

func (s *ServerTestSuite) Test_ServeHTTP_InvokesCertPut_WhenUrlIsCert() {
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
	srv.ServeHTTP(s.ResponseWriter, req)

	s.Assert().True(invoked)
}

func (s *ServerTestSuite) Test_ServeHTTP_DoesNotInvoke_WhenUrlIsCertAndMethodIsNotPut() {
	invoked := false
	certOrig := cert
	defer func() { cert = certOrig }()
	cert = CertMock{
		PutMock: func(http.ResponseWriter, *http.Request) (string, error) {
			invoked = true
			return "", nil
		},
	}
	req, _ := http.NewRequest("GET", s.CertUrl, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.Assert().False(invoked)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatusNotFound_WhenUrlIsCertAndMethodIsNotPut() {
	req, _ := http.NewRequest("GET", s.CertUrl, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 404)
}

// ServeHTTP > Certs

func (s *ServerTestSuite) Test_ServeHTTP_InvokesCertGetAll_WhenUrlIsCerts() {
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
	srv.ServeHTTP(s.ResponseWriter, req)

	s.Assert().True(invoked)
}

// ServeHTTP > Reload

func (s *ServerTestSuite) Test_ServeHTTP_InvokesReload_WhenUrlIsReload() {
	invoked := false
	reloadOrig := reload
	defer func() { reload = reloadOrig }()
	reload = ReloadMock{
		ExecuteMock: func() error {
			invoked = true
			return nil
		},
	}
	url := fmt.Sprintf("%s/reload", s.BaseUrl)
	req, _ := http.NewRequest("GET", url, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.Assert().True(invoked)
}

// ServeHTTP > Reconfigure

func (s *ServerTestSuite) Test_ServeHTTP_SetsContentTypeToJSON_WhenUrlIsReconfigure() {
	var actual string
	httpWriterSetContentType = func(w http.ResponseWriter, value string) {
		actual = value
	}
	req, _ := http.NewRequest("GET", s.ReconfigureUrl, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.Equal("application/json", actual)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJSON_WhenUrlIsReconfigure() {
	expected, _ := json.Marshal(server.Response{
		Status: "OK",
		Service: proxy.Service{
			ReqMode:          "http",
			ServiceColor:     s.ServiceColor,
			ServiceDomain:    s.ServiceDomain,
			OutboundHostname: s.OutboundHostname,
			PathType:         s.PathType,
			ServiceDest:      []proxy.ServiceDest{s.sd},
			ServiceName:      s.ServiceName,
		},
		ServiceName: s.ServiceName,
	})

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, s.RequestReconfigure)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJSONWithAllPortsAndPaths() {
	sd := []proxy.ServiceDest{
		proxy.ServiceDest{
			ServicePath: []string{"/path/to/my-service"},
			Port:        "1111",
			SrcPort:     2222,
		},
		proxy.ServiceDest{
			ServicePath: []string{"/path/to/my-service-1"},
			Port:        "3333",
			SrcPort:     4444,
		},
		proxy.ServiceDest{
			ServicePath: []string{"/path/to/my-service-2"},
			Port:        "4444",
		},
	}
	expected, _ := json.Marshal(server.Response{
		Status: "OK",
		Service: proxy.Service{
			ReqMode:     "http",
			PathType:    s.PathType,
			ServiceDest: sd,
			ServiceName: s.ServiceName,
		},
		ServiceName: s.ServiceName,
	})
	addr := fmt.Sprintf(
		"%s?serviceName=%s&servicePath.1=%s&port.1=%s&srcPort.1=%d&servicePath.2=%s&port.2=%s&srcPort.2=%d&servicePath.3=%s&port.3=%s",
		s.ReconfigureBaseUrl,
		s.ServiceName,
		strings.Join(sd[0].ServicePath, ","),
		sd[0].Port,
		sd[0].SrcPort,
		strings.Join(sd[1].ServicePath, ","),
		sd[1].Port,
		sd[1].SrcPort,
		strings.Join(sd[2].ServicePath, ","),
		sd[2].Port,
	)
	req, _ := http.NewRequest("GET", addr, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJsonWithPathType_WhenPresent() {
	pathType := "path_reg"
	req, _ := http.NewRequest("GET", s.ReconfigureUrl+"&pathType="+pathType, nil)
	expected, _ := json.Marshal(server.Response{
		Status:      "OK",
		ServiceName: s.ServiceName,
		Service: proxy.Service{
			ReqMode:          "http",
			ServiceName:      s.ServiceName,
			ServiceColor:     s.ServiceColor,
			ServiceDomain:    s.ServiceDomain,
			OutboundHostname: s.OutboundHostname,
			PathType:         pathType,
			ServiceDest:      []proxy.ServiceDest{s.sd},
		},
	})

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

// TODO: Deprecated (dec. 2016).
func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJsonWithReqRep_WhenPresent() {
	search := "search"
	replace := "replace"
	url := fmt.Sprintf(
		s.ReconfigureUrl + "&reqRepSearch=" + search + "&reqRepReplace=" + replace,
	)
	req, _ := http.NewRequest("GET", url, nil)
	expected, _ := json.Marshal(server.Response{
		Status:      "OK",
		ServiceName: s.ServiceName,
		Service: proxy.Service{
			ReqMode:          "http",
			ServiceName:      s.ServiceName,
			ServiceColor:     s.ServiceColor,
			ServiceDomain:    s.ServiceDomain,
			OutboundHostname: s.OutboundHostname,
			ReqRepSearch:     search,
			ReqRepReplace:    replace,
			ServiceDest:      []proxy.ServiceDest{s.sd},
		},
	})

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJsonWithReqPath_WhenPresent() {
	search := "search"
	replace := "replace"
	url := fmt.Sprintf(
		s.ReconfigureUrl + "&reqPathSearch=" + search + "&reqPathReplace=" + replace,
	)
	req, _ := http.NewRequest("GET", url, nil)
	expected, _ := json.Marshal(server.Response{
		Status:      "OK",
		ServiceName: s.ServiceName,
		Service: proxy.Service{
			ReqMode:          "http",
			ServiceName:      s.ServiceName,
			ServiceColor:     s.ServiceColor,
			ServiceDomain:    s.ServiceDomain,
			OutboundHostname: s.OutboundHostname,
			ReqPathSearch:    search,
			ReqPathReplace:   replace,
			ServiceDest:      []proxy.ServiceDest{s.sd},
		},
	})

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJsonWithMode_WhenPresent() {
	search := "search"
	replace := "replace"
	url := fmt.Sprintf(
		s.ReconfigureUrl + "&reqPathSearch=" + search + "&reqPathReplace=" + replace + "&reqMode=tcp&srcPort=1234&port=4321",
	)
	req, _ := http.NewRequest("GET", url, nil)
	expected, _ := json.Marshal(server.Response{
		ServiceName: s.ServiceName,
		Status:      "OK",
		Service: proxy.Service{
			ServiceName:      s.ServiceName,
			ReqMode:          "tcp",
			ServiceColor:     s.ServiceColor,
			ServiceDomain:    s.ServiceDomain,
			OutboundHostname: s.OutboundHostname,
			ReqPathSearch:    search,
			ReqPathReplace:   replace,
			ServiceDest: []proxy.ServiceDest{
				proxy.ServiceDest{
					ServicePath: []string{"/path/to/my/service/api", "/path/to/my/other/service/api"},
					SrcPort:     1234,
					Port:        "4321",
				},
			},
		},
	})

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJsonWithTemplatePaths_WhenPresent() {
	templateFePath := "something"
	templateBePath := "else"
	url := fmt.Sprintf(
		"%s&templateFePath=%s&templateBePath=%s",
		s.ReconfigureUrl,
		templateFePath,
		templateBePath,
	)
	req, _ := http.NewRequest("GET", url, nil)
	expected, _ := json.Marshal(server.Response{
		ServiceName: s.ServiceName,
		Status:      "OK",
		Service: proxy.Service{
			ServiceName:      s.ServiceName,
			ReqMode:          "http",
			ServiceColor:     s.ServiceColor,
			ServiceDomain:    s.ServiceDomain,
			TemplateFePath:   templateFePath,
			TemplateBePath:   templateBePath,
			OutboundHostname: s.OutboundHostname,
			ServiceDest:      []proxy.ServiceDest{s.sd},
		},
	})

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJsonWithUsers_WhenPresent() {
	users := []proxy.User{
		{Username: "user1", Password: "pass1"},
		{Username: "user2", Password: "pass2"},
	}
	req, _ := http.NewRequest("GET", s.ReconfigureUrl+"&users=user1:pass1,user2:pass2", nil)
	expected, _ := json.Marshal(server.Response{
		Status:      "OK",
		ServiceName: s.ServiceName,
		Service: proxy.Service{
			ReqMode:          "http",
			ServiceName:      s.ServiceName,
			ServiceColor:     s.ServiceColor,
			ServiceDomain:    s.ServiceDomain,
			OutboundHostname: s.OutboundHostname,
			Users:            users,
			ServiceDest:      []proxy.ServiceDest{s.sd},
		},
	})

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJsonWithPorts_WhenPresent() {
	port := "1234"
	httpsPort := 4321
	mode := "swaRM"
	address := fmt.Sprintf(
		"%s&port=%s&httpsPort=%d&httpsOnly=true",
		s.ReconfigureUrl,
		port,
		httpsPort,
	)
	req, _ := http.NewRequest("GET", address, nil)
	sd := proxy.ServiceDest{
		ServicePath: s.sd.ServicePath,
		Port:        port,
	}
	expected, _ := json.Marshal(server.Response{
		Status:      "OK",
		ServiceName: s.ServiceName,
		Mode:        mode,
		Service: proxy.Service{
			ServiceName:      s.ServiceName,
			ReqMode:          "http",
			ServiceColor:     s.ServiceColor,
			ServiceDomain:    s.ServiceDomain,
			OutboundHostname: s.OutboundHostname,
			HttpsOnly:        true,
			HttpsPort:        httpsPort,
			ServiceDest:      []proxy.ServiceDest{sd},
		},
	})

	srv := Serve{Mode: mode}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJsonWithSkipCheck_WhenPresent() {
	req, _ := http.NewRequest("GET", s.ReconfigureUrl+"&skipCheck=true", nil)
	expected, _ := json.Marshal(server.Response{
		Status:      "OK",
		ServiceName: s.ServiceName,
		Service: proxy.Service{
			ServiceName:      s.ServiceName,
			ReqMode:          "http",
			ServiceColor:     s.ServiceColor,
			ServiceDomain:    s.ServiceDomain,
			OutboundHostname: s.OutboundHostname,
			PathType:         s.PathType,
			SkipCheck:        true,
			ServiceDest:      []proxy.ServiceDest{s.sd},
		},
	})

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJsonTimeoutServer_WhenPresent() {
	req, _ := http.NewRequest("GET", s.ReconfigureUrl+"&timeoutServer=9999", nil)
	expected, _ := json.Marshal(server.Response{
		Status:      "OK",
		ServiceName: s.ServiceName,
		Service: proxy.Service{
			ServiceName:      s.ServiceName,
			ReqMode:          "http",
			ServiceColor:     s.ServiceColor,
			ServiceDomain:    s.ServiceDomain,
			OutboundHostname: s.OutboundHostname,
			ServiceDest:      []proxy.ServiceDest{s.sd},
			TimeoutServer:    "9999",
		},
	})

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJsonTimeoutTunnel_WhenPresent() {
	req, _ := http.NewRequest("GET", s.ReconfigureUrl+"&timeoutTunnel=9999", nil)
	expected, _ := json.Marshal(server.Response{
		Status:      "OK",
		ServiceName: s.ServiceName,
		Service: proxy.Service{
			ServiceName:      s.ServiceName,
			ReqMode:          "http",
			ServiceColor:     s.ServiceColor,
			ServiceDomain:    s.ServiceDomain,
			OutboundHostname: s.OutboundHostname,
			ServiceDest:      []proxy.ServiceDest{s.sd},
			TimeoutTunnel:    "9999",
		},
	})

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_WritesErrorHeader_WhenReconfigureDistributeIsTrueAndError() {
	serve := Serve{}
	serve.Port = s.ServiceDest[0].Port
	addr := fmt.Sprintf("http://127.0.0.1:8080%s&distribute=true&returnError=true", s.ReconfigureUrl)
	req, _ := http.NewRequest("GET", addr, nil)

	serve.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 500)
}

func (s *ServerTestSuite) Test_ServeHTTP_WritesErrorHeader_WhenRemoveDistributeIsTrueAndError() {
	serve := Serve{}
	serve.Port = s.ServiceDest[0].Port
	addr := fmt.Sprintf("http://127.0.0.1:8080%s&distribute=true&returnError=true", s.RemoveUrl)
	req, _ := http.NewRequest("GET", addr, nil)

	serve.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 500)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatus400_WhenUrlIsReconfigureAndServiceNameQueryIsNotPresent() {
	req, _ := http.NewRequest("GET", s.ReconfigureBaseUrl, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 400)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatus200_WhenUrlIsReconfigureAndReqModeIsTcp() {
	addr := fmt.Sprintf("%s?serviceName=redis&port=6379&srcPort=6379&reqMode=tcp", s.ReconfigureBaseUrl)
	req, _ := http.NewRequest("GET", addr, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 200)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatus400_WhenUrlIsReconfigureAndReqModeIsTcpAndSrcPortIsNotPresent() {
	addr := fmt.Sprintf("%s?serviceName=redis&port=6379&reqMode=tcp", s.ReconfigureBaseUrl)
	req, _ := http.NewRequest("GET", addr, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 400)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatus400_WhenUrlIsReconfigureAndReqModeIsTcpAndPortIsNotPresent() {
	addr := fmt.Sprintf("%s?serviceName=redis&srcPort=6379&reqMode=tcp", s.ReconfigureBaseUrl)
	req, _ := http.NewRequest("GET", addr, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 400)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatus400_WhenServicePathQueryIsNotPresent() {
	url := fmt.Sprintf("%s?serviceName=my-service", s.ReconfigureBaseUrl)
	req, _ := http.NewRequest("GET", url, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 400)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatus400_WhenModeIsServiceAndPortIsNotPresent() {
	req, _ := http.NewRequest("GET", s.ReconfigureUrl, nil)

	srv := Serve{Mode: "service"}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 400)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatus400_WhenModeIsSwarmAndPortIsNotPresent() {
	req, _ := http.NewRequest("GET", s.ReconfigureUrl, nil)

	srv := Serve{Mode: "swARM"}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 400)
}

func (s *ServerTestSuite) Test_ServeHTTP_InvokesReconfigureExecute() {
	s.Service.AclName = "my-acl"
	url := fmt.Sprintf("%s&aclName=my-acl", s.ReconfigureUrl)
	req, _ := http.NewRequest("GET", url, nil)

	s.invokesReconfigure(req, true)
}

func (s *ServerTestSuite) Test_ServeHTTP_DoesNotInvokeReconfigureExecute_WhenDistributeIsTrue() {
	req, _ := http.NewRequest(
		"GET",
		fmt.Sprintf("%s&distribute=true", s.ReconfigureUrl),
		nil,
	)
	s.invokesReconfigure(req, false)
}

func (s *ServerTestSuite) Test_ServeHTTP_DoesNotInvokeRemoveExecute_WhenDistributeIsTrue() {
	req, _ := http.NewRequest(
		"GET",
		fmt.Sprintf("%s&distribute=true", s.RemoveUrl),
		nil,
	)
	s.invokesReconfigure(req, false)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatus500_WhenReconfigureExecuteFails() {
	mockObj := getReconfigureMock("Execute")
	mockObj.On("Execute", []string{}).Return(fmt.Errorf("This is an error"))
	actions.NewReconfigure = func(baseData actions.BaseReconfigure, serviceData proxy.Service, mode string) actions.Reconfigurable {
		return mockObj
	}

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, s.RequestReconfigure)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 500)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJson_WhenConsulTemplatePathIsPresent() {
	pathFe := "/path/to/consul/fe/template"
	pathBe := "/path/to/consul/fe/template"
	address := fmt.Sprintf(
		"%s?serviceName=%s&consulTemplateFePath=%s&consulTemplateBePath=%s",
		s.ReconfigureBaseUrl,
		s.ServiceName,
		pathFe,
		pathBe)
	req, _ := http.NewRequest("GET", address, nil)
	sd := proxy.ServiceDest{
		ServicePath: []string{},
	}
	expected, _ := json.Marshal(server.Response{
		Status:      "OK",
		ServiceName: s.ServiceName,
		Service: proxy.Service{
			ReqMode:              "http",
			ServiceName:          s.ServiceName,
			ConsulTemplateFePath: pathFe,
			ConsulTemplateBePath: pathBe,
			PathType:             s.PathType,
			ServiceDest:          []proxy.ServiceDest{sd},
		},
	})

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_InvokesReconfigureExecute_WhenConsulTemplatePathIsPresent() {
	sd := proxy.ServiceDest{
		ServicePath: []string{},
	}
	pathFe := "/path/to/consul/fe/template"
	pathBe := "/path/to/consul/be/template"
	mockObj := getReconfigureMock("")
	var actualBase actions.BaseReconfigure
	expectedBase := actions.BaseReconfigure{
		ConsulAddresses: []string{s.ConsulAddress},
	}
	expectedService := proxy.Service{
		ServiceName:          s.ServiceName,
		ConsulTemplateFePath: pathFe,
		ConsulTemplateBePath: pathBe,
		PathType:             s.PathType,
		ReqMode:              "http",
		ServiceDest:          []proxy.ServiceDest{sd},
	}
	var actualService proxy.Service
	actions.NewReconfigure = func(baseData actions.BaseReconfigure, serviceData proxy.Service, mode string) actions.Reconfigurable {
		actualBase = baseData
		actualService = serviceData
		return mockObj
	}
	serverImpl := Serve{BaseReconfigure: expectedBase}
	address := fmt.Sprintf(
		"%s?serviceName=%s&consulTemplateFePath=%s&consulTemplateBePath=%s",
		s.ReconfigureBaseUrl,
		s.ServiceName,
		pathFe,
		pathBe)
	req, _ := http.NewRequest("GET", address, nil)

	serverImpl.ServeHTTP(s.ResponseWriter, req)

	s.Equal(expectedBase, actualBase)
	s.Equal(expectedService, actualService)
	mockObj.AssertCalled(s.T(), "Execute", []string{})
}

func (s *ServerTestSuite) Test_ServeHTTP_InvokesPutCert_WhenServiceCertIsPresent() {
	actualCertName := ""
	expectedCert := "my-cert with new line \\n"
	actualCert := ""
	certOrig := cert
	defer func() { cert = certOrig }()
	cert = CertMock{
		PutCertMock: func(certName string, certContent []byte) (string, error) {
			actualCertName = certName
			actualCert = string(certContent[:])
			return "", nil
		},
	}
	address := fmt.Sprintf(
		"%s?serviceName=%s&servicePath=%s&serviceCert=%s",
		s.ReconfigureBaseUrl,
		s.ServiceName,
		strings.Join(s.sd.ServicePath, ","),
		expectedCert,
	)
	req, _ := http.NewRequest("GET", address, nil)

	serverImpl.ServeHTTP(s.ResponseWriter, req)

	s.Equal(s.ServiceName, actualCertName)
	s.Equal(strings.Replace(expectedCert, "\\n", "\n", -1), actualCert)
}

func (s *ServerTestSuite) Test_ServeHTTP_InvokesPutCertWithDomainName_WhenServiceCertIsPresent() {
	actualCertName := ""
	expectedCert := "my-cert"
	actualCert := ""
	certOrig := cert
	defer func() { cert = certOrig }()
	cert = CertMock{
		PutCertMock: func(certName string, certContent []byte) (string, error) {
			actualCertName = certName
			actualCert = string(certContent[:])
			return "", nil
		},
	}
	address := fmt.Sprintf("%s&serviceDomain=%s&serviceCert=%s", s.ReconfigureUrl, s.ServiceDomain[0], expectedCert)
	req, _ := http.NewRequest("GET", address, nil)

	serverImpl.ServeHTTP(s.ResponseWriter, req)

	s.Equal(s.ServiceDomain[0], actualCertName)
	s.Equal(expectedCert, actualCert)
}

// ServeHTTP > Remove

func (s *ServerTestSuite) Test_ServeHTTP_SetsContentTypeToJSON_WhenUrlIsRemove() {
	var actual string
	httpWriterSetContentType = func(w http.ResponseWriter, value string) {
		actual = value
	}
	req, _ := http.NewRequest("GET", s.RemoveUrl, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.Equal("application/json", actual)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsJSON_WhenUrlIsRemove() {
	expected, _ := json.Marshal(server.Response{
		Status:      "OK",
		ServiceName: s.ServiceName,
	})

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, s.RequestRemove)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatus400_WhenUrlIsRemoveAndServiceNameQueryIsNotPresent() {
	req, _ := http.NewRequest("GET", s.RemoveBaseUrl, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 400)
}

func (s *ServerTestSuite) Test_ServeHTTP_InvokesRemoveExecute() {
	mockObj := getRemoveMock("")
	aclName := "my-acl"
	var actual actions.Remove
	expected := actions.Remove{
		ServiceName:     s.ServiceName,
		TemplatesPath:   "",
		ConfigsPath:     "",
		ConsulAddresses: []string{s.ConsulAddress},
		InstanceName:    s.InstanceName,
		AclName:         aclName,
	}
	actions.NewRemove = func(
		serviceName, aclName, configsPath, templatesPath string,
		consulAddresses []string,
		instanceName, mode string,
	) actions.Removable {
		actual = actions.Remove{
			ServiceName:     serviceName,
			AclName:         aclName,
			TemplatesPath:   templatesPath,
			ConfigsPath:     configsPath,
			ConsulAddresses: consulAddresses,
			InstanceName:    instanceName,
			Mode:            mode,
		}
		return mockObj
	}
	url := fmt.Sprintf("%s?serviceName=%s&aclName=%s", s.RemoveBaseUrl, s.ServiceName, aclName)
	req, _ := http.NewRequest("GET", url, nil)

	serverImpl.ServeHTTP(s.ResponseWriter, req)

	s.Equal(expected, actual)
	mockObj.AssertCalled(s.T(), "Execute", []string{})
}

// ServeHTTP > Config

func (s *ServerTestSuite) Test_ServeHTTP_SetsContentTypeToText_WhenUrlIsConfig() {
	var actual string
	httpWriterSetContentType = func(w http.ResponseWriter, value string) {
		actual = value
	}
	req, _ := http.NewRequest("GET", s.ConfigUrl, nil)

	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.Equal("text/html", actual)
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsConfig_WhenUrlIsConfig() {
	expected := "some text"
	readFileOrig := proxy.ReadFile
	defer func() { proxy.ReadFile = readFileOrig }()
	proxy.ReadFile = func(filename string) ([]byte, error) {
		return []byte(expected), nil
	}

	req, _ := http.NewRequest("GET", s.ConfigUrl, nil)
	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "Write", []byte(expected))
}

func (s *ServerTestSuite) Test_ServeHTTP_ReturnsStatus500_WhenReadFileFails() {
	readFileOrig := readFile
	defer func() { readFile = readFileOrig }()
	readFile = func(filename string) ([]byte, error) {
		return []byte(""), fmt.Errorf("This is an error")
	}

	req, _ := http.NewRequest("GET", s.ConfigUrl, nil)
	srv := Serve{}
	srv.ServeHTTP(s.ResponseWriter, req)

	s.ResponseWriter.AssertCalled(s.T(), "WriteHeader", 500)
}

// Suite

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
				if strings.EqualFold(r.URL.Query().Get("returnError"), "true") {
					w.WriteHeader(http.StatusInternalServerError)
				} else {
					w.WriteHeader(http.StatusOK)
					w.Header().Set("Content-Type", "application/json")
				}
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

type ServerMock struct {
	mock.Mock
}

func (m *ServerMock) Execute(args []string) error {
	params := m.Called(args)
	return params.Error(0)
}

func (m *ServerMock) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	m.Called(w, req)
}

func getServerMock() *ServerMock {
	mockObj := new(ServerMock)
	mockObj.On("Execute", mock.Anything).Return(nil)
	mockObj.On("ServeHTTP", mock.Anything, mock.Anything)
	return mockObj
}

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

type ReloadMock struct {
	ExecuteMock func() error
}

func (m ReloadMock) Execute() error {
	return m.ExecuteMock()
}

type RunMock struct {
	mock.Mock
}

func (m *RunMock) Execute(args []string) error {
	params := m.Called(args)
	return params.Error(0)
}

func getRunMock(skipMethod string) *ReconfigureMock {
	mockObj := new(ReconfigureMock)
	if skipMethod != "Execute" {
		mockObj.On("Execute", mock.Anything).Return(nil)
	}
	return mockObj
}

type ReconfigureMock struct {
	mock.Mock
}

func (m *ReconfigureMock) Execute(args []string) error {
	params := m.Called(args)
	return params.Error(0)
}

func (m *ReconfigureMock) GetData() (actions.BaseReconfigure, proxy.Service) {
	m.Called()
	return actions.BaseReconfigure{}, proxy.Service{}
}

func (m *ReconfigureMock) ReloadAllServices(addresses []string, instanceName, mode, listenerAddress string) error {
	params := m.Called(addresses, instanceName, mode, listenerAddress)
	return params.Error(0)
}

func (m *ReconfigureMock) GetTemplates(sr *proxy.Service) (front, back string, err error) {
	params := m.Called(sr)
	return params.String(0), params.String(1), params.Error(2)
}

func getReconfigureMock(skipMethod string) *ReconfigureMock {
	mockObj := new(ReconfigureMock)
	if skipMethod != "Execute" {
		mockObj.On("Execute", mock.Anything).Return(nil)
	}
	if skipMethod != "GetData" {
		mockObj.On("GetData", mock.Anything, mock.Anything).Return(nil)
	}
	if skipMethod != "ReloadAllServices" {
		mockObj.On("ReloadAllServices", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	}
	if skipMethod != "GetTemplates" {
		mockObj.On("GetTemplates", mock.Anything).Return("", "", nil)
	}
	return mockObj
}

type RemoveMock struct {
	mock.Mock
}

func (m *RemoveMock) Execute(args []string) error {
	params := m.Called(args)
	return params.Error(0)
}

func getRemoveMock(skipMethod string) *RemoveMock {
	mockObj := new(RemoveMock)
	if skipMethod != "Execute" {
		mockObj.On("Execute", mock.Anything).Return(nil)
	}
	return mockObj
}

// Util

func (s *ServerTestSuite) invokesReconfigure(req *http.Request, invoke bool) {
	mockObj := getReconfigureMock("")
	var actualBase actions.BaseReconfigure
	expectedBase := actions.BaseReconfigure{
		ConsulAddresses: []string{s.ConsulAddress},
	}
	var actualService proxy.Service
	actions.NewReconfigure = func(baseData actions.BaseReconfigure, serviceData proxy.Service, mode string) actions.Reconfigurable {
		actualBase = baseData
		actualService = serviceData
		return mockObj
	}
	serverImpl := Serve{BaseReconfigure: expectedBase}
	portOrig := s.ServiceDest[0].Port
	defer func() { s.ServiceDest[0].Port = portOrig }()
	s.ServiceDest[0].Port = ""

	serverImpl.ServeHTTP(s.ResponseWriter, req)

	if invoke {
		s.Equal(expectedBase, actualBase)
		s.Equal(s.Service, actualService)
		mockObj.AssertCalled(s.T(), "Execute", []string{})
	} else {
		mockObj.AssertNotCalled(s.T(), "Execute", []string{})
	}
}
