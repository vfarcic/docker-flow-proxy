package main

import (
	"github.com/stretchr/testify/suite"
	"fmt"
	"os"
	"os/exec"
	"testing"
	"github.com/stretchr/testify/mock"
)

type ReconfigureTestSuite struct {
	suite.Suite
	ServiceReconfigure
	ConsulAddress	string
	ConsulTemplate	string
	ConfigsPath		string
	TemplatesPath	string
	reconfigure		Reconfigure
	Pid				string
}

func (s *ReconfigureTestSuite) SetupTest() {
	s.ServiceName = "myService"
	s.Pid = "123"
	s.ConsulAddress = "1.2.3.4:1234"
	s.ServicePath = []string{"path/to/my/service/api"}
	s.ServiceDomain = "my-domain.com"
	s.ConfigsPath = "path/to/configs/dir"
	s.TemplatesPath = "test_configs/tmpl"
	s.ConsulTemplate = `frontend myService-fe
	bind *:80
	bind *:443
	option http-server-close
	acl url_myService path_beg path/to/my/service/api
	use_backend myService-be if url_myService

backend myService-be
  reqrep ^([^\ ]*)\ path/to/my/service/api/(.*) \1\ /\2
	{{range $i, $e := service "myService" "any"}}
	server {{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}} check
	{{end}}`
	cmdRunHa = func(cmd *exec.Cmd) error {
		return nil
	}
	cmdRunConsul = func(cmd *exec.Cmd) error {
		return nil
	}
	s.reconfigure = Reconfigure {
		BaseReconfigure: BaseReconfigure{
			ConsulAddress: s.ConsulAddress,
			TemplatesPath: s.TemplatesPath,
			ConfigsPath: s.ConfigsPath,
		},
		ServiceReconfigure: ServiceReconfigure{
			ServiceName: s.ServiceName,
			ServicePath: s.ServicePath,
		},
	}
	readFile = func(fileName string) ([]byte, error) {
		return []byte(""), nil
	}
	readPidFile = func(fileName string) ([]byte, error) {
		return []byte(s.Pid), nil
	}
	readDir = func (dirname string) ([]os.FileInfo, error) {
		return nil, nil
	}
	writeConsulConfigFile = func(fileName string, data []byte, perm os.FileMode) error {
		return nil
	}
	writeConsulTemplateFile = func(fileName string, data []byte, perm os.FileMode) error {
		return nil
	}
}

// getConsulTemplate

func (s ReconfigureTestSuite) Test_GetConsulTemplate_ReturnsFormattedContent() {
	actual := s.reconfigure.getConsulTemplate()

	s.Equal(s.ConsulTemplate, actual)
}

func (s ReconfigureTestSuite) Test_GetConsulTemplate_AddsHost() {
	s.ConsulTemplate = `frontend myService-fe
	bind *:80
	bind *:443
	option http-server-close
	acl url_myService path_beg path/to/my/service/api
	acl domain_myService hdr_dom(host) -i my-domain.com
	use_backend myService-be if url_myService domain_myService

backend myService-be
  reqrep ^([^\ ]*)\ path/to/my/service/api/(.*) \1\ /\2
	{{range $i, $e := service "myService" "any"}}
	server {{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}} check
	{{end}}`
	s.reconfigure.ServiceDomain = s.ServiceDomain
	actual := s.reconfigure.getConsulTemplate()

	s.Equal(s.ConsulTemplate, actual)
}

func (s ReconfigureTestSuite) Test_GetConsulTemplate_AddsColor() {
	s.reconfigure.ServiceColor = "black"
	expected := fmt.Sprintf(`service "%s-%s"`, s.ServiceName, s.reconfigure.ServiceColor)

	actual := s.reconfigure.getConsulTemplate()

	s.Contains(actual, expected)
}

// Execute

func (s ReconfigureTestSuite) Test_Execute_CreatesConsulTemplate() {
	var actual string
	writeConsulTemplateFile = func(filename string, data []byte, perm os.FileMode) error {
		if len(actual) == 0 {
			actual = string(data)
		}
		return nil
	}

	s.reconfigure.Execute([]string{})

	s.Equal(s.ConsulTemplate, actual)
}

