package actions

import (
	"../proxy"
	"fmt"
	"github.com/stretchr/testify/suite"
	"testing"
	"net/http/httptest"
	"net/http"
	"encoding/json"
)

type ReloadTestSuite struct {
	suite.Suite
}

func TestReloadUnitTestSuite(t *testing.T) {
	suite.Run(t, new(ReloadTestSuite))
}

func (s *ReloadTestSuite) SetupTest() {
}

// Execute

func (s *ReloadTestSuite) Test_Execute_Invokes_HaProxyReload() {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	mockObj := getProxyMock("")
	proxy.Instance = mockObj
	reload := Reload{}

	reload.Execute(false)

	mockObj.AssertCalled(s.T(), "Reload")
}

func (s *ReloadTestSuite) Test_Execute_ReturnsError_WhenHaProxyReloadFails() {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	mockObj := getProxyMock("Reload")
	mockObj.On("Reload").Return(fmt.Errorf("This is an error"))
	proxy.Instance = mockObj
	reload := Reload{}

	err := reload.Execute(false)

	s.Error(err)
}

func (s *ReloadTestSuite) Test_Execute_InvokesCreateConfigFromTemplates_WhenRecreateIsTrue() {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	mockObj := getProxyMock("")
	proxy.Instance = mockObj
	reload := Reload{}

	reload.Execute(true)

	mockObj.AssertCalled(s.T(), "CreateConfigFromTemplates")
}

func (s *ReloadTestSuite) Test_Execute_ReturnsError_WhenCreateConfigFromTemplatesFails() {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	mockObj := getProxyMock("CreateConfigFromTemplates")
	mockObj.On("CreateConfigFromTemplates").Return(fmt.Errorf("This is an error"))
	proxy.Instance = mockObj
	reload := Reload{}

	err := reload.Execute(true)

	s.Error(err)
}

func (s *ReloadTestSuite) Test_ReloadClusterConfig_SendsARequestToSwarmListener_WhenListenerAddressIsDefined() {
	actual := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual = r.URL.Path
	}))
	defer func() { srv.Close() }()

	reload := Reload{}
	reload.ReloadClusterConfig(srv.URL)

	s.Equal("/v1/docker-flow-swarm-listener/notify-services", actual)
}

func (s *ReloadTestSuite) Test_ReloadClusterConfig_ReturnsError_WhenSwarmListenerStatusIsNot200() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer func() { srv.Close() }()

	reload := Reload{}
	err := reload.ReloadClusterConfig(srv.URL)

	s.Error(err)
}

func (s *ReloadTestSuite) Test_ReloadClusterConfig_ReturnsError_WhenSwarmListenerFails() {
	httpGetOrig := httpGet
	defer func() { httpGet = httpGetOrig }()
	httpGet = func(url string) (*http.Response, error) {
		resp := http.Response{
			StatusCode: http.StatusOK,
		}
		return &resp, fmt.Errorf("This is an error")
	}

	reload := Reload{}
	err := reload.ReloadClusterConfig("http://google.com")

	s.Error(err)
}

// ReloadConfig

func (s *ReloadTestSuite) Test_ReloadConfig_SendsARequestToSwarmListener_WhenListenerAddressIsDefined() {
	actual := ""
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actual = r.URL.Path
		configs := []map[string]string{
			{"serviceName": "someService"},
		}
		marshal, _ := json.Marshal(configs)
		w.Write(marshal)
	}))
	defer func() { srv.Close() }()

	var usedServiceData proxy.Service
	OldNewReconfigure := NewReconfigure
	defer func() { NewReconfigure = OldNewReconfigure }()
	mock := getReconfigureMock("");
	NewReconfigure = func(baseData BaseReconfigure, serviceData proxy.Service, mode string) Reconfigurable {
		usedServiceData = serviceData
		return mock
	}

	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	mockObj := getProxyMock("")
	proxy.Instance = mockObj


	reload := Reload{}
	err := reload.ReloadConfig(BaseReconfigure{}, "swarm", srv.URL)

	s.Equal("/v1/docker-flow-swarm-listener/get-services", actual)
	s.NoError(err)
	s.Equal("someService", usedServiceData.ServiceName)
	mock.AssertNumberOfCalls(s.T(), "Execute", 1)
	mockObj.AssertCalled(s.T(), "Reload")

}

func (s *ReloadTestSuite) Test_ReloadConfig_ReturnsError_WhenSwarmListenerReturnsWrongData() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		configs := []string{"dummyData"}
		marshal, _ := json.Marshal(configs)
		w.Write(marshal)
	}))
	defer func() { srv.Close() }()

	reload := Reload{}
	err := reload.ReloadConfig(BaseReconfigure{}, "swarm", srv.URL)

	s.Error(err)
}

func (s *ReloadTestSuite) Test_ReloadConfig_ReturnsError_WhenSwarmListenerStatusIsNot200() {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer func() { srv.Close() }()

	reload := Reload{}
	err := reload.ReloadConfig(BaseReconfigure{}, "swarm", srv.URL)

	s.Error(err)
}

func (s *ReloadTestSuite) Test_ReloadConfig_ReturnsError_WhenSwarmListenerFails() {
	httpGetOrig := httpGet
	defer func() { httpGet = httpGetOrig }()
	httpGet = func(url string) (*http.Response, error) {
		resp := http.Response{
			StatusCode: http.StatusOK,
		}
		return &resp, fmt.Errorf("This is an error")
	}

	reload := Reload{}
	err := reload.ReloadConfig(BaseReconfigure{}, "swarm", "http://google.com")

	s.Error(err)
}

// NewReload

func (s *ReloadTestSuite) Test_NewReload_ReturnsNewInstance() {
	r := NewReload()

	s.NotNil(r)
}
