package proxy

type ServiceDest struct {
	// The internal port of a service that should be reconfigured.
	// The port is used only in the *swarm* mode.
	Port string
	// The URL path of the service.
	ServicePath []string
	// The source (entry) port of a service.
	// Useful only when specifying multiple destinations of a single service.
	SrcPort        int
	SrcPortAcl     string
	SrcPortAclName string
}

type Service struct {
	// ACLs are ordered alphabetically by their names.
	// If not specified, serviceName is used instead.
	AclName string
	// The path to the Consul Template representing a snippet of the backend configuration.
	// If set, proxy template will be loaded from the specified file.
	ConsulTemplateFePath string
	// The path to the Consul Template representing a snippet of the frontend configuration.
	// If specified, proxy template will be loaded from the specified file.
	ConsulTemplateBePath string
	// Whether to distribute a request to all the instances of the proxy.
	// Used only in the swarm mode.
	Distribute bool
	// Whether to redirect all http requests to https
	HttpsOnly bool
	// The internal HTTPS port of a service that should be reconfigured.
	// The port is used only in the swarm mode.
	// If not specified, the `port` parameter will be used instead.
	HttpsPort int
	// The hostname where the service is running, for instance on a separate swarm.
	// If specified, the proxy will dispatch requests to that domain.
	OutboundHostname string
	// The ACL derivative. Defaults to path_beg.
	// See https://cbonte.github.io/haproxy-dconv/configuration-1.5.html#7.3.6-path for more info.
	PathType string
	// Whether to redirect to https when X-Forwarded-Proto is http
	RedirectWhenHttpProto bool
	// The request mode. The proxy should be able to work with any mode supported by HAProxy. However, actively supported and tested modes are *http* and *tcp*. Please open an GitHub issue if the mode you're using does not work as expected. The default value is *http*.
	// Adding support for *sni*. Setting this to "sni" implies TCP with an SNI-based routing.
	ReqMode string
	// Deprecated in favor of ReqPathReplace
	ReqRepReplace string
	// Deprecated in favor of ReqPathSearch
	ReqRepSearch string
	// A regular expression to apply the modification.
	// If specified, `reqPathSearch` needs to be set as well.
	ReqPathReplace string
	// A regular expression to search the content to be replaced.
	// If specified, `reqPathReplace` needs to be set as well.
	ReqPathSearch string
	// Content of the PEM-encoded certificate to be used by the proxy when serving traffic over SSL.
	ServiceCert string
	// The domain of the service.
	// If set, the proxy will allow access only to requests coming to that domain.
	ServiceDomain []string
	// The name of the service.
	// It must match the name of the Swarm service or the one stored in Consul.
	ServiceName string
	// Whether to skip adding proxy checks.
	// This option is used only in the default mode.
	SkipCheck bool
	// If set to true, server certificates are not verified. This flag should be set for SSL enabled backend services.
	SslVerifyNone bool
	// The path to the template representing a snippet of the backend configuration.
	// If specified, the backend template will be loaded from the specified file.
	// If specified, `templateFePath` must be set as well.
	// See the https://github.com/vfarcic/docker-flow-proxy#templates section for more info.
	TemplateBePath string
	// The path to the template representing a snippet of the frontend configuration.
	// If specified, the frontend template will be loaded from the specified file.
	// If specified, `templateBePath` must be set as well.
	// See the https://github.com/vfarcic/docker-flow-proxy#templates section for more info.
	TemplateFePath string
	// The server timeout in seconds
	TimeoutServer string
	// The tunnel timeout in seconds
	TimeoutTunnel string
	// A comma-separated list of credentials(<user>:<pass>) for HTTP basic auth, which applies only to the service that will be reconfigured.
	Users               []User
	UsersPassEncrypted  bool
	ServiceColor        string
	ServicePort         string
	AclCondition        string
	FullServiceName     string
	Host                string
	LookupRetry         int
	LookupRetryInterval int
	ServiceDest         []ServiceDest
}

type Services []Service

func (slice Services) Len() int {
	return len(slice)
}

func (slice Services) Less(i, j int) bool {
	return slice[i].AclName < slice[j].AclName
}

func (slice Services) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

type User struct {
	Username string
	Password string
}
