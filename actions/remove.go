package actions

import (
	"../proxy"
	"fmt"
)

// Removable defines functions that must be implemented by any struct in charge of removing services from the proxy.
type Removable interface {
	executable
}

// Remove contains the information required for removing services from the proxy
type Remove struct {
	ConfigsPath   string `short:"c" long:"configs-path" default:"/cfg" description:"The path to the configurations directory"`
	InstanceName  string `long:"proxy-instance-name" env:"PROXY_INSTANCE_NAME" default:"docker-flow" required:"true" description:"The name of the proxy instance."`
	ServiceName   string `short:"s" long:"service-name" required:"true" description:"The name of the service that should be removed (e.g. my-service)."`
	TemplatesPath string `short:"t" long:"templates-path" default:"/cfg/tmpl" description:"The path to the templates directory"`
	AclName       string
}

// NewRemove returns singleton based on the Removable interface
var NewRemove = func(serviceName, aclName, configsPath, templatesPath string, instanceName string) Removable {
	return &Remove{
		ServiceName:   serviceName,
		AclName:       aclName,
		TemplatesPath: templatesPath,
		ConfigsPath:   configsPath,
		InstanceName:  instanceName,
	}
}

// Execute initiates the removal of a service
func (m *Remove) Execute(args []string) error {
	logPrintf("Removing %s configuration", m.ServiceName)
	if err := m.removeFiles(m.TemplatesPath, m.ServiceName, m.AclName); err != nil {
		logPrintf(err.Error())
		return err
	}
	proxy.Instance.RemoveService(m.ServiceName)
	reload := reload{}
	if err := reload.Execute(true); err != nil {
		logPrintf(err.Error())
		return err
	}
	return nil
}

func (m *Remove) removeFiles(templatesPath, serviceName, aclName string) error {
	logPrintf("Removing the %s configuration files", serviceName)
	if len(aclName) == 0 {
		aclName = serviceName
	}
	paths := []string{
		fmt.Sprintf("%s/%s-fe.cfg", templatesPath, aclName),
		fmt.Sprintf("%s/%s-be.cfg", templatesPath, aclName),
	}
	//	mu.Lock()
	//	defer mu.Unlock()
	for _, path := range paths {
		osRemove(path)
	}
	return nil
}
