package proxy

import (
	"fmt"
	"github.com/mitchellh/mapstructure"
	"reflect"
	"strconv"
	"strings"
)

var extractUsersFromString = ExtractUsersFromString
var usersBasePath string = "/run/secrets/dfp_users_%s"

// Data used to generate proxy configuration. It is extracted as a separate struct since a single service can have multiple combinations.
type ServiceDest struct {
	// The internal port of a service that should be reconfigured.
	// The port is used only in the *swarm* mode.
	Port string
	// The request mode. The proxy should be able to work with any mode supported by HAProxy.
	// However, actively supported and tested modes are *http*, *tcp*, and *sni*.
	ReqMode string
	// Internal use only. Do not modify.
	ReqModeFormatted string
	// The URL path of the service.
	ServicePath []string
	// The source (entry) port of a service.
	// Useful only when specifying multiple destinations of a single service.
	SrcPort int
	// Internal use only. Do not modify.
	SrcPortAcl string
	// Internal use only. Do not modify.
	SrcPortAclName string
	// Whether to verify client SSL and deny request when it is invalid
	VerifyClientSsl bool
	// If specified, only requests with the same agent will be forwarded to the backend.
	UserAgent UserAgent
}

// Data used to generate proxy configuration. It is extracted as a separate struct since each user agent needs an ACL identifier. If specified, only requests with the same agent will be forwarded to the backend.
type UserAgent struct {
	Value 	[]string
	AclName string
}

