package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	haproxy "./proxy"
	"./registry"
)

const ServiceTemplateFeFilename = "service-formatted-fe.ctmpl"
const ServiceTemplateBeFilename = "service-formatted-be.ctmpl"

var mu = &sync.Mutex{}

type Reconfigurable interface {
	Executable
	GetData() (BaseReconfigure, ServiceReconfigure)
	ReloadAllServices(addresses []string, instanceName, mode, listenerAddress string) error
	GetTemplates(sr ServiceReconfigure) (front, back string, err error)
}

type Reconfigure struct {
	BaseReconfigure
	ServiceReconfigure
}

type User struct {
	Username string
	Password string
}

type ServiceReconfigure struct {
	ServiceName          string   `short:"s" long:"service-name" required:"true" description:"The name of the service that should be reconfigured (e.g. my-service)."`
	ServiceColor         string   `short:"C" long:"service-color" description:"The color of the service release in case blue-green deployment is performed (e.g. blue)."`
	ServicePath          []string `short:"p" long:"service-path" description:"Path that should be configured in the proxy (e.g. /api/v1/my-service)."`
	ServicePort          string
	ServiceDomain        []string `long:"service-domain" description:"The domain of the service. If specified, proxy will allow access only to requests coming from that domain (e.g. my-domain.com)."`
	ServiceCert          string   `long:"service-cert" description:"Content of the PEM-encoded certificate to be used by the proxy when serving traffic over SSL."`
	OutboundHostname     string   `long:"outbound-hostname" description:"The hostname running the service. If specified, proxy will redirect traffic to this hostname instead of using the service's name."`
	ConsulTemplateFePath string   `long:"consul-template-fe-path" description:"The path to the Consul Template representing snippet of the frontend configuration. If specified, proxy template will be loaded from the specified file."`
	ConsulTemplateBePath string   `long:"consul-template-be-path" description:"The path to the Consul Template representing snippet of the backend configuration. If specified, proxy template will be loaded from the specified file."`
	Mode                 string   `short:"m" long:"mode" env:"MODE" description:"If set to 'swarm', proxy will operate assuming that Docker service from v1.12+ is used."`
	PathType             string
	Port                 string
	SkipCheck            bool
	Acl                  string
	AclName              string
	AclCondition         string
	Users                []User
	FullServiceName      string
	Host                 string
	Distribute           bool
	LookupRetry          int
	LookupRetryInterval  int
	ReqRepSearch         string
	ReqRepReplace		 string
}

type BaseReconfigure struct {
	ConsulAddresses       []string
	ConfigsPath           string `short:"c" long:"configs-path" default:"/cfg" description:"The path to the configurations directory"`
	InstanceName          string `long:"proxy-instance-name" env:"PROXY_INSTANCE_NAME" default:"docker-flow" required:"true" description:"The name of the proxy instance."`
	TemplatesPath         string `short:"t" long:"templates-path" default:"/cfg/tmpl" description:"The path to the templates directory"`
	skipAddressValidation bool
}

var reconfigure Reconfigure

var NewReconfigure = func(baseData BaseReconfigure, serviceData ServiceReconfigure) Reconfigurable {
	return &Reconfigure{baseData, serviceData}
}

// TODO: Remove args
func (m *Reconfigure) Execute(args []string) error {
	mu.Lock()
	defer mu.Unlock()
	if isSwarm(m.ServiceReconfigure.Mode) && !m.skipAddressValidation {
		host := m.ServiceName
		if len(m.OutboundHostname) > 0 {
			host = m.OutboundHostname
		}
		if _, err := lookupHost(host); err != nil {
			logPrintf("Could not reach the service %s. Is the service running and connected to the same network as the proxy?", host)
			return err
		}
	}
	if err := m.createConfigs(m.TemplatesPath, &m.ServiceReconfigure); err != nil {
		return err
	}
	if err := haproxy.Instance.CreateConfigFromTemplates(); err != nil {
		return err
	}
	if err := haproxy.Instance.Reload(); err != nil {
		return err
	}
	if len(m.ConsulAddresses) > 0 || !isSwarm(m.ServiceReconfigure.Mode) {
		if err := m.putToConsul(m.ConsulAddresses, m.ServiceReconfigure, m.InstanceName); err != nil {
			return err
		}
	}
	return nil
}

