package actions

import (
	"../proxy"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type FetchTestSuite struct {
	suite.Suite
	proxy.Service
	ConsulAddress     string
	ConsulTemplateFe  string
	ConsulTemplateBe  string
	ConfigsPath       string
	TemplatesPath     string
	fetch             Fetch
	Server            *httptest.Server
	PutPathResponse   string
	ConsulRequestBody proxy.Service
	InstanceName      string
}

func (s *FetchTestSuite) SetupTest() {
	sd := proxy.ServiceDest{
		ServicePath: []string{"path/to/my/service/api", "path/to/my/other/service/api"},
	}
	s.InstanceName = "proxy-test-instance"
	s.ServiceDest = []proxy.ServiceDest{sd}
	s.ConfigsPath = "path/to/configs/dir"
	s.TemplatesPath = "test_configs/tmpl"
	s.PathType = "path_beg"
	s.ConsulTemplateFe = `
    acl url_myService path_beg path/to/my/service/api path_beg path/to/my/other/service/api
    use_backend myService-be if url_myService`
	s.ConsulTemplateBe = `
backend myService-be
    mode http
    {{range $i, $e := service "myService" "any"}}
    server {{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}} check
    {{end}}`
	s.ConsulAddress = s.Server.URL
	s.fetch = Fetch{
		BaseReconfigure: BaseReconfigure{
			ConsulAddresses: []string{s.ConsulAddress},
			TemplatesPath:   s.TemplatesPath,
			ConfigsPath:     s.ConfigsPath,
			InstanceName:    s.InstanceName,
		},
	}

}

// Suite

func TestFetchUnitTestSuite(t *testing.T) {
	logPrintf = func(format string, v ...interface{}) {}
	s := new(FetchTestSuite)
	s.ServiceName = "myService"
	s.PutPathResponse = "PUT_PATH_OK"
	s.Server = GetTestServer(s.Service, s.InstanceName)
	defer s.Server.Close()
	registryInstanceOrig := registryInstance
	defer func() { registryInstance = registryInstanceOrig }()
	registryInstance = getRegistrarableMock("")
	writeFeTemplateOrig := writeFeTemplate
	defer func() { writeFeTemplate = writeFeTemplateOrig }()
	writeFeTemplate = func(filename string, data []byte, perm os.FileMode) error {
		return nil
	}
	writeBeTemplateOrig := writeBeTemplate
	defer func() { writeBeTemplate = writeBeTemplateOrig }()
	writeBeTemplate = func(filename string, data []byte, perm os.FileMode) error {
		return nil
	}
	mockObj := getProxyMock("")
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = mockObj
	suite.Run(t, s)
}

// ReloadAllServices

func (s *FetchTestSuite) Test_ReloadAllServices_ReturnsError_WhenFail() {
	err := s.fetch.ReloadServicesFromRegistry([]string{"this/address/does/not/exist"}, s.InstanceName, "")

	s.Error(err)
}

func (s *FetchTestSuite) Test_ReloadAllServices_InvokesProxyCreateConfigFromTemplates() {
	mockObj := getProxyMock("")
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = mockObj

	s.fetch.ReloadServicesFromRegistry([]string{s.ConsulAddress}, s.InstanceName, "")

	mockObj.AssertCalled(s.T(), "CreateConfigFromTemplates")
}

func (s *FetchTestSuite) Test_ReloadAllServices_ReturnsError_WhenProxyCreateConfigFromTemplatesFails() {
	mockObj := getProxyMock("CreateConfigFromTemplates")
	mockObj.On("CreateConfigFromTemplates", mock.Anything, mock.Anything).Return(fmt.Errorf("This is an error"))
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = mockObj

	actual := s.fetch.ReloadServicesFromRegistry([]string{s.ConsulAddress}, s.InstanceName, "")

	s.Error(actual)
}

func (s *FetchTestSuite) Test_ReloadAllServices_InvokesProxyReload() {
	proxyMock := getProxyMock("")
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = proxyMock

	s.fetch.ReloadServicesFromRegistry([]string{s.ConsulAddress}, s.InstanceName, "")

	proxyMock.AssertCalled(s.T(), "Reload")
	proxyMock.AssertCalled(s.T(), "CreateConfigFromTemplates")
}

func (s *FetchTestSuite) Test_ReloadAllServices_ReturnsError_WhenProxyReloadFails() {
	mockObj := getProxyMock("Reload")
	mockObj.On("Reload").Return(fmt.Errorf("This is an error"))
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = mockObj

	actual := s.fetch.ReloadServicesFromRegistry([]string{s.ConsulAddress}, s.InstanceName, "")

	s.Error(actual)
}

func (s *FetchTestSuite) Test_ReloadAllServices_AddsHttpIfNotPresent() {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = getProxyMock("")
	address := strings.Replace(s.ConsulAddress, "http://", "", -1)
	err := s.fetch.ReloadServicesFromRegistry([]string{address}, s.InstanceName, "")

	s.NoError(err)
}

func (s *FetchTestSuite) Test_ReloadClusterConfig_SendsARequestToSwarmListener_WhenListenerAddressIsDefined() {
	actual := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual = r.URL.Path
	}))
	defer func() { srv.Close() }()

	s.fetch.ReloadClusterConfig(srv.URL)

	s.Equal("/v1/docker-flow-swarm-listener/notify-services", actual)
}

