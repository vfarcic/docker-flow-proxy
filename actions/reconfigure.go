package actions

import (
	"../proxy"
	"../registry"
	"bytes"
	"fmt"
	"html/template"
	"os"
	"strconv"
	"strings"
	"sync"
)

const serviceTemplateFeFilename = "service-formatted-fe.ctmpl"
const serviceTemplateBeFilename = "service-formatted-be.ctmpl"

var mu = &sync.Mutex{}

// Reconfigurable defines mandatory interface
type Reconfigurable interface {
	Execute(reloadAfter bool) error
	GetData() (BaseReconfigure, proxy.Service)
	GetTemplates() (front, back string, err error)
}

// Reconfigure structure holds data required to reconfigure the proxy
type Reconfigure struct {
	BaseReconfigure
	proxy.Service
	Mode string `short:"m" long:"mode" env:"MODE" description:"If set to 'swarm', proxy will operate assuming that Docker service from v1.12+ is used."`
}

// BaseReconfigure contains base data required to reconfigure the proxy
type BaseReconfigure struct {
	ConsulAddresses []string
	ConfigsPath     string `short:"c" long:"configs-path" default:"/cfg" description:"The path to the configurations directory"`
	InstanceName    string `long:"proxy-instance-name" env:"PROXY_INSTANCE_NAME" default:"docker-flow" required:"true" description:"The name of the proxy instance."`
	TemplatesPath   string `short:"t" long:"templates-path" default:"/cfg/tmpl" description:"The path to the templates directory"`
}

var reconfigureInstance Reconfigure

// NewReconfigure creates new instance of the Reconfigurable interface
var NewReconfigure = func(baseData BaseReconfigure, serviceData proxy.Service, mode string) Reconfigurable {
	return &Reconfigure{
		BaseReconfigure: baseData,
		Service:         serviceData,
		Mode:            mode,
	}
}

// Execute creates a new configuration and reloads the proxy
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

// GetData returns structure with reconfiguration data and the service
func (m *Reconfigure) GetData() (BaseReconfigure, proxy.Service) {
	return m.BaseReconfigure, m.Service
}

// GetTemplates returns frontend and backend templates
func (m *Reconfigure) GetTemplates() (front, back string, err error) {
	sr := &m.Service
	if value, err := strconv.ParseBool(os.Getenv("CHECK_RESOLVERS")); err == nil {
		sr.CheckResolvers = value
	}
	for i := range sr.ServiceDest {
		if len(sr.ServiceDest[i].ReqMode) == 0 {
			sr.ServiceDest[i].ReqMode = "http"
		}
	}
	if len(sr.ConsulTemplateFePath) > 0 && len(sr.ConsulTemplateBePath) > 0 { // TODO: Deprecated (Consul). Remove it.
		front, err = m.getConsulTemplateFromFile(sr.ConsulTemplateFePath)
		if err != nil {
			return "", "", err
		}
		back, err = m.getConsulTemplateFromFile(sr.ConsulTemplateBePath)
		if err != nil {
			return "", "", err
		}
	} else {
		m.formatData(sr)
		if len(sr.TemplateFePath) > 0 {
			feTmpl, err := readTemplateFile(sr.TemplateFePath)
			if err != nil {
				return "", "", err
			}
			front = m.parseFrontTemplate(string(feTmpl), sr)
		}
		if len(sr.TemplateBePath) > 0 {
			beTmpl, err := readTemplateFile(sr.TemplateBePath)
			if err != nil {
				return "", "", err
			}
			back = m.parseBackTemplate(string(beTmpl), "", sr)
		} else {
			back = m.parseBackTemplate(proxy.GetBackTemplate(sr, m.Mode), m.getUsersList(sr), sr)
		}
	}
	return front, back, nil
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
		ServiceCert:          sr.ServiceCert,
		OutboundHostname:     sr.OutboundHostname,
		PathType:             sr.PathType,
		ConsulTemplateFePath: sr.ConsulTemplateFePath,
		ConsulTemplateBePath: sr.ConsulTemplateBePath,
		Port:                 port,
	}
	if err := registryInstance.PutService(addresses, instanceName, r); err != nil {
		return err
	}
	return nil
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

func (m *Reconfigure) getUsersList(sr *proxy.Service) string {
	if len(sr.Users) > 0 {
		return `userlist {{.ServiceName}}Users{{range .Users}}
    user {{.Username}} {{if .PassEncrypted}}password{{end}}{{if not .PassEncrypted}}insecure-password{{end}} {{.Password}}{{end}}

`
	}
	return ""
}

func (m *Reconfigure) parseFrontTemplate(src string, sr *proxy.Service) string {
	var buf bytes.Buffer
	if len(src) > 0 {
		tmpl, _ := template.New("").Parse(src)
		tmpl.Execute(&buf, sr)
	}
	return buf.String()
}

func (m *Reconfigure) parseBackTemplate(src, usersList string, sr *proxy.Service) string {
	tmplUsersList, _ := template.New("template").Parse(usersList)
	tmpl, _ := template.New("").Parse(src)
	var bufUsersList bytes.Buffer
	var buf bytes.Buffer
	tmplUsersList.Execute(&bufUsersList, sr)
	tmpl.Execute(&buf, sr)
	return bufUsersList.String() + buf.String()
}

// TODO: Deprecated (Consul). Remove it.
func (m *Reconfigure) getConsulTemplateFromFile(path string) (string, error) {
	content, err := readTemplateFile(path)
	if err != nil {
		return "", fmt.Errorf("Could not read the file %s\n%s", path, err.Error())
	}
	return string(content), nil
}

// TODO: Deprecated (Consul). Remove it.
func (m *Reconfigure) hasTemplate() bool {
	return len(m.ConsulTemplateBePath) != 0 ||
		len(m.ConsulTemplateFePath) != 0 ||
		len(m.TemplateBePath) != 0 ||
		len(m.TemplateFePath) != 0
}
