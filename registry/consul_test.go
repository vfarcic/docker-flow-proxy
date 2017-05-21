package registry

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"strings"
	"sync"
	"testing"
)

type ConsulTestSuite struct {
	suite.Suite
	registry          Registry
	templatesPath     string
	feTemplateName    string
	beTemplateName    string
	serviceName       string
	consulAddress     string
	feTemplate        string
	beTemplate        string
	createConfigsArgs CreateConfigsArgs
}

func (s *ConsulTestSuite) SetupTest() {
	s.templatesPath = "/path/to/templates"
	s.feTemplateName = "my-fe-template.ctmpl"
	s.beTemplateName = "my-be-template.ctmpl"
	s.serviceName = "my-service"
	s.consulAddress = "http://consul.io"
	s.feTemplate = "this is a FE template"
	s.beTemplate = "this is a BE template"
	s.createConfigsArgs = CreateConfigsArgs{
		Addresses:     []string{"http://consul.io"},
		TemplatesPath: "/path/to/templates",
		FeFile:        "my-fe-template.ctmpl",
		FeTemplate:    "this is a FE template",
		BeFile:        "my-be-template.ctmpl",
		BeTemplate:    "this is a BE template",
		ServiceName:   "my-service",
	}
	cmdRunConsulTemplateOrig := cmdRunConsulTemplate
	defer func() { cmdRunConsulTemplate = cmdRunConsulTemplateOrig }()
	cmdRunConsulTemplate = func(cmd *exec.Cmd) error {
		return nil
	}
	writeConsulTemplateFileOrig := WriteConsulTemplateFile
	defer func() { WriteConsulTemplateFile = writeConsulTemplateFileOrig }()
	WriteConsulTemplateFile = func(filename string, data []byte, perm os.FileMode) error {
		return nil
	}
}

// PutService

func (s *ConsulTestSuite) Test_PutService_PutsDataToConsul() {
	instanceName := "my-instance"
	var actualUrl, actualBody, actualMethod []string
	var mu = &sync.Mutex{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := ioutil.ReadAll(r.Body)
		mu.Lock()
		actualMethod = append(actualMethod, r.Method)
		actualUrl = append(actualUrl, r.URL.Path)
		actualBody = append(actualBody, string(body))
		mu.Unlock()
	}))
	defer server.Close()
	err := Consul{}.PutService([]string{server.URL}, instanceName, s.registry)

	s.NoError(err)

	type data struct{ key, value string }

	d := []data{
		{"color", s.registry.ServiceColor},
		{"path", strings.Join(s.registry.ServicePath, ",")},
		{"domain", strings.Join(s.registry.ServiceDomain, ",")},
		{"hostname", s.registry.OutboundHostname},
		{"pathtype", s.registry.PathType},
		{"consultemplatefepath", s.registry.ConsulTemplateFePath},
		{"consultemplatebepath", s.registry.ConsulTemplateBePath},
		{"port", string(s.registry.Port)},
	}
	for _, e := range d {
		s.Contains(actualUrl, fmt.Sprintf("/v1/kv/%s/%s/%s", instanceName, s.registry.ServiceName, e.key))
		s.Contains(actualBody, e.value)
		s.Equal("PUT", actualMethod[0])
	}
}

func (s *ConsulTestSuite) Test_PutService_ReturnsError_WhenFailure() {
	err := Consul{}.PutService([]string{"http:///THIS/URL/DOES/NOT/EXIST"}, "my-instance", s.registry)

	s.Error(err)
}

func (s *ConsulTestSuite) Test_PutService_DoesNotReturnError_WhenOneOfTheAddressesDoesNotFail() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer server.Close()

	addresses := []string{"http:///THIS/URL/DOES/NOT/EXIST", server.URL, "http:///THIS/URL/ALSO/DOES/NOT/EXIST"}
	err := Consul{}.PutService(addresses, "my-instance", s.registry)

	s.NoError(err)
}

func (s *ConsulTestSuite) Test_SendPutRequest_AddsHttp_WhenNotPresent() {
	instanceName := "my-proxy-instance"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer server.Close()
	url := strings.Replace(server.URL, "http://", "", -1)

	err := Consul{}.PutService([]string{url}, instanceName, s.registry)

	s.NoError(err)
}

