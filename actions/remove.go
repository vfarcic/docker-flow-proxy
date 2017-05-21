package actions

import (
	"../proxy"
	"fmt"
	"strings"
)

type Removable interface {
	Executable
}

type Remove struct {
	ConfigsPath     string `short:"c" long:"configs-path" default:"/cfg" description:"The path to the configurations directory"`
	ConsulAddresses []string
	InstanceName    string `long:"proxy-instance-name" env:"PROXY_INSTANCE_NAME" default:"docker-flow" required:"true" description:"The name of the proxy instance."`
	ServiceName     string `short:"s" long:"service-name" required:"true" description:"The name of the service that should be removed (e.g. my-service)."`
	TemplatesPath   string `short:"t" long:"templates-path" default:"/cfg/tmpl" description:"The path to the templates directory"`
	Mode            string
	AclName         string
}

var RemoveInstance Remove

var NewRemove = func(serviceName, aclName, configsPath, templatesPath string, consulAddresses []string, instanceName, mode string) Removable {
	return &Remove{
		ServiceName:     serviceName,
		AclName:         aclName,
		TemplatesPath:   templatesPath,
		ConfigsPath:     configsPath,
		ConsulAddresses: consulAddresses,
		InstanceName:    instanceName,
		Mode:            mode,
	}
}

// TODO: Remove args
func (m *Remove) Execute(args []string) error {
	logPrintf("Removing %s configuration", m.ServiceName)
	if err := m.removeFiles(m.TemplatesPath, m.ServiceName, m.AclName, m.ConsulAddresses, m.InstanceName, m.Mode); err != nil {
		logPrintf(err.Error())
		return err
	}
	proxy.Instance.RemoveService(m.ServiceName)
	reload := Reload{}
	if err := reload.Execute(true); err != nil {
		logPrintf(err.Error())
		return err
	}
	return nil
}

func (m *Remove) removeFiles(templatesPath, serviceName, aclName string, registryAddresses []string, instanceName, mode string) error {
	logPrintf("Removing the %s configuration files", serviceName)
	if len(aclName) == 0 {
		aclName = serviceName
	}
	paths := []string{
		fmt.Sprintf("%s/%s-fe.cfg", templatesPath, aclName),
		fmt.Sprintf("%s/%s-be.cfg", templatesPath, aclName),
	}
	mu.Lock()
	defer mu.Unlock()
	for _, path := range paths {
		OsRemove(path)
	}
	if !strings.EqualFold(mode, "service") && !strings.EqualFold(mode, "swarm") {
		var err error
		if len(registryAddresses) > 0 {
			for _, address := range registryAddresses {
				if err = registryInstance.DeleteService([]string{address}, serviceName, instanceName); err == nil {
					return nil
				}
			}
			return fmt.Errorf("Could not remove the service from Consul\n%s", err.Error())
		}
	}
	return nil
}