func (s ReconfigureTestSuite) Test_Execute_WritesTemplateToFile() {
	var actual string
	expected := fmt.Sprintf("%s/%s", s.TemplatesPath, ServiceTemplateFilename)
	writeConsulTemplateFile = func(filename string, data []byte, perm os.FileMode) error {
		if len(actual) == 0 {
			actual = filename
		}
		return nil
	}

	s.reconfigure.Execute([]string{})

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_Execute_SetsFilePermissions() {
	var actual os.FileMode
	var expected os.FileMode = 0664
	writeConsulTemplateFile = func(filename string, data []byte, perm os.FileMode) error {
		actual = perm
		return nil
	}

	s.reconfigure.Execute([]string{})

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_Execute_RunsConsulTemplate() {
	actual := HaProxyTestSuite{}.mockConsulExecCmd()
	expected := []string{
		"consul-template",
		"-consul",
		s.ConsulAddress,
		"-template",
		fmt.Sprintf(
			`%s/%s:%s/%s.cfg`,
			s.TemplatesPath,
			ServiceTemplateFilename,
			s.TemplatesPath,
			s.ServiceName,
		),
		"-once",
	}

	s.reconfigure.Execute([]string{})

	s.Equal(expected, *actual)
}

func (s ReconfigureTestSuite) Test_Execute_RunsConsulTemplateWithTrimmedHttp() {
	actual := HaProxyTestSuite{}.mockConsulExecCmd()
	expected := []string{
		"consul-template",
		"-consul",
		s.ConsulAddress,
		"-template",
		fmt.Sprintf(
			`%s/%s:%s/%s.cfg`,
			s.TemplatesPath,
			ServiceTemplateFilename,
			s.TemplatesPath,
			s.ServiceName,
		),
		"-once",
	}
	s.reconfigure.ConsulAddress = fmt.Sprintf("HttP://%s", s.ConsulAddress)

	s.reconfigure.Execute([]string{})

	s.Equal(expected, *actual)
}

func (s ReconfigureTestSuite) Test_Execute_RunsConsulTemplateWithTrimmedHttps() {
	actual := HaProxyTestSuite{}.mockConsulExecCmd()
	expected := []string{
		"consul-template",
		"-consul",
		s.ConsulAddress,
		"-template",
		fmt.Sprintf(
			`%s/%s:%s/%s.cfg`,
			s.TemplatesPath,
			ServiceTemplateFilename,
			s.TemplatesPath,
			s.ServiceName,
		),
		"-once",
	}
	s.reconfigure.ConsulAddress = fmt.Sprintf("HttPs://%s", s.ConsulAddress)

	s.reconfigure.Execute([]string{})

	s.Equal(expected, *actual)
}

func (s ReconfigureTestSuite) Test_Execute_ReturnsError_WhenConsulTemplateCommandFails() {
	cmdRunConsul = func(cmd *exec.Cmd) error {
		return fmt.Errorf("This is an error")
	}

	err := s.reconfigure.Execute([]string{})

	s.Error(err)
}

func (s ReconfigureTestSuite) Test_Execute_SavesConfigsToTheFile() {
	var actualFilename string
	var actualData string
	expected := fmt.Sprintf("%s/haproxy.cfg", s.ConfigsPath)
	writeConsulConfigFile = func(fileName string, data []byte, perm os.FileMode) error {
		actualFilename = fileName
		actualData = string(data)
		return nil
	}

	s.reconfigure.Execute([]string{})

	s.Equal(expected, actualFilename)
}

func (s ReconfigureTestSuite) Test_Execute_ReturnsError_WhenGetConfigsFail() {
	s.reconfigure.TemplatesPath = "/this/path/does/not/exist"

	err := s.reconfigure.Execute([]string{})
	s.Error(err)
}

func (s ReconfigureTestSuite) Test_Execute_RunsHaProxy() {
	actual := HaProxyTestSuite{}.mockHaExecCmd()
	expected := []string{
		"haproxy",
		"-f",
		"/cfg/haproxy.cfg",
		"-D",
		"-p",
		"/var/run/haproxy.pid",
		"-sf",
		s.Pid,
	}

	s.reconfigure.Execute([]string{})

	s.Equal(expected, *actual)
}

func (s ReconfigureTestSuite) Test_Execute_ReturnsError_WhenHaCommandFails() {
	cmdRunHa = func(cmd *exec.Cmd) error {
		return fmt.Errorf("This is an error")
	}

	err := s.reconfigure.Execute([]string{})

	s.Error(err)
}

func (s ReconfigureTestSuite) Test_Execute_ReadsPidFile() {
	var actual string
	readPidFile = func(fileName string) ([]byte, error) {
		actual = fileName
		return []byte(s.Pid), nil
	}

	s.reconfigure.Execute([]string{})

	s.Equal("/var/run/haproxy.pid", actual)
}

func (s ReconfigureTestSuite) Test_Execute_ReturnsError_WhenReadPidFails() {
	readPidFile = func(fileName string) ([]byte, error) {
		return []byte(""), fmt.Errorf("This is an error")
	}

	err := s.reconfigure.Execute([]string{})

	s.Error(err)
}

// NewReconfigure

func (s ReconfigureTestSuite) Test_NewReconfigure_AddsBaseAndService() {
	br := BaseReconfigure{ConsulAddress: "myConsulAddress"}
	sr := ServiceReconfigure{ServiceName: "myService"}

	r := NewReconfigure(br, sr)

	actualBr, actualSr := r.GetData()
	s.Equal(br, actualBr)
	s.Equal(sr, actualSr)
}

func (s ReconfigureTestSuite) Test_NewReconfigure_CreatesNewStruct() {
	r1 := NewReconfigure(
		BaseReconfigure{ConsulAddress: "myConsulAddress"},
		ServiceReconfigure{ServiceName: "myService"},
	)
	r2 := NewReconfigure(BaseReconfigure{}, ServiceReconfigure{})

	actualBr1, actualSr1 := r1.GetData()
	actualBr2, actualSr2 := r2.GetData()
	s.NotEqual(actualBr1, actualBr2)
	s.NotEqual(actualSr1, actualSr2)
}

// Suite

func TestReconfigureTestSuite(t *testing.T) {
	suite.Run(t, new(ReconfigureTestSuite))
}

// Mock

func (s HaProxyTestSuite) mockConsulExecCmd() *[]string {
	var actualCommand []string
	cmdRunConsul = func(cmd *exec.Cmd) error {
		actualCommand = cmd.Args
		return nil
	}
	return &actualCommand
}

type ReconfigureMock struct{
	mock.Mock
}

func (m *ReconfigureMock) Execute(args []string) error {
	params := m.Called(args)
	return params.Error(0)
}

func (m *ReconfigureMock) GetData() (BaseReconfigure, ServiceReconfigure) {
	m.Called()
	return BaseReconfigure{}, ServiceReconfigure{}
}

func getReconfigureMock(skipMethod string) *ReconfigureMock {
	mockObj := new(ReconfigureMock)
	if skipMethod != "Execute" {
		mockObj.On("Execute", mock.Anything).Return(nil)
	}
	return mockObj
}