// SendPutRequest

func (s *ConsulTestSuite) Test_SendPutRequest_SendsDataToConsul() {
	instanceName := "my-proxy-instance"
	key := "my-key"
	value := "my-value"
	serviceName := "my-service"
	var actualUrl, actualBody, actualMethod string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer r.Body.Close()
		body, _ := ioutil.ReadAll(r.Body)
		actualMethod = r.Method
		actualUrl = r.URL.Path
		actualBody = string(body)
	}))
	defer server.Close()

	c := make(chan error)
	go Consul{}.SendPutRequest([]string{server.URL}, serviceName, key, value, instanceName, c)
	err := <-c

	s.NoError(err)
	s.Equal(fmt.Sprintf("/v1/kv/%s/%s/%s", instanceName, serviceName, key), actualUrl)
	s.Equal(value, actualBody)
	s.Equal("PUT", actualMethod)
}

func (s *ConsulTestSuite) Test_SendPutRequest_ReturnsError_WhenAddressDoesNotExist() {
	instanceName := "my-proxy-instance"
	key := "my-key"
	value := "my-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer server.Close()

	c := make(chan error)
	go Consul{}.SendPutRequest([]string{"http:///THIS/URL/DOES/NOT/EXIST"}, s.serviceName, key, value, instanceName, c)
	err := <-c

	s.Error(err)
}

func (s *ConsulTestSuite) Test_SendPutRequest_DoesNotReturnError_WhenOneOfTheAddressesExists() {
	instanceName := "my-proxy-instance"
	key := "my-key"
	value := "my-value"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer server.Close()
	addresses := []string{"http:///THIS/URL/DOES/NOT/EXIST", server.URL}

	c := make(chan error)
	go Consul{}.SendPutRequest(addresses, s.serviceName, key, value, instanceName, c)
	err := <-c

	s.NoError(err)
}

// DeleteService

func (s *ConsulTestSuite) Test_DeleteService_DeletesServiceFromConsul() {
	instanceName := "my-proxy-instance"
	var actualUrl, actualMethod, actualQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualMethod = r.Method
		actualUrl = r.URL.Path
		actualQuery = r.URL.RawQuery
	}))
	defer server.Close()

	err := Consul{}.DeleteService([]string{server.URL}, s.registry.ServiceName, instanceName)

	s.NoError(err)
	s.Equal(fmt.Sprintf("/v1/kv/%s/%s", instanceName, s.registry.ServiceName), actualUrl)
	s.Equal("DELETE", actualMethod)
	s.Equal("recurse", actualQuery)
}

func (s *ConsulTestSuite) Test_DeleteService_ReturnsError_WhenFailure() {
	addresses := []string{"http:///THIS/URL/DOES/NOT/EXIST"}
	err := Consul{}.DeleteService(addresses, s.registry.ServiceName, "my-instance")

	s.Error(err)
}

func (s *ConsulTestSuite) Test_DeleteService_DoesNotReturnError_WhenOneOfTheAddressesFail() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	addresses := []string{"http:///THIS/URL/DOES/NOT/EXIST", server.URL}
	err := Consul{}.DeleteService(addresses, s.registry.ServiceName, "my-instance")

	s.NoError(err)
}

func (s *ConsulTestSuite) Test_DeleteService_AddsHttp_WhenNotPresent() {
	instanceName := "my-proxy-instance"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	}))
	defer server.Close()
	address := strings.Replace(server.URL, "http://", "", -1)

	err := Consul{}.DeleteService([]string{address}, s.registry.ServiceName, instanceName)

	s.NoError(err)
}

// GetServiceAttribute

func (s *ConsulTestSuite) Test_GetServiceAttribute_ReturnsError_WhenConsulReturnsError() {
	addresses := []string{"http:///THIS/URL/DOES/NOT/EXIST"}
	_, err := Consul{}.GetServiceAttribute(addresses, "my-instance", "my-service", "my-key")

	s.Error(err)
}

func (s *ConsulTestSuite) Test_GetServiceAttribute_ReturnsError_WhenConsulReturnsNon200Code() {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	_, err := Consul{}.GetServiceAttribute([]string{server.URL}, "my-instance", "my-service", "my-key")

	s.Error(err)
}

