package registry

const (
	COLOR_KEY                   = "color"
	PATH_KEY                    = "path"
	DOMAIN_KEY                  = "domain"
	CERT_KEY                    = "cert"
	HOSTNAME_KEY                = "hostname"
	PATH_TYPE_KEY               = "pathtype"
	CONSUL_TEMPLATE_FE_PATH_KEY = "consultemplatefepath"
	CONSUL_TEMPLATE_BE_PATH_KEY = "consultemplatebepath"
	PORT                        = "port"
)

type Registry struct {
	ServiceName          string
	Port                 string
	ServiceColor         string
	ServicePath          []string
	ServiceDomain        []string
	ServiceCert          string
	OutboundHostname     string
	PathType             string
	ConsulTemplateFePath string
	ConsulTemplateBePath string
}

type Registrarable interface {
	PutService(addresses []string, instanceName string, r Registry) error
	SendPutRequest(addresses []string, serviceName, key, value, instanceName string, c chan error)
	DeleteService(addresses []string, serviceName, instanceName string) error
	CreateConfigs(args *CreateConfigsArgs) error
	GetServiceAttribute(addresses []string, serviceName, key, instanceName string) (string, error)
}