// Description of a service that should be added to the proxy configuration.
type Service struct {
	// ACLs are ordered alphabetically by their names.
	// If not specified, serviceName is used instead.
	AclName string `split_words:"true"`
	// Additional headers that will be added to the request before forwarding it to the service.
	// Please consult https://www.haproxy.com/doc/aloha/7.0/haproxy/http_rewriting.html#add-a-header-to-the-request for more info.
	AddReqHeader []string `split_words:"true"`
	// Additional headers that will be added to the response before forwarding it to the client.
	AddResHeader []string `split_words:"true"`
	// Whether to use `docker` as a check resolver. Set through the environment variable CHECK_RESOLVERS
	CheckResolvers bool `split_words:"true"`
	// One of the five connection modes supported by the HAProxy.
	// `http-keep-alive`: all requests and responses are processed.
	// `http-tunnel`: only the first request and response are processed, everything else is forwarded with no analysis.
	// `httpclose`: tunnel with "Connection: close" added in both directions.
	// `http-server-close`: the server-facing connection is closed after the response.
	// `forceclose`: the connection is actively closed after end of response.
	// In general, it is preferred to use http-server-close with application servers, and some static servers might benefit from http-keep-alive.
	// Connection mode is restricted to HTTP mode only.
	// If specified, connection mode will be applied to the backend section.
	ConnectionMode string `split_words:"true"`
	// The path to the Consul Template representing a snippet of the backend configuration.
	// If set, proxy template will be loaded from the specified file.
	ConsulTemplateBePath string `split_words:"true"`
	// The path to the Consul Template representing a snippet of the frontend configuration.
	// If specified, proxy template will be loaded from the specified file.
	ConsulTemplateFePath string `split_words:"true"`
	// Additional headers that will be deleted in the request before forwarding it to the service. Please consult https://www.haproxy.com/doc/aloha/7.0/haproxy/http_rewriting.html#delete-a-header-in-the-request for more info.
	DelReqHeader []string `split_words:"true"`
	// Additional headers that will be deleted in the response before forwarding it to the client. Please consult https://www.haproxy.com/doc/aloha/7.0/haproxy/http_rewriting.html#delete-a-header-in-the-response for more info.
	DelResHeader []string `split_words:"true"`
	// Whether to distribute a request to all the instances of the proxy.
	// Used only in the swarm mode.
	Distribute bool `split_words:"true"`
	// Whether to redirect all http requests to https
	HttpsOnly bool `split_words:"true"`
	// The internal HTTPS port of a service that should be reconfigured.
	// The port is used only in the swarm mode.
	// If not specified, the `port` parameter will be used instead.
	HttpsPort int `split_words:"true"`
	// If set to true, it will be the default_backend service.
	IsDefaultBackend bool `split_words:"true"`
	// The hostname where the service is running, for instance on a separate swarm.
	// If specified, the proxy will dispatch requests to that domain.
	OutboundHostname string `split_words:"true"`
	// The ACL derivative. Defaults to path_beg.
	// See https://cbonte.github.io/haproxy-dconv/configuration-1.5.html#7.3.6-path for more info.
	PathType string `split_words:"true"`
	// Whether to redirect to https when X-Forwarded-Proto is http
	RedirectWhenHttpProto bool `split_words:"true"`
	// Deprecated in favor of ReqPathReplace
	ReqRepReplace string `split_words:"true"`
	// Deprecated in favor of ReqPathSearch
	ReqRepSearch string `split_words:"true"`
	// A regular expression to apply the modification.
	// If specified, `reqPathSearch` needs to be set as well.
	ReqPathReplace string `split_words:"true"`
	// A regular expression to search the content to be replaced.
	// If specified, `reqPathReplace` needs to be set as well.
	ReqPathSearch string `split_words:"true"`
	// Content of the PEM-encoded certificate to be used by the proxy when serving traffic over SSL.
	ServiceCert string `split_words:"true"`
	// The domain of the service.
	// If set, the proxy will allow access only to requests coming to that domain.
	ServiceDomain []string `split_words:"true"`
	// Whether to include subdomains and FDQN domains in the match. If set to false, and, for example, `serviceDomain` is set to `acme.com`, `something.acme.com` would not be considered a match unless this parameter is set to `true`. If this option is used, it is recommended to put any subdomains higher in the list using `aclName`.
	ServiceDomainMatchAll bool `split_words:"true"`
	// The name of the service.
	// It must match the name of the Swarm service or the one stored in Consul.
	ServiceName string `split_words:"true"`
	// Additional headers that will be set to the request before forwarding it to the service. If a specified header exists, it will be replaced with the new one.
	SetReqHeader []string `split_words:"true"`
	// Additional headers that will be set to the response before forwarding it to the client. If a specified header exists, it will be replaced with the new one.
	SetResHeader []string `split_words:"true"`
	// If set to true, server certificates are not verified. This flag should be set for SSL enabled backend services.
	SslVerifyNone bool `split_words:"true"`
	// The path to the template representing a snippet of the backend configuration.
	// If specified, the backend template will be loaded from the specified file.
	// If specified, `templateFePath` must be set as well.
	// See the https://github.com/vfarcic/docker-flow-proxy#templates section for more info.
	TemplateBePath string `split_words:"true"`
	// The path to the template representing a snippet of the frontend configuration.
	// If specified, the frontend template will be loaded from the specified file.
	// If specified, `templateBePath` must be set as well.
	// See the https://github.com/vfarcic/docker-flow-proxy#templates section for more info.
	TemplateFePath string `split_words:"true"`
	// The server timeout in seconds
	TimeoutServer string `split_words:"true"`
	// The tunnel timeout in seconds
	TimeoutTunnel string `split_words:"true"`
	// A comma-separated list of credentials(<user>:<pass>) for HTTP basic auth, which applies only to the service that will be reconfigured.
	Users []User `split_words:"true"`
	// Whether to add "X-Forwarded-Proto https" header.
	XForwardedProto     bool `envconfig:"x_forwarded_proto" split_words:"true"`
	ServiceColor        string
	ServicePort         string
	AclCondition        string
	FullServiceName     string
	Host                string
	LookupRetry         int
	LookupRetryInterval int
	ServiceDest         []ServiceDest
}

// The list of services used inside the proxy
type Services []Service

func (slice Services) Len() int {
	return len(slice)
}