func (s *ConsulTestSuite) Test_GetServiceAttribute_ReturnsConsulResponse() {
	expected := "this is a response"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(expected))
	}))
	actual, _ := Consul{}.GetServiceAttribute([]string{server.URL}, "my-instance", "my-service", "my-key")

	s.Equal(expected, actual)
}

// CreateConfigs

func (s *ConsulTestSuite) Test_CreateConfigs_ReturnsError_WhenConsulTemplateFeCommandFails() {
	cmdRunConsulTemplate = func(cmd *exec.Cmd) error {
		return fmt.Errorf("This is an error")
	}

	err := Consul{}.CreateConfigs(&s.createConfigsArgs)

	s.Error(err)
}

func (s *ConsulTestSuite) Test_CreateConfigs_ReturnsError_WhenConsulTemplateBeCommandFails() {
	counter := 0
	cmdRunConsulTemplate = func(cmd *exec.Cmd) error {
		if counter == 0 {
			counter = counter + 1
			return nil
		}
		return fmt.Errorf("This is an error")
	}

	err := Consul{}.CreateConfigs(&s.createConfigsArgs)

	s.Error(err)
}

func (s *ConsulTestSuite) Test_CreateConfigs_RunsConsulTemplate() {
	var actual [][]string
	cmdRunConsulTemplate = func(cmd *exec.Cmd) error {
		actual = append(actual, cmd.Args)
		return nil
	}
	expectedFe := []string{
		"consul-template",
		"-consul",
		strings.Replace(s.consulAddress, "http://", "", -1),
		"-template",
		fmt.Sprintf(
			`%s/%s:%s/%s-%s.cfg`,
			s.templatesPath,
			s.feTemplateName,
			s.templatesPath,
			s.serviceName,
			"fe",
		),
		"-once",
	}
	expectedBe := []string{
		"consul-template",
		"-consul",
		strings.Replace(s.consulAddress, "http://", "", -1),
		"-template",
		fmt.Sprintf(
			`%s/%s:%s/%s-%s.cfg`,
			s.templatesPath,
			s.beTemplateName,
			s.templatesPath,
			s.serviceName,
			"be",
		),
		"-once",
	}

	Consul{}.CreateConfigs(&s.createConfigsArgs)

	s.Equal(2, len(actual))
	s.Equal(expectedFe, actual[0])
	s.Equal(expectedBe, actual[1])
}

func (s *ConsulTestSuite) Test_CreateConfigs_CreatesConsulTemplate() {
	var actual string
	WriteConsulTemplateFile = func(filename string, data []byte, perm os.FileMode) error {
		actual = string(data)
		return nil
	}

	Consul{}.CreateConfigs(&s.createConfigsArgs)

	s.Equal(s.feTemplate, actual)
}

func (s *ConsulTestSuite) Test_CreateConfigs_RunsConsulTemplateWithTrimmedHttp() {
	var actual [][]string
	cmdRunConsulTemplate = func(cmd *exec.Cmd) error {
		actual = append(actual, cmd.Args)
		return nil
	}
	expectedFe := []string{
		"consul-template",
		"-consul",
		strings.Replace(s.consulAddress, "http://", "", -1),
		"-template",
		fmt.Sprintf(
			`%s/%s:%s/%s-%s.cfg`,
			s.templatesPath,
			s.feTemplateName,
			s.templatesPath,
			s.serviceName,
			"fe",
		),
		"-once",
	}
	expectedBe := []string{
		"consul-template",
		"-consul",
		strings.Replace(s.consulAddress, "http://", "", -1),
		"-template",
		fmt.Sprintf(
			`%s/%s:%s/%s-%s.cfg`,
			s.templatesPath,
			s.beTemplateName,
			s.templatesPath,
			s.serviceName,
			"be",
		),
		"-once",
	}

	s.createConfigsArgs.Addresses = []string{strings.Replace(s.consulAddress, "http://", "hTtP://", -1)}
	Consul{}.CreateConfigs(&s.createConfigsArgs)

	s.Equal(2, len(actual))
	s.Equal(expectedFe, actual[0])
	s.Equal(expectedBe, actual[1])
}