func (m *Reconfigure) GetData() (BaseReconfigure, ServiceReconfigure) {
	return m.BaseReconfigure, m.ServiceReconfigure
}

func (m *Reconfigure) ReloadAllServices(addresses []string, instanceName, mode, listenerAddress string) error {
	if len(listenerAddress) > 0 {
		fullAddress := fmt.Sprintf("%s/v1/docker-flow-swarm-listener/notify-services", listenerAddress)
		resp, err := httpGet(fullAddress)
		if err != nil {
			return err
		} else if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Swarm Listener responded with the status code %d", resp.StatusCode)
		}
		logPrintf("A request was sent to the Swarm listener running on %s. The proxy will be reconfigured soon.", listenerAddress)
	} else if len(addresses) > 0 || !isSwarm(mode) {
		return m.reloadFromRegistry(addresses, instanceName, mode)
	}
	return nil
}

func (m *Reconfigure) reloadFromRegistry(addresses []string, instanceName, mode string) error {
	var resp *http.Response
	var err error
	logPrintf("Configuring existing services")
	found := false
	for _, address := range addresses {
		var servicesUrl string
		address = strings.ToLower(address)
		if !strings.HasPrefix(address, "http") {
			address = fmt.Sprintf("http://%s", address)
		}
		if isSwarm(mode) {
			// TODO: Test
			servicesUrl = fmt.Sprintf("%s/v1/kv/docker-flow/service?recurse", address)
		} else {
			servicesUrl = fmt.Sprintf("%s/v1/catalog/services", address)
		}
		resp, err = http.Get(servicesUrl)
		if err == nil {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("Could not retrieve the list of services from Consul")
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	c := make(chan ServiceReconfigure)
	count := 0
	if isSwarm(mode) {
		// TODO: Test
		type Key struct {
			Value string `json:"Key"`
		}
		data := []Key{}
		json.Unmarshal(body, &data)
		count = len(data)
		for _, key := range data {
			parts := strings.Split(key.Value, "/")
			serviceName := parts[len(parts)-1]
			go m.getService(addresses, serviceName, instanceName, c)
		}
	} else {
		var data map[string]interface{}
		json.Unmarshal(body, &data)
		count = len(data)
		for key, _ := range data {
			go m.getService(addresses, key, instanceName, c)
		}
	}
	logPrintf("\tFound %d services", count)
	for i := 0; i < count; i++ {
		s := <-c
		s.Mode = mode
		if len(s.ServicePath) > 0 {
			logPrintf("\tConfiguring %s", s.ServiceName)
			m.createConfigs(m.TemplatesPath, &s)
		}
	}
	if err := haproxy.Instance.CreateConfigFromTemplates(); err != nil {
		return err
	}
	return haproxy.Instance.Reload()
}

func (m *Reconfigure) getService(addresses []string, serviceName, instanceName string, c chan ServiceReconfigure) {
	sr := ServiceReconfigure{ServiceName: serviceName}

	path, err := registryInstance.GetServiceAttribute(addresses, serviceName, registry.PATH_KEY, instanceName)
	domain, err := registryInstance.GetServiceAttribute(addresses, serviceName, registry.DOMAIN_KEY, instanceName)
	if err == nil {
		sr.ServicePath = strings.Split(path, ",")
		sr.ServiceColor, _ = m.getServiceAttribute(addresses, serviceName, registry.COLOR_KEY, instanceName)
		sr.ServiceDomain = strings.Split(domain, ",")
		sr.ServiceCert, _ = m.getServiceAttribute(addresses, serviceName, registry.CERT_KEY, instanceName)
		sr.OutboundHostname, _ = m.getServiceAttribute(addresses, serviceName, registry.HOSTNAME_KEY, instanceName)
		sr.PathType, _ = m.getServiceAttribute(addresses, serviceName, registry.PATH_TYPE_KEY, instanceName)
		skipCheck, _ := m.getServiceAttribute(addresses, serviceName, registry.SKIP_CHECK_KEY, instanceName)
		sr.SkipCheck, _ = strconv.ParseBool(skipCheck)
		sr.ConsulTemplateFePath, _ = m.getServiceAttribute(addresses, serviceName, registry.CONSUL_TEMPLATE_FE_PATH_KEY, instanceName)
		sr.ConsulTemplateBePath, _ = m.getServiceAttribute(addresses, serviceName, registry.CONSUL_TEMPLATE_BE_PATH_KEY, instanceName)
		sr.Port, _ = m.getServiceAttribute(addresses, serviceName, registry.PORT, instanceName)
	}
	c <- sr
}

// TODO: Remove in favour of registry.GetServiceAttribute
func (m *Reconfigure) getServiceAttribute(addresses []string, serviceName, key, instanceName string) (string, bool) {
	for _, address := range addresses {
		url := fmt.Sprintf("%s/v1/kv/%s/%s/%s?raw", address, instanceName, serviceName, key)
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			return string(body), true
		}
	}
	return "", false
}

func (m *Reconfigure) createConfigs(templatesPath string, sr *ServiceReconfigure) error {
	logPrintf("Creating configuration for the service %s", sr.ServiceName)
	feTemplate, beTemplate, err := m.GetTemplates(*sr)
	if err != nil {
		return err
	}
	if strings.EqualFold(sr.Mode, "service") || strings.EqualFold(sr.Mode, "swarm") {
		if len(sr.AclName) == 0 {
			sr.AclName = sr.ServiceName
		}
		destFe := fmt.Sprintf("%s/%s-fe.cfg", templatesPath, sr.AclName)
		writeFeTemplate(destFe, []byte(feTemplate), 0664)
		destBe := fmt.Sprintf("%s/%s-be.cfg", templatesPath, sr.AclName)
		writeBeTemplate(destBe, []byte(beTemplate), 0664)
	} else {
		args := registry.CreateConfigsArgs{
			Addresses:     m.ConsulAddresses,
			TemplatesPath: templatesPath,
			FeFile:        ServiceTemplateFeFilename,
			FeTemplate:    feTemplate,
			BeFile:        ServiceTemplateBeFilename,
			BeTemplate:    beTemplate,
			ServiceName:   sr.ServiceName,
		}
		if err = registryInstance.CreateConfigs(&args); err != nil {
			return err
		}
	}
	return nil
}

func (m *Reconfigure) putToConsul(addresses []string, sr ServiceReconfigure, instanceName string) error {
	r := registry.Registry{
		ServiceName:          sr.ServiceName,
		ServiceColor:         sr.ServiceColor,
		ServicePath:          sr.ServicePath,
		ServiceDomain:        sr.ServiceDomain,
		ServiceCert:          sr.ServiceCert,
		OutboundHostname:     sr.OutboundHostname,
		PathType:             sr.PathType,
		SkipCheck:            sr.SkipCheck,
		ConsulTemplateFePath: sr.ConsulTemplateFePath,
		ConsulTemplateBePath: sr.ConsulTemplateBePath,
		Port:                 sr.Port,
	}
	if err := registryInstance.PutService(addresses, instanceName, r); err != nil {
		return err
	}
	return nil
}

//This function should replace the getTemplateFromGo last lines.
func (m *Reconfigure) parseTemplate(front, back string, sr ServiceReconfigure) (pFront, pBack string) {
	tmplFront, _ := template.New("consulTemplate").Parse(front)
	tmplBack, _ := template.New("consulTemplate").Parse(back)
	var ctFront bytes.Buffer
	var ctBack bytes.Buffer
	tmplFront.Execute(&ctFront, sr)
	tmplBack.Execute(&ctBack, sr)
	return ctFront.String(), ctBack.String()
}

func (m *Reconfigure) GetTemplates(sr ServiceReconfigure) (front, back string, err error) {
	if len(sr.ConsulTemplateFePath) > 0 && len(sr.ConsulTemplateBePath) > 0 {
		front, err = m.getConsulTemplateFromFile(sr.ConsulTemplateFePath)
		if err != nil {
			return "", "", err
		}
		back, err = m.getConsulTemplateFromFile(sr.ConsulTemplateBePath)
		if err != nil {
			return "", "", err
		}

		//TODO: remove this line when Test are ready.
		front, back = m.parseTemplate(front, back, sr)
	} else {
		front, back = m.getTemplateFromGo(sr)
	}

	//TODO: remove comment when Tests are ready.
	//This requires rewrite test and remove last lines of getTemplateFromGo (Best solution).
	//front, back = m.parseTemplate(front, back, sr)

	return front, back, nil
}

func (m *Reconfigure) getTemplateFromGo(sr ServiceReconfigure) (frontend, backend string) {
	sr.Acl = ""
	sr.AclCondition = ""
	if len(sr.AclName) == 0 {
		sr.AclName = sr.ServiceName
	}
	sr.Host = m.ServiceName
	if len(m.OutboundHostname) > 0 {
		sr.Host = m.OutboundHostname
	}
	if len(sr.ServiceDomain) > 0 {
		sr.Acl = `
    acl domain_{{.ServiceName}} hdr_dom(host) -i{{range .ServiceDomain}} {{.}}{{end}}`
		sr.AclCondition = fmt.Sprintf(" domain_%s", sr.ServiceName)
	}
	if len(sr.ServiceColor) > 0 {
		sr.FullServiceName = fmt.Sprintf("%s-%s", sr.ServiceName, sr.ServiceColor)
	} else {
		sr.FullServiceName = sr.ServiceName
	}
	if len(sr.PathType) == 0 {
		sr.PathType = "path_beg"
	}
	srcFront := fmt.Sprintf(
		`
    acl url_{{.ServiceName}}{{range .ServicePath}} {{$.PathType}} {{.}}{{end}}%s
    use_backend {{.AclName}}-be if url_{{.ServiceName}}{{.AclCondition}}`,
		sr.Acl,
	)
//	if len(sr.ReqRepSearch) > 0 && len(sr.ReqRepReplace) > 0 {
//		srcFront += `
//    reqrep {{.ReqRepSearch}} {{.ReqRepReplace}} if url_{{.ServiceName}}`
//	}
	srcBack := ""
	if len(sr.Users) > 0 {
		srcBack += `userlist {{.ServiceName}}Users{{range .Users}}
    user {{.Username}} insecure-password {{.Password}}{{end}}

`
	}
	srcBack += `backend {{.AclName}}-be
    mode http`
	if len(sr.ReqRepSearch) > 0 && len(sr.ReqRepReplace) > 0 {
		srcBack += `
    reqrep {{.ReqRepSearch}}     {{.ReqRepReplace}}`
	}
	if strings.EqualFold(sr.Mode, "service") || strings.EqualFold(sr.Mode, "swarm") {
		srcBack += `
    server {{.ServiceName}} {{.Host}}:{{.Port}}`
	} else { // It's Consul
		srcBack += `
    {{"{{"}}range $i, $e := service "{{.FullServiceName}}" "any"{{"}}"}}
    server {{"{{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}}"}}{{if eq .SkipCheck false}} check{{end}}
    {{"{{end}}"}}`
	}
	if len(sr.Users) > 0 {
		srcBack += `
    acl {{.ServiceName}}UsersAcl http_auth({{.ServiceName}}Users)
    http-request auth realm {{.ServiceName}}Realm if !{{.ServiceName}}UsersAcl`
	} else 	if len(os.Getenv("USERS")) > 0 {
		srcBack += `
    acl defaultUsersAcl http_auth(defaultUsers)
    http-request auth realm defaultRealm if !defaultUsersAcl`
	}

	//TODO: remove this lines when Tests are ready.
	tmplFront, _ := template.New("consulTemplate").Parse(srcFront)
	tmplBack, _ := template.New("consulTemplate").Parse(srcBack)
	var ctFront bytes.Buffer
	var ctBack bytes.Buffer
	tmplFront.Execute(&ctFront, sr)
	tmplBack.Execute(&ctBack, sr)

	return ctFront.String(), ctBack.String()

	//TODO: remove this comment when Tests are ready.
	//return srcFront, srcBack
}

// TODO: Move to registry package
func (m *Reconfigure) getConsulTemplateFromFile(path string) (string, error) {
	content, err := readTemplateFile(path)
	if err != nil {
		return "", fmt.Errorf("Could not read the file %s\n%s", path, err.Error())
	}
	return string(content), nil
}