func (s *FetchTestSuite) Test_ReloadClusterConfig_ReturnsError_WhenSwarmListenerStatusIsNot200() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer func() { srv.Close() }()

	err := s.fetch.ReloadClusterConfig(srv.URL)

	s.Error(err)
}

func (s *FetchTestSuite) Test_ReloadClusterConfig_ReturnsError_WhenSwarmListenerFails() {
	httpGetOrig := httpGet
	defer func() { httpGet = httpGetOrig }()
	httpGet = func(url string) (*http.Response, error) {
		resp := http.Response{
			StatusCode: http.StatusOK,
		}
		return &resp, fmt.Errorf("This is an error")
	}

	err := s.fetch.ReloadClusterConfig("http://google.com")

	s.Error(err)
}

// ReloadConfig

func (s *FetchTestSuite) Test_ReloadConfig_SendsARequestToSwarmListener_WhenListenerAddressIsDefined() {
	actual := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual = r.URL.Path
		configs := []map[string]string{
			{"serviceName": "someService", "serviceDomain": "my-domain"},
		}
		marshal, _ := json.Marshal(configs)
		w.Write(marshal)
	}))
	defer func() { srv.Close() }()

	var usedServiceData proxy.Service
	OldNewReconfigure := NewReconfigure
	defer func() { NewReconfigure = OldNewReconfigure }()
	reconfigureMock := getReconfigureMock("")
	NewReconfigure = func(baseData BaseReconfigure, serviceData proxy.Service, mode string) Reconfigurable {
		usedServiceData = serviceData
		return reconfigureMock
	}

	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxyMock := getProxyMock("")
	proxy.Instance = proxyMock

	err := s.fetch.ReloadConfig(BaseReconfigure{}, "swarm", srv.URL)

	s.Equal("/v1/docker-flow-swarm-listener/get-services", actual)
	s.NoError(err)
	s.Equal("someService", usedServiceData.ServiceName)
	reconfigureMock.AssertCalled(s.T(), "Execute", false)
	reconfigureMock.AssertNumberOfCalls(s.T(), "Execute", 1)
	proxyMock.AssertCalled(s.T(), "Reload")
	proxyMock.AssertCalled(s.T(), "CreateConfigFromTemplates")

}

func (s *FetchTestSuite) Test_ReloadConfig_ReturnsError_WhenSwarmListenerReturnsWrongData() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		configs := []string{"dummyData"}
		marshal, _ := json.Marshal(configs)
		w.Write(marshal)
	}))
	defer func() { srv.Close() }()

	err := s.fetch.ReloadConfig(BaseReconfigure{}, "swarm", srv.URL)

	s.Error(err)
}

func (s *FetchTestSuite) Test_ReloadConfig_ReturnsError_WhenSwarmListenerStatusIsNot200() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer func() { srv.Close() }()

	err := s.fetch.ReloadConfig(BaseReconfigure{}, "swarm", srv.URL)

	s.Error(err)
}

func (s *FetchTestSuite) Test_ReloadConfig_ReturnsError_WhenSwarmListenerFails() {
	httpGetOrig := httpGet
	defer func() { httpGet = httpGetOrig }()
	httpGet = func(url string) (*http.Response, error) {
		resp := http.Response{
			StatusCode: http.StatusOK,
		}
		return &resp, fmt.Errorf("This is an error")
	}

	err := s.fetch.ReloadConfig(BaseReconfigure{}, "swarm", "http://google.com")

	s.Error(err)
}