func (s *ConsulTestSuite) Test_CreateConfigs_RunsConsulTemplateWithTrimmedHttps() {
	var actual [][]string
	cmdRunConsulTemplate = func(cmd *exec.Cmd) error {
		actual = append(actual, cmd.Args)
		return nil
	}
	expectedFe := []string{
		"consul-template",
		"-consul",
		strings.Replace(s.consulAddress, "http://", "", -1),
		"-template",
		fmt.Sprintf(
			`%s/%s:%s/%s-%s.cfg`,
			s.templatesPath,
			s.feTemplateName,
			s.templatesPath,
			s.serviceName,
			"fe",
		),
		"-once",
	}
	expectedBe := []string{
		"consul-template",
		"-consul",
		strings.Replace(s.consulAddress, "http://", "", -1),
		"-template",
		fmt.Sprintf(
			`%s/%s:%s/%s-%s.cfg`,
			s.templatesPath,
			s.beTemplateName,
			s.templatesPath,
			s.serviceName,
			"be",
		),
		"-once",
	}

	s.createConfigsArgs.Addresses = []string{strings.Replace(s.consulAddress, "http://", "hTTPs://", -1)}
	Consul{}.CreateConfigs(&s.createConfigsArgs)

	s.Equal(2, len(actual))
	s.Equal(expectedFe, actual[0])
	s.Equal(expectedBe, actual[1])
}

func (s *ConsulTestSuite) Test_CreateConfigs_WritesTemplateToFile() {
	var actual []string
	expected := []string{
		fmt.Sprintf("%s/%s", s.templatesPath, s.feTemplateName),
		fmt.Sprintf("%s/%s", s.templatesPath, s.beTemplateName),
	}
	WriteConsulTemplateFile = func(filename string, data []byte, perm os.FileMode) error {
		actual = append(actual, filename)
		return nil
	}

	s.createConfigsArgs.Addresses = []string{fmt.Sprintf("HttP://%s", s.consulAddress)}
	Consul{}.CreateConfigs(&s.createConfigsArgs)

	s.Equal(expected, actual)
}

func (s *ConsulTestSuite) Test_CreateConfigs_DoesNotWriteFrontTemplate_WhenEmpty() {
	var actual []string
	expected := []string{
		fmt.Sprintf("%s/%s", s.templatesPath, s.beTemplateName),
	}
	WriteConsulTemplateFile = func(filename string, data []byte, perm os.FileMode) error {
		actual = append(actual, filename)
		return nil
	}
	args := CreateConfigsArgs{
		Addresses:     []string{"http://consul.io"},
		TemplatesPath: "/path/to/templates",
		FeFile:        "",
		FeTemplate:    "",
		BeFile:        "my-be-template.ctmpl",
		BeTemplate:    "this is a BE template",
		ServiceName:   "my-service",
	}

	s.createConfigsArgs.Addresses = []string{fmt.Sprintf("HttP://%s", s.consulAddress)}
	Consul{}.CreateConfigs(&args)

	s.Equal(expected, actual)
}

func (s *ConsulTestSuite) Test_CreateConfigs_SetsFilePermissions() {
	var actual os.FileMode
	var expected os.FileMode = 0664
	WriteConsulTemplateFile = func(filename string, data []byte, perm os.FileMode) error {
		actual = perm
		return nil
	}

	s.createConfigsArgs.Addresses = []string{fmt.Sprintf("HttP://%s", s.consulAddress)}
	Consul{}.CreateConfigs(&s.createConfigsArgs)

	s.Equal(expected, actual)
}

// Suite

func TestConsulUnitTestSuite(t *testing.T) {
	s := new(ConsulTestSuite)
	s.registry = Registry{
		ServiceName:          "my-service",
		ServiceColor:         "ServiceColor",
		ServicePath:          []string{"pat1", "path2"},
		ServiceDomain:        []string{"ServiceDomain"},
		OutboundHostname:     "OutboundHostname",
		PathType:             "PathType",
		ConsulTemplateFePath: "ConsulTemplateFePath",
		ConsulTemplateBePath: "ConsulTemplateBePath",
	}
	suite.Run(t, s)
}
