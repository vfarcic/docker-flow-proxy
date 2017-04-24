package actions

import (
	"../proxy"
	"../registry"
	"bytes"
	"fmt"
	"html/template"
	"os"
	"strings"
	"sync"
)

const serviceTemplateFeFilename = "service-formatted-fe.ctmpl"
const serviceTemplateBeFilename = "service-formatted-be.ctmpl"

var mu = &sync.Mutex{}

// Methods that should be created for reconfigure actions
type Reconfigurable interface {
	Execute(reloadAfter bool) error
	GetData() (BaseReconfigure, proxy.Service)
	GetTemplates() (front, back string, err error)
}

// Data structure that holds reconfigure data
type Reconfigure struct {
	BaseReconfigure
	proxy.Service
	Mode string `short:"m" long:"mode" env:"MODE" description:"If set to 'swarm', proxy will operate assuming that Docker service from v1.12+ is used."`
}

// Base structure
type BaseReconfigure struct {
	ConsulAddresses []string
	ConfigsPath     string `short:"c" long:"configs-path" default:"/cfg" description:"The path to the configurations directory"`
	InstanceName    string `long:"proxy-instance-name" env:"PROXY_INSTANCE_NAME" default:"docker-flow" required:"true" description:"The name of the proxy instance."`
	TemplatesPath   string `short:"t" long:"templates-path" default:"/cfg/tmpl" description:"The path to the templates directory"`
}

// TODO: Change proxy.Service to *proxy.Service
// Singleton instance
var ReconfigureInstance Reconfigure

/*
Creates new instance of the Reconfigurable interface
TODO: Change proxy.Service to *proxy.Service
*/
var NewReconfigure = func(baseData BaseReconfigure, serviceData proxy.Service, mode string) Reconfigurable {
	return &Reconfigure{
		BaseReconfigure: baseData,
		Service:         serviceData,
		Mode:            mode,
	}
}

func (m *Reconfigure) Execute(reloadAfter bool) error {
	mu.Lock()
	defer mu.Unlock()
	if isSwarm(m.Mode) && strings.EqualFold(os.Getenv("SKIP_ADDRESS_VALIDATION"), "false") {
		host := m.ServiceName
		if len(m.OutboundHostname) > 0 {
			host = m.OutboundHostname
		}
		if _, err := lookupHost(host); err != nil {
			logPrintf("Could not reach the service %s. Is the service running and connected to the same network as the proxy?", host)
			return err
		}
	}
	if err := m.createConfigs(); err != nil {
		return err
	}
	if !m.hasTemplate() {
		proxy.Instance.AddService(m.Service)
	}
	if reloadAfter {
		reload := Reload{}
		if err := reload.Execute(true); err != nil {
			logPrintf(err.Error())
			return err
		}
		//MW: this happens only when reloadAfter is requested
		//its little ugly because it should not happen when
		//reconfiguration is made from consul config
		//but in that case we never call it with reloadAfter
		//see Fetch.reloadFromRegistry
		if len(m.ConsulAddresses) > 0 || !isSwarm(m.Mode) {
			if err := m.putToConsul(m.ConsulAddresses, m.Service, m.InstanceName); err != nil {
				logPrintf(err.Error())
				return err
			}
		}
	}
	return nil
}

func (m *Reconfigure) GetData() (BaseReconfigure, proxy.Service) {
	return m.BaseReconfigure, m.Service
}

