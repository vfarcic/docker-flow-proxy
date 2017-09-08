// +build !integration

package actions

import (
	"../proxy"
	"fmt"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"testing"
)

type RemoveTestSuite struct {
	suite.Suite
	remove        Remove
	ServiceName   string
	ConfigsPath   string
	TemplatesPath string
	InstanceName  string
}

func TestRemoveUnitTestSuite(t *testing.T) {
	logPrintf = func(format string, v ...interface{}) {}
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = getProxyMock("")
	suite.Run(t, new(RemoveTestSuite))
}

func (s *RemoveTestSuite) SetupTest() {
	s.ServiceName = "myService"
	s.TemplatesPath = "/path/to/templates"
	s.ConfigsPath = "/path/to/configs"
	s.InstanceName = "my-proxy-instance"
	osRemove = func(name string) error {
		return nil
	}
	s.remove = Remove{
		ServiceName:   s.ServiceName,
		ConfigsPath:   s.ConfigsPath,
		TemplatesPath: s.TemplatesPath,
		InstanceName:  s.InstanceName,
	}
}

// Execute

func (s RemoveTestSuite) Test_Execute_RemovesConfigurationFile() {
	var actual []string
	expected := []string{
		fmt.Sprintf("%s/%s-fe.cfg", s.TemplatesPath, s.ServiceName),
		fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName),
	}
	osRemove = func(name string) error {
		actual = append(actual, name)
		return nil
	}

	s.remove.Execute([]string{})

	s.Equal(expected, actual)
}

func (s RemoveTestSuite) Test_Execute_RemovesConfigurationFileUsingAclName_WhenPresent() {
	s.remove.AclName = "my-acl"
	var actual []string
	expected := []string{
		fmt.Sprintf("%s/%s-fe.cfg", s.TemplatesPath, s.remove.AclName),
		fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.remove.AclName),
	}
	osRemove = func(name string) error {
		actual = append(actual, name)
		return nil
	}

	s.remove.Execute([]string{})

	s.Equal(expected, actual)
}

func (s RemoveTestSuite) Test_Execute_Invokes_HaProxyCreateConfigFromTemplates() {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	mockObj := getProxyMock("")
	proxy.Instance = mockObj

	s.remove.Execute([]string{})

	mockObj.AssertCalled(s.T(), "CreateConfigFromTemplates")
}

func (s RemoveTestSuite) Test_Execute_ReturnsError_WhenHaProxyCreateConfigFromTemplatesFails() {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	mockObj := getProxyMock("CreateConfigFromTemplates")
	mockObj.On("CreateConfigFromTemplates", mock.Anything, mock.Anything).Return(fmt.Errorf("This is an error"))
	proxy.Instance = mockObj

	err := s.remove.Execute([]string{})

	s.Error(err)
}

func (s RemoveTestSuite) Test_Execute_Invokes_HaProxyReload() {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	mockObj := getProxyMock("")
	proxy.Instance = mockObj

	s.remove.Execute([]string{})

	mockObj.AssertCalled(s.T(), "Reload")
}

func (s RemoveTestSuite) Test_Execute_ReturnsError_WhenHaProxyReloadFails() {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	mockObj := getProxyMock("CreateConfigFromTemplates")
	mockObj.On("CreateConfigFromTemplates", mock.Anything, mock.Anything).Return(fmt.Errorf("This is an error"))
	proxy.Instance = mockObj

	err := s.remove.Execute([]string{})

	s.Error(err)
}

func (s RemoveTestSuite) Test_Execute_RemovesService() {
	mockObj := getProxyMock("")
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = mockObj
	s.remove.ServiceName = "my-soon-to-be-removed-service"

	s.remove.Execute([]string{})

	mockObj.AssertCalled(s.T(), "RemoveService", s.remove.ServiceName)
}
