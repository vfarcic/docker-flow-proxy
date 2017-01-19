package actions

import (
	"../proxy"
	"../registry"
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
)

const ServiceTemplateFeFilename = "service-formatted-fe.ctmpl"
const ServiceTemplateBeFilename = "service-formatted-be.ctmpl"

var mu = &sync.Mutex{}

type Reconfigurable interface {
	Executable
	GetData() (BaseReconfigure, proxy.Service)
	ReloadAllServices(addresses []string, instanceName, mode, listenerAddress string) error
	GetTemplates(sr *proxy.Service) (front, back string, err error)
}

type Reconfigure struct {
	BaseReconfigure
	proxy.Service
	Mode string `short:"m" long:"mode" env:"MODE" description:"If set to 'swarm', proxy will operate assuming that Docker service from v1.12+ is used."`
}

type BaseReconfigure struct {
	ConsulAddresses       []string
	ConfigsPath           string `short:"c" long:"configs-path" default:"/cfg" description:"The path to the configurations directory"`
	InstanceName          string `long:"proxy-instance-name" env:"PROXY_INSTANCE_NAME" default:"docker-flow" required:"true" description:"The name of the proxy instance."`
	TemplatesPath         string `short:"t" long:"templates-path" default:"/cfg/tmpl" description:"The path to the templates directory"`
	skipAddressValidation bool
}

var ReconfigureInstance Reconfigure

var NewReconfigure = func(baseData BaseReconfigure, serviceData proxy.Service, mode string) Reconfigurable {
	return &Reconfigure{
		BaseReconfigure: baseData,
		Service:         serviceData,
		Mode:            mode,
	}
}

// TODO: Remove args
func (m *Reconfigure) Execute(args []string) error {
	mu.Lock()
	defer mu.Unlock()
	if isSwarm(m.Mode) && !m.skipAddressValidation {
		host := m.ServiceName
		if len(m.OutboundHostname) > 0 {
			host = m.OutboundHostname
		}
		if _, err := lookupHost(host); err != nil {
			logPrintf("Could not reach the service %s. Is the service running and connected to the same network as the proxy?", host)
			return err
		}
	}
	if err := m.createConfigs(m.TemplatesPath, &m.Service); err != nil {
		return err
	}
	if len(m.ConsulTemplateBePath) == 0 && len(m.ConsulTemplateFePath) == 0 {
		proxy.Instance.AddService(m.Service)
	}
	if err := proxy.Instance.CreateConfigFromTemplates(); err != nil {
		return err
	}
	reload := Reload{}
	if err := reload.Execute(); err != nil {
		return err
	}
	if len(m.ConsulAddresses) > 0 || !isSwarm(m.Mode) {
		if err := m.putToConsul(m.ConsulAddresses, m.Service, m.InstanceName); err != nil {
			return err
		}
	}
	return nil
}

func (m *Reconfigure) GetData() (BaseReconfigure, proxy.Service) {
	return m.BaseReconfigure, m.Service
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
	c := make(chan proxy.Service)
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
		if len(s.ServiceDest) > 0 && len(s.ServiceDest[0].ServicePath) > 0 {
			logPrintf("\tConfiguring %s", s.ServiceName)
			m.createConfigs(m.TemplatesPath, &s)
		}
	}
	if err := proxy.Instance.CreateConfigFromTemplates(); err != nil {
		return err
	}
	reload := Reload{}
	return reload.Execute()
}