func (slice Services) Less(i, j int) bool {
	firstHasRoot := hasRoot(slice[i])
	secondHasRoot := hasRoot(slice[j])
	firstHasWellKnown := hasWellKnown(slice[i])
	secondHasWellKnown := hasWellKnown(slice[j])
	if firstHasWellKnown && !secondHasWellKnown {
		return true
	} else if !firstHasWellKnown && secondHasWellKnown {
		return false
	} else if firstHasRoot && !secondHasRoot {
		return false
	} else if !firstHasRoot && secondHasRoot {
		return true
	} else {
		return slice[i].AclName < slice[j].AclName
	}
}

func hasRoot(service Service) bool {
	for _, sd := range service.ServiceDest {
		for _, path := range sd.ServicePath {
			if path == "/" {
				return true
			}
		}
	}
	return false
}

func hasWellKnown(service Service) bool {
	for _, sd := range service.ServiceDest {
		for _, path := range sd.ServicePath {
			if strings.HasPrefix(strings.ToLower(path), "/.well-known") {
				return true
			}
		}
	}
	return false
}

func (slice Services) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

func ExtractUsersFromString(context, usersString string, encrypted, skipEmptyPassword bool) []*User {
	collectedUsers := []*User{}
	// TODO: Test
	if len(usersString) == 0 {
		return collectedUsers
	}
	splitter := func(x rune) bool {
		return x == '\n' || x == ','
	}
	users := strings.FieldsFunc(usersString, splitter)
	for _, user := range users {
		user = strings.Trim(user, "\n\t ")
		if len(user) == 0 {
			continue
		}
		if strings.Contains(user, ":") {
			colonIndex := strings.Index(user, ":")
			userName := strings.Trim(user[0:colonIndex], "\t ")
			userPass := strings.Trim(user[colonIndex+1:], "\t ")
			if len(userName) == 0 || len(userPass) == 0 {
				logPrintf("There is a user with no name or with invalid format for the service %s", context)
			} else {
				collectedUsers = append(collectedUsers, &User{Username: userName, Password: userPass, PassEncrypted: encrypted})
			}
		} else {
			if len(user) == 0 { // TODO: Test
				logPrintf("There is a user with no name or with invalid format for the service %s", context)
			} else if skipEmptyPassword { // TODO: Test
				logPrintf(
					"For service %s There is a user %s with no password for the service %s",
					user,
					context,
				)
			} else if !skipEmptyPassword {
				collectedUsers = append(collectedUsers, &User{Username: user})
			}
		}
	}
	return collectedUsers
}

type ServiceParameterProvider interface {
	Fill(service *Service)
	GetString(name string) string
}

type MapParameterProvider struct {
	theMap *map[string]string
}

func (p *MapParameterProvider) Fill(service *Service) {
	mapstructure.Decode(p.theMap, service)
	//above library does not handle bools as strings
	v := reflect.ValueOf(service).Elem()
	for i := 0; i < v.NumField(); i++ {
		if v.Field(i).CanSet() && v.Field(i).Kind() == reflect.Bool {
			fieldName := v.Type().Field(i).Name
			value := ""
			if len(p.GetString(fieldName)) > 0 {
				value = p.GetString(fieldName)
			} else if len(p.GetString(LowerFirst(fieldName))) > 0 {
				value = p.GetString(LowerFirst(fieldName))
			}
			value = strings.ToLower(value)
			if strings.EqualFold(value, "true") {
				v.Field(i).SetBool(true)
			} else if strings.EqualFold(value, "false") {
				v.Field(i).SetBool(false)
			}
		}
	}
}

func (p *MapParameterProvider) GetString(name string) string {
	return (*p.theMap)[name]
}

func GetServiceFromMap(req *map[string]string) *Service {
	provider := MapParameterProvider{theMap: req}
	return GetServiceFromProvider(&provider)
}

