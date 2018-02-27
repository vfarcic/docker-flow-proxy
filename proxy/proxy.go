package proxy

var proxyInstance proxy = HaProxy{}

// Data contains the information about all the services
type Data struct {
	Services map[string]Service
}

var dataInstance = Data{}

type proxy interface {
	RunCmd(extraArgs []string) error
	CreateConfigFromTemplates() error
	ReadConfig() (string, error)
	Reload() error
	GetCertPaths() []string
	GetCerts() map[string]string
	AddService(service Service)
	RemoveService(service string)
	GetServices() map[string]Service
}