func (m *Reconfigure) getService(addresses []string, serviceName, instanceName string, c chan proxy.Service) {
	sr := proxy.Service{ServiceName: serviceName}

	path, err := registryInstance.GetServiceAttribute(addresses, serviceName, registry.PATH_KEY, instanceName)
	domain, err := registryInstance.GetServiceAttribute(addresses, serviceName, registry.DOMAIN_KEY, instanceName)
	port, _ := m.getServiceAttribute(addresses, serviceName, registry.PORT, instanceName)
	sd := proxy.ServiceDest{
		ServicePath: strings.Split(path, ","),
		Port:        port,
	}
	if err == nil {
		sr.ServiceDest = []proxy.ServiceDest{sd}
		sr.ServiceColor, _ = m.getServiceAttribute(addresses, serviceName, registry.COLOR_KEY, instanceName)
		sr.ServiceDomain = strings.Split(domain, ",")
		sr.ServiceCert, _ = m.getServiceAttribute(addresses, serviceName, registry.CERT_KEY, instanceName)
		sr.OutboundHostname, _ = m.getServiceAttribute(addresses, serviceName, registry.HOSTNAME_KEY, instanceName)
		sr.PathType, _ = m.getServiceAttribute(addresses, serviceName, registry.PATH_TYPE_KEY, instanceName)
		skipCheck, _ := m.getServiceAttribute(addresses, serviceName, registry.SKIP_CHECK_KEY, instanceName)
		sr.SkipCheck, _ = strconv.ParseBool(skipCheck)
		sr.ConsulTemplateFePath, _ = m.getServiceAttribute(addresses, serviceName, registry.CONSUL_TEMPLATE_FE_PATH_KEY, instanceName)
		sr.ConsulTemplateBePath, _ = m.getServiceAttribute(addresses, serviceName, registry.CONSUL_TEMPLATE_BE_PATH_KEY, instanceName)
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

func (m *Reconfigure) createConfigs(templatesPath string, sr *proxy.Service) error {
	logPrintf("Creating configuration for the service %s", sr.ServiceName)
	feTemplate, beTemplate, err := m.GetTemplates(sr)
	if err != nil {
		return err
	}
	if strings.EqualFold(m.Mode, "service") || strings.EqualFold(m.Mode, "swarm") {
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

func (m *Reconfigure) putToConsul(addresses []string, sr proxy.Service, instanceName string) error {
	path := []string{}
	port := ""
	if len(sr.ServiceDest) > 0 {
		path = sr.ServiceDest[0].ServicePath
		port = sr.ServiceDest[0].Port
	}
	r := registry.Registry{
		ServiceName:          sr.ServiceName,
		ServiceColor:         sr.ServiceColor,
		ServicePath:          path,
		ServiceDomain:        sr.ServiceDomain,
		ServiceCert:          sr.ServiceCert,
		OutboundHostname:     sr.OutboundHostname,
		PathType:             sr.PathType,
		SkipCheck:            sr.SkipCheck,
		ConsulTemplateFePath: sr.ConsulTemplateFePath,
		ConsulTemplateBePath: sr.ConsulTemplateBePath,
		Port:                 port,
	}
	if err := registryInstance.PutService(addresses, instanceName, r); err != nil {
		return err
	}
	return nil
}

func (m *Reconfigure) GetTemplates(sr *proxy.Service) (front, back string, err error) {
	if len(sr.TemplateFePath) > 0 && len(sr.TemplateBePath) > 0 {
		feTmpl, err := readTemplateFile(sr.TemplateFePath)
		if err != nil {
			return "", "", err
		}
		beTmpl, err := readTemplateFile(sr.TemplateBePath)
		if err != nil {
			return "", "", err
		}
		front, back = m.parseTemplate(string(feTmpl), "", string(beTmpl), sr)
	} else if len(sr.ConsulTemplateFePath) > 0 && len(sr.ConsulTemplateBePath) > 0 { // Sunset
		front, err = m.getConsulTemplateFromFile(sr.ConsulTemplateFePath)
		if err != nil {
			return "", "", err
		}
		back, err = m.getConsulTemplateFromFile(sr.ConsulTemplateBePath)
		if err != nil {
			return "", "", err
		}
	} else {
		if len(sr.ReqMode) == 0 {
			sr.ReqMode = "http"
		}
		m.formatData(sr)
		front, back = m.parseTemplate(
			"",
			m.getUsersList(sr),
			m.getBackTemplate(sr),
			sr)
	}
	return front, back, nil
}

// TODO: Move to ha_proxy.go
func (m *Reconfigure) formatData(sr *proxy.Service) {
	sr.AclCondition = ""
	if len(sr.AclName) == 0 {
		sr.AclName = sr.ServiceName
	}
	sr.Host = m.ServiceName
	if len(m.OutboundHostname) > 0 {
		sr.Host = m.OutboundHostname
	}
	if len(sr.ServiceColor) > 0 {
		sr.FullServiceName = fmt.Sprintf("%s-%s", sr.ServiceName, sr.ServiceColor)
	} else {
		sr.FullServiceName = sr.ServiceName
	}
	if len(sr.PathType) == 0 {
		sr.PathType = "path_beg"
	}
	for i, sd := range sr.ServiceDest {
		if sd.SrcPort > 0 {
			sr.ServiceDest[i].SrcPortAclName = fmt.Sprintf(" srcPort_%s%d", sr.ServiceName, sd.SrcPort)
			sr.ServiceDest[i].SrcPortAcl = fmt.Sprintf(`
    acl srcPort_%s%d dst_port %d`, sr.ServiceName, sd.SrcPort, sd.SrcPort)
		}
	}
}

// TODO: Move to ha_proxy.go
func (m *Reconfigure) getBackTemplate(sr *proxy.Service) string {
	back := m.getBackTemplateProtocol("http", sr)
	if sr.HttpsPort > 0 {
		back += fmt.Sprintf(
			`
%s`,
			m.getBackTemplateProtocol("https", sr))
	}
	return back
}

func (m *Reconfigure) getBackTemplateProtocol(protocol string, sr *proxy.Service) string {
	prefix := ""
	if strings.EqualFold(protocol, "https") {
		prefix = "https-"
	}
	proxy_mode := "http"
	if len(os.Getenv("PROXY_MODE")) > 0 {
		proxy_mode = os.Getenv("PROXY_MODE")
	}
	tmpl := fmt.Sprintf(`{{range .ServiceDest}}
backend %s{{$.AclName}}-be{{.Port}}
    mode %s`,
		prefix,
		proxy_mode,
	)
	// TODO: Deprecated (dec. 2016).
	if len(sr.TimeoutServer) > 0 {
		tmpl += `
    timeout server {{$.TimeoutServer}}s`
	}
	if len(sr.TimeoutTunnel) > 0 {
		tmpl += `
    timeout tunnel {{$.TimeoutTunnel}}s`
	}
	if len(sr.ReqRepSearch) > 0 && len(sr.ReqRepReplace) > 0 {
		tmpl += `
    reqrep {{$.ReqRepSearch}}     {{$.ReqRepReplace}}`
	}
	if len(sr.ReqPathSearch) > 0 && len(sr.ReqPathReplace) > 0 {
		tmpl += `
    http-request set-path %[path,regsub({{$.ReqPathSearch}},{{$.ReqPathReplace}})]`
	}
	if strings.EqualFold(m.Mode, "service") || strings.EqualFold(m.Mode, "swarm") {
		if strings.EqualFold(protocol, "https") {
			tmpl += `
    server {{$.ServiceName}} {{$.Host}}:{{$.HttpsPort}}`
		} else {
			tmpl += `
    server {{$.ServiceName}} {{$.Host}}:{{.Port}}`
		}
	} else { // It's Consul
		tmpl += `
    {{"{{"}}range $i, $e := service "{{$.FullServiceName}}" "any"{{"}}"}}
    server {{"{{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}}"}}{{if eq $.SkipCheck false}} check{{end}}
    {{"{{end}}"}}`
	}
	if len(sr.Users) > 0 {
		tmpl += `
    acl {{$.ServiceName}}UsersAcl http_auth({{$.ServiceName}}Users)
    http-request auth realm {{$.ServiceName}}Realm if !{{$.ServiceName}}UsersAcl`
	} else if len(os.Getenv("USERS")) > 0 {
		tmpl += `
    acl defaultUsersAcl http_auth(defaultUsers)
    http-request auth realm defaultRealm if !defaultUsersAcl`
	}
	tmpl += "{{end}}"
	return tmpl
}

func (m *Reconfigure) getUsersList(sr *proxy.Service) string {
	if len(sr.Users) > 0 {
		return `userlist {{.ServiceName}}Users{{range .Users}}
    user {{.Username}} insecure-password {{.Password}}{{end}}

`
	}
	return ""
}

func (m *Reconfigure) parseTemplate(front, usersList, back string, sr *proxy.Service) (pFront, pBack string) {
	var ctFront bytes.Buffer
	if len(front) > 0 {
		tmplFront, _ := template.New("template").Parse(front)
		tmplFront.Execute(&ctFront, sr)
	}
	tmplUsersList, _ := template.New("template").Parse(usersList)
	tmplBack, _ := template.New("template").Parse(back)
	var ctUsersList bytes.Buffer
	var ctBack bytes.Buffer
	tmplUsersList.Execute(&ctUsersList, sr)
	tmplBack.Execute(&ctBack, sr)
	return ctFront.String(), ctUsersList.String() + ctBack.String()
}

// TODO: Move to registry package
func (m *Reconfigure) getConsulTemplateFromFile(path string) (string, error) {
	content, err := readTemplateFile(path)
	if err != nil {
		return "", fmt.Errorf("Could not read the file %s\n%s", path, err.Error())
	}
	return string(content), nil
}