func GetServiceFromProvider(provider ServiceParameterProvider) *Service {
	sr := new(Service)
	provider.Fill(sr)
	if len(provider.GetString("httpsPort")) > 0 {
		sr.HttpsPort, _ = strconv.Atoi(provider.GetString("httpsPort"))
	}
	if len(provider.GetString("serviceDomain")) > 0 {
		sr.ServiceDomain = strings.Split(provider.GetString("serviceDomain"), ",")
	}
	if len(provider.GetString("addReqHeader")) > 0 {
		sr.AddReqHeader = strings.Split(provider.GetString("addReqHeader"), ",")
	} else if len(provider.GetString("addHeader")) > 0 { // TODO: Deprecated since Apr. 2017.
		sr.AddReqHeader = strings.Split(provider.GetString("addHeader"), ",")
	}
	if len(provider.GetString("setReqHeader")) > 0 {
		sr.SetReqHeader = strings.Split(provider.GetString("setReqHeader"), ",")
	} else if len(provider.GetString("setHeader")) > 0 { // TODO: Deprecated since Apr. 2017.
		sr.SetReqHeader = strings.Split(provider.GetString("setHeader"), ",")
	}
	if len(provider.GetString("delReqHeader")) > 0 {
		sr.DelReqHeader = strings.Split(provider.GetString("delReqHeader"), ",")
	}
	if len(provider.GetString("addResHeader")) > 0 {
		sr.AddResHeader = strings.Split(provider.GetString("addResHeader"), ",")
	}
	if len(provider.GetString("setResHeader")) > 0 {
		sr.SetResHeader = strings.Split(provider.GetString("setResHeader"), ",")
	}
	if len(provider.GetString("delResHeader")) > 0 {
		sr.DelResHeader = strings.Split(provider.GetString("delResHeader"), ",")
	}
	globalUsersString := GetSecretOrEnvVar("USERS", "")
	globalUsersEncrypted := strings.EqualFold(GetSecretOrEnvVar("USERS_PASS_ENCRYPTED", ""), "true")
	sr.Users = mergeUsers(
		sr.ServiceName,
		provider.GetString("users"),
		provider.GetString("usersSecret"),
		getBoolParam(provider, "usersPassEncrypted"),
		globalUsersString,
		globalUsersEncrypted,
	)

	sr.ServiceDest = getServiceDestList(sr, provider)
	return sr
}

func getServiceDestList(sr *Service, provider ServiceParameterProvider) []ServiceDest {
	sdList := []ServiceDest{}
	sd := getServiceDest(sr, provider, -1)
	if isServiceDestValid(&sd) || (len(sr.ConsulTemplateFePath) > 0 && len(sr.ConsulTemplateBePath) > 0) {
		sdList = append(sdList, sd)
	}
	for i := 1; i <= 10; i++ {
		sd := getServiceDest(sr, provider, i)
		if isIndexedServiceDestValid(&sd) {
			sdList = append(sdList, sd)
		} else {
			break
		}
	}
	if len(sdList) == 0 {
		reqMode := "http"
		if len(provider.GetString("reqMode")) > 0 {
			reqMode = provider.GetString("reqMode")
		}
		sdList = append(sdList, ServiceDest{ReqMode: reqMode})
	}
	if len(sr.ServiceDomain) > 0 {
		for i := range sdList {
			if len(sdList[i].ServicePath) == 0 {
				sdList[i].ServicePath = []string{"/"}
			}
		}
	}
	return sdList
}