func (m *Reconfigure) createConfigs() error {
	templatesPath := m.TemplatesPath
	sr := &m.Service
	logPrintf("Creating configuration for the service %s", sr.ServiceName)
	feTemplate, beTemplate, err := m.GetTemplates()
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
			FeFile:        serviceTemplateFeFilename,
			FeTemplate:    feTemplate,
			BeFile:        serviceTemplateBeFilename,
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

func (m *Reconfigure) GetTemplates() (front, back string, err error) {
	sr := &m.Service
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
	rmode := sr.ReqMode
	if strings.EqualFold(sr.ReqMode, "sni") {
		rmode = "tcp"
	}
	tmpl := fmt.Sprintf(`{{range .ServiceDest}}
backend %s{{$.ServiceName}}-be{{.Port}}
    mode %s`,
		prefix, rmode,
	)
	if len(sr.ConnectionMode) > 0 {
		tmpl += `
    option {{$.ConnectionMode}}`
	}
	if strings.EqualFold(os.Getenv("DEBUG"), "true") {
		tmpl += `
    log global`
	}
	tmpl += m.getHeaders(sr)
	if len(sr.TimeoutServer) > 0 {
		tmpl += `
    timeout server {{$.TimeoutServer}}s`
	}
	if len(sr.TimeoutTunnel) > 0 {
		tmpl += `
    timeout tunnel {{$.TimeoutTunnel}}s`
	}
	// TODO: Deprecated (dec. 2016).
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
    server {{$.ServiceName}} {{$.Host}}:{{$.HttpsPort}}{{if eq $.SslVerifyNone true}} ssl verify none{{end}}`
		} else {
			tmpl += `
    server {{$.ServiceName}} {{$.Host}}:{{.Port}}{{if eq $.SslVerifyNone true}} ssl verify none{{end}}`
		}
	} else { // It's Consul
		tmpl += `
    {{"{{"}}range $i, $e := service "{{$.FullServiceName}}" "any"{{"}}"}}
    server {{"{{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}}"}}{{if eq $.SkipCheck false}} check{{if eq $.SslVerifyNone true}} ssl verify none{{end}}{{end}}
    {{"{{end}}"}}`
	}
	if len(sr.Users) > 0 {
		tmpl += `
    acl {{$.ServiceName}}UsersAcl http_auth({{$.ServiceName}}Users)
    http-request auth realm {{$.ServiceName}}Realm if !{{$.ServiceName}}UsersAcl
    http-request del-header Authorization`
	} else if len(proxy.GetSecretOrEnvVar("USERS", "")) > 0 {
		tmpl += `
    acl defaultUsersAcl http_auth(defaultUsers)
    http-request auth realm defaultRealm if !defaultUsersAcl
    http-request del-header Authorization`
	}
	tmpl += "{{end}}"
	return tmpl
}

func (m *Reconfigure) getHeaders(sr *proxy.Service) string {
	tmpl := ""
	if sr.XForwardedProto {
		tmpl += `
    http-request add-header X-Forwarded-Proto https if { ssl_fc }`
	}
	for _, header := range sr.AddReqHeader {
		tmpl += fmt.Sprintf(`
    http-request add-header %s`,
			header,
		)
	}
	for _, header := range sr.SetReqHeader {
		tmpl += fmt.Sprintf(`
    http-request set-header %s`,
			header,
		)
	}
	for _, header := range sr.AddResHeader {
		tmpl += fmt.Sprintf(`
    http-response add-header %s`,
			header,
		)
	}
	for _, header := range sr.SetResHeader {
		tmpl += fmt.Sprintf(`
    http-response set-header %s`,
			header,
		)
	}
	for _, header := range sr.DelReqHeader {
		tmpl += fmt.Sprintf(`
    http-request del-header %s`,
			header,
		)
	}
	for _, header := range sr.DelResHeader {
		tmpl += fmt.Sprintf(`
    http-response del-header %s`,
			header,
		)
	}
	return tmpl
}

func (m *Reconfigure) getUsersList(sr *proxy.Service) string {
	if len(sr.Users) > 0 {
		return `userlist {{.ServiceName}}Users{{range .Users}}
    user {{.Username}} {{if .PassEncrypted}}password{{end}}{{if not .PassEncrypted}}insecure-password{{end}} {{.Password}}{{end}}

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

func (m *Reconfigure) hasTemplate() bool {
	return len(m.ConsulTemplateBePath) != 0 ||
		len(m.ConsulTemplateFePath) != 0 ||
		len(m.TemplateBePath) != 0 ||
		len(m.TemplateFePath) != 0
}