func getServiceDest(sr *Service, provider ServiceParameterProvider, index int) ServiceDest {
	suffix := ""
	if index > 0 {
		suffix = fmt.Sprintf(".%d", index)
	}
	path := []string{}
	userAgent := UserAgent{}
	if len(provider.GetString(fmt.Sprintf("servicePath%s", suffix))) > 0 {
		path = strings.Split(provider.GetString(fmt.Sprintf("servicePath%s", suffix)), ",")
	}
	if len(provider.GetString(fmt.Sprintf("userAgent%s", suffix))) > 0 {
		userAgent.Value = strings.Split(provider.GetString(fmt.Sprintf("userAgent%s", suffix)), ",")
		userAgent.AclName = replaceNonAlphabetAndNumbers(userAgent.Value)
	}
	reqMode := "http"
	if len(provider.GetString(fmt.Sprintf("reqMode%s", suffix))) > 0 {
		reqMode = provider.GetString(fmt.Sprintf("reqMode%s", suffix))
	}
	port := provider.GetString(fmt.Sprintf("port%s", suffix))
	srcPort, _ := strconv.Atoi(provider.GetString(fmt.Sprintf("srcPort%s", suffix)))
	verifyClientSsl := getBoolParam(provider, fmt.Sprintf("verifyClientSsl%s", suffix))
	return ServiceDest{
		Port: port,
		ReqMode: reqMode,
		SrcPort: srcPort,
		ServicePath: path,
		VerifyClientSsl: verifyClientSsl,
		UserAgent: userAgent,
	}

}

func isServiceDestValid(sd *ServiceDest) bool {
	return len(sd.ServicePath) > 0 || len(sd.Port) > 0
}

func isIndexedServiceDestValid(sd *ServiceDest) bool {
	return len(sd.ServicePath) > 0 && len(sd.Port) > 0
}

func getBoolParam(req ServiceParameterProvider, param string) bool {
	value := false
	if len(req.GetString(param)) > 0 {
		value, _ = strconv.ParseBool(req.GetString(param))
	}
	return value
}

func mergeUsers(
	serviceName,
	usersParam,
	usersSecret string,
	usersPassEncrypted bool,
	globalUsersString string,
	globalUsersEncrypted bool,
) []User {
	var collectedUsers []*User
	paramUsers := extractUsersFromString(serviceName, usersParam, usersPassEncrypted, false)
	fileUsers, _ := getUsersFromFile(serviceName, usersSecret, usersPassEncrypted)
	if len(paramUsers) > 0 {
		if !allUsersHavePasswords(paramUsers) {
			if len(usersSecret) == 0 {
				fileUsers = ExtractUsersFromString(serviceName, globalUsersString, globalUsersEncrypted, true)
			}
			for _, u := range paramUsers {
				if !u.HasPassword() {
					if userByName := findUserByName(fileUsers, u.Username); userByName != nil {
						u.Password = "sdasdsad"
						u.Password = userByName.Password
						u.PassEncrypted = userByName.PassEncrypted
					} else {
						// TODO: Return an error
						// TODO: Test
						logPrintf("For service %s it was impossible to find password for user %s.",
							serviceName, u.Username)
					}
				}
			}
		}
		collectedUsers = paramUsers
	} else {
		collectedUsers = fileUsers
	}
	ret := []User{}
	for _, u := range collectedUsers {
		if u.HasPassword() {
			ret = append(ret, *u)
		}
	}
	if len(ret) == 0 && (len(usersParam) != 0 || len(usersSecret) != 0) {
		//we haven't found any users but they were requested so generating dummy one
		ret = append(ret, *RandomUser())
	}
	if len(ret) == 0 {
		return nil
	}
	return ret
}

func getUsersFromFile(serviceName, fileName string, passEncrypted bool) ([]*User, error) {
	if len(fileName) > 0 {
		usersFile := fmt.Sprintf(usersBasePath, fileName)
		content, err := readFile(usersFile)
		if err != nil {
			logPrintf(
				"For service %s it was impossible to load userFile %s due to error %s",
				serviceName,
				usersFile,
				err.Error(),
			)
			return []*User{}, err
		}
		userContents := strings.TrimRight(string(content[:]), "\n")
		return ExtractUsersFromString(serviceName, userContents, passEncrypted, true), nil
	}
	return []*User{}, nil
}

func allUsersHavePasswords(users []*User) bool {
	for _, u := range users {
		if !u.HasPassword() {
			return false
		}
	}
	return true
}

func findUserByName(users []*User, name string) *User {
	for _, u := range users {
		if strings.EqualFold(name, u.Username) {
			return u
		}
	}
	return nil
}
