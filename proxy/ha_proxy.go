package proxy

import (
	"bytes"
	"fmt"
	"html/template"
	"os"
	"os/exec"
	"sort"
	"strings"
)

type HaProxy struct {
	TemplatesPath string
	ConfigsPath   string
	ConfigData    ConfigData
}

// TODO: Change to pointer
var Instance Proxy

// TODO: Move to data from proxy.go when static (e.g. env. vars.)
type ConfigData struct {
	CertsString          string
	ConnectionMode       string
	TimeoutConnect       string
	TimeoutClient        string
	TimeoutServer        string
	TimeoutQueue         string
	TimeoutTunnel        string
	TimeoutHttpRequest   string
	TimeoutHttpKeepAlive string
	StatsUser            string
	StatsPass            string
	UserList             string
	ExtraGlobal          string
	ExtraDefaults        string
	ExtraFrontend        string
	ContentFrontend      string
	ContentFrontendTcp   string
	ContentFrontendSNI   string
}

func NewHaProxy(templatesPath, configsPath string, certs map[string]bool) Proxy {
	data.Certs = certs
	data.Services = map[string]Service{}
	return HaProxy{
		TemplatesPath: templatesPath,
		ConfigsPath:   configsPath,
	}
}

func (m HaProxy) AddCert(certName string) {
	if data.Certs == nil {
		data.Certs = map[string]bool{}
	}
	data.Certs[certName] = true
}

func (m HaProxy) GetCerts() map[string]string {
	certs := map[string]string{}
	for cert := range data.Certs {
		content, _ := ReadFile(fmt.Sprintf("/certs/%s", cert))
		certs[cert] = string(content)
	}
	return certs
}

func (m HaProxy) RunCmd(extraArgs []string) error {
	args := []string{
		"-f",
		"/cfg/haproxy.cfg",
		"-D",
		"-p",
		"/var/run/haproxy.pid",
	}
	args = append(args, extraArgs...)
	cmd := exec.Command("haproxy", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmdRunHa(cmd); err != nil {
		configData, _ := readConfigsFile("/cfg/haproxy.cfg")
		return fmt.Errorf("Command %s\n%s\n%s", strings.Join(cmd.Args, " "), err.Error(), string(configData))
	}
	return nil
}

func (m HaProxy) CreateConfigFromTemplates() error {
	configsContent, err := m.getConfigs()
	if err != nil {
		return err
	}
	configPath := fmt.Sprintf("%s/haproxy.cfg", m.ConfigsPath)
	return writeFile(configPath, []byte(configsContent), 0664)
}

func (m HaProxy) ReadConfig() (string, error) {
	configPath := fmt.Sprintf("%s/haproxy.cfg", m.ConfigsPath)
	out, err := ReadFile(configPath)
	if err != nil {
		return "", err
	}
	return string(out[:]), nil
}

func (m HaProxy) Reload() error {
	logPrintf("Reloading the proxy")
	pidPath := "/var/run/haproxy.pid"
	pid, err := readPidFile(pidPath)
	if err != nil {
		return fmt.Errorf("Could not read the %s file\n%s", pidPath, err.Error())
	}
	cmdArgs := []string{"-sf", string(pid)}
	return HaProxy{}.RunCmd(cmdArgs)
}

func (m HaProxy) AddService(service Service) {
	data.Services[service.ServiceName] = service
}

func (m HaProxy) RemoveService(service string) {
	delete(data.Services, service)
}

func (m HaProxy) getConfigs() (string, error) {
	contentArr := []string{}
	configsFiles := []string{"haproxy.tmpl"}
	configs, err := readConfigsDir(m.TemplatesPath)
	if err != nil {
		return "", fmt.Errorf("Could not read the directory %s\n%s", m.TemplatesPath, err.Error())
	}
	for _, fi := range configs {
		if strings.HasSuffix(fi.Name(), "-fe.cfg") {
			configsFiles = append(configsFiles, fi.Name())
		}
	}
	for _, fi := range configs {
		if strings.HasSuffix(fi.Name(), "-be.cfg") {
			configsFiles = append(configsFiles, fi.Name())
		}
	}
	for _, file := range configsFiles {
		templateBytes, err := readConfigsFile(fmt.Sprintf("%s/%s", m.TemplatesPath, file))
		if err != nil {
			return "", fmt.Errorf("Could not read the file %s\n%s", file, err.Error())
		}
		contentArr = append(contentArr, string(templateBytes))
	}
	if len(configsFiles) == 1 {
		contentArr = append(contentArr, `    acl url_dummy path_beg /dummy
    use_backend dummy-be if url_dummy

backend dummy-be
    server dummy 1.1.1.1:1111 check`)
	}
	tmpl, _ := template.New("contentTemplate").Parse(
		strings.Join(contentArr, "\n\n"),
	)
	var content bytes.Buffer
	tmpl.Execute(&content, m.getConfigData())
	return content.String(), nil
}

func (m HaProxy) getConfigData() ConfigData {
	certs := []string{}
	if len(data.Certs) > 0 {
		certs = append(certs, " ssl")
		for cert := range data.Certs {
			certs = append(certs, fmt.Sprintf("crt /certs/%s", cert))
		}
	}
	d := ConfigData{
		CertsString:          strings.Join(certs, " "),
		TimeoutConnect:       "5",
		TimeoutClient:        "20",
		TimeoutServer:        "20",
		TimeoutQueue:         "30",
		TimeoutTunnel:        "3600",
		TimeoutHttpRequest:   "5",
		TimeoutHttpKeepAlive: "15",
		StatsUser:            "admin",
		StatsPass:            "admin",
		ConnectionMode:       "http-server-close",
	}
	if len(os.Getenv("CONNECTION_MODE")) > 0 {
		d.ConnectionMode = os.Getenv("CONNECTION_MODE")
	}
	if len(os.Getenv("TIMEOUT_CONNECT")) > 0 {
		d.TimeoutConnect = os.Getenv("TIMEOUT_CONNECT")
	}
	if len(os.Getenv("TIMEOUT_CLIENT")) > 0 {
		d.TimeoutClient = os.Getenv("TIMEOUT_CLIENT")
	}
	if len(os.Getenv("TIMEOUT_SERVER")) > 0 {
		d.TimeoutServer = os.Getenv("TIMEOUT_SERVER")
	}
	if len(os.Getenv("TIMEOUT_QUEUE")) > 0 {
		d.TimeoutQueue = os.Getenv("TIMEOUT_QUEUE")
	}
	if len(os.Getenv("TIMEOUT_TUNNEL")) > 0 {
		d.TimeoutTunnel = os.Getenv("TIMEOUT_TUNNEL")
	}
	if len(os.Getenv("TIMEOUT_HTTP_REQUEST")) > 0 {
		d.TimeoutHttpRequest = os.Getenv("TIMEOUT_HTTP_REQUEST")
	}
	if len(os.Getenv("TIMEOUT_HTTP_KEEP_ALIVE")) > 0 {
		d.TimeoutHttpKeepAlive = os.Getenv("TIMEOUT_HTTP_KEEP_ALIVE")
	}
	if len(os.Getenv("STATS_USER")) > 0 {
		d.StatsUser = os.Getenv("STATS_USER")
	}
	if len(os.Getenv("STATS_PASS")) > 0 {
		d.StatsPass = os.Getenv("STATS_PASS")
	}
	if len(os.Getenv("USERS")) > 0 {
		d.UserList = "\nuserlist defaultUsers\n"
		users := strings.Split(os.Getenv("USERS"), ",")
		for _, user := range users {
			userPass := strings.Split(user, ":")
			d.UserList = fmt.Sprintf("%s    user %s insecure-password %s\n", d.UserList, userPass[0], userPass[1])
		}
	}
	if strings.EqualFold(os.Getenv("DEBUG"), "true") {
		d.ExtraGlobal += `
    debug`
	} else {
		d.ExtraDefaults += `
    option  dontlognull
    option  dontlog-normal`
	}
	d.ExtraFrontend = os.Getenv("EXTRA_FRONTEND")
	if len(os.Getenv("BIND_PORTS")) > 0 {
		bindPorts := strings.Split(os.Getenv("BIND_PORTS"), ",")
		for _, bindPort := range bindPorts {
			d.ExtraFrontend += fmt.Sprintf("\n    bind *:%s", bindPort)
		}
	}
	services := Services{}
	// Add the default 80 frontend
	s := Service{
		ReqMode:     "http",
		ServiceName: "service",
		ServiceDest: []ServiceDest{
			{SrcPort: 80},
		},
	}
	services = append(services, s)
	for _, s := range data.Services {
		if len(s.AclName) == 0 {
			s.AclName = s.ServiceName
		}
		for i := 0; i < len(s.ServiceDest); i++ {
			if s.ServiceDest[i].SrcPort == 0 {
				s.ServiceDest[i].SrcPort = 80
			}
		}
		services = append(services, s)
	}
	sort.Sort(services)
	snimap := make(map[int]string)
	httpmap := make(map[int]string)
	for _, s := range services {
		if len(s.ReqMode) == 0 {
			s.ReqMode = "http"
		}
		if strings.EqualFold(s.ReqMode, "http") {
			if len(s.ServiceDest) > 0 {
				localports := make(map[int]bool)
				for _, sd := range s.ServiceDest {
					// Ensure that only one frontend entry exists for each SrcPort
					// This is probably better handled by splitting up getFrontTemplate
					// to render the frontend ... part and remaining lines separately
					_, header_exists := httpmap[sd.SrcPort]
					_, localexists := localports[sd.SrcPort]
					if !localexists {
						tcontent := m.getFrontTemplate(s, d.CertsString, !header_exists)
						httpmap[sd.SrcPort] += tcontent
						localports[sd.SrcPort] = true
					}
				}
			} else {
				_, header_exists := httpmap[80]
				httpmap[80] += m.getFrontTemplate(s, d.CertsString, !header_exists)
			}
		} else if strings.EqualFold(s.ReqMode, "sni") {
			for _, sd := range s.ServiceDest {
				_, header_exists := snimap[sd.SrcPort]
				snimap[sd.SrcPort] += m.getFrontTemplateSNI(s, !header_exists)
			}
		} else {
			d.ContentFrontendTcp += m.getFrontTemplateTcp(s)
		}
	}
	// Merge the http entries into one single string. Sorted by port.
	var httpports []int
	for k := range httpmap {
		httpports = append(httpports, k)
	}
	sort.Ints(httpports)
	d.ContentFrontend = ``
	for _, k := range httpports {
		d.ContentFrontend += httpmap[k]
	}
	// Merge the SNI entries into one single string. Sorted by port.
	var sniports []int
	for k := range snimap {
		sniports = append(sniports, k)
	}
	sort.Ints(sniports)
	d.ContentFrontendSNI = ``
	for _, k := range sniports {
		d.ContentFrontendSNI += snimap[k]
	}
	return d
}

func (m *HaProxy) getFrontTemplateSNI(s Service, gen_header bool) string {
	tmplString := ``
	if gen_header {
		tmplString += `{{range .ServiceDest}}

frontend service_{{.SrcPort}}
    bind *:{{.SrcPort}}
    mode tcp
    tcp-request inspect-delay 5s
    tcp-request content accept if { req_ssl_hello_type 1 }{{end}}`
	}
	tmplString += `{{range .ServiceDest}}
    acl sni_{{$.AclName}}{{.Port}}{{range .ServicePath}} {{$.PathType}} {{.}}{{end}}{{.SrcPortAcl}}{{end}}{{range .ServiceDest}}
    use_backend {{$.ServiceName}}-be{{.Port}} if sni_{{$.AclName}}{{.Port}}{{$.AclCondition}}{{.SrcPortAclName}}{{end}}`
	return m.templateToString(tmplString, s)
}

func (m *HaProxy) getFrontTemplateTcp(s Service) string {
	tmplString := `{{range .ServiceDest}}

frontend {{$.ServiceName}}_{{.SrcPort}}
    bind *:{{.SrcPort}}
    mode tcp
    default_backend {{$.ServiceName}}-be{{.Port}}{{end}}`
	return m.templateToString(tmplString, s)
}

func (m *HaProxy) getFrontTemplate(s Service, certs string, gen_header bool) string {
	if len(s.PathType) == 0 {
		s.PathType = "path_beg"
	}
	tmplString := ``
	if gen_header {
		tmplString += `{{range .ServiceDest}}frontend service_{{.SrcPort}}`
		tmplString += fmt.Sprintf(`
    bind *:{{.SrcPort}}%s
    mode http
{{end}}`, certs)
	}
	tmplString += `{{range .ServiceDest}}{{if $.AclName}}
    acl url_{{$.AclName}}{{.Port}}{{range .ServicePath}} {{$.PathType}} {{.}}{{end}}{{.SrcPortAcl}}{{end}}{{end}}`
	if len(s.ServiceDomain) > 0 {
		domFunc := "hdr_dom"
		for i, domain := range s.ServiceDomain {
			if strings.HasPrefix(domain, "*") {
				s.ServiceDomain[i] = strings.Trim(domain, "*")
				domFunc = "hdr_end"
			}
		}
		tmplString += fmt.Sprintf(
			`
    acl domain_{{.AclName}} %s(host) -i{{range .ServiceDomain}} {{.}}{{end}}`,
			domFunc,
		)
		s.AclCondition = fmt.Sprintf(" domain_%s", s.ServiceName)
	}
	if s.HttpsPort > 0 {
		tmplString += `
    acl http_{{.ServiceName}} src_port 80
    acl https_{{.ServiceName}} src_port 443`
	}
	if s.HttpsOnly {
		tmplString += `{{range .ServiceDest}}{{if $.AclName}}
    redirect scheme https if !{ ssl_fc } url_{{$.AclName}}{{.Port}}{{$.AclCondition}}{{.SrcPortAclName}}{{end}}{{end}}`
	}
	if s.HttpsPort > 0 {
		tmplString += `{{range .ServiceDest}}{{if $.AclName}}
    use_backend {{$.ServiceName}}-be{{.Port}} if url_{{$.AclName}}{{.Port}}{{$.AclCondition}}{{.SrcPortAclName}} http_{{$.ServiceName}}
    use_backend https-{{$.ServiceName}}-be{{.Port}} if url_{{$.AclName}}{{.Port}}{{$.AclCondition}} https_{{$.ServiceName}}{{end}}{{end}}`
	} else {
		tmplString += `{{range .ServiceDest}}{{if $.AclName}}
    use_backend {{$.ServiceName}}-be{{.Port}} if url_{{$.AclName}}{{.Port}}{{$.AclCondition}}{{.SrcPortAclName}}{{end}}{{end}}`
	}
	return m.templateToString(tmplString, s)
}

func (m *HaProxy) templateToString(templateString string, service Service) string {
	tmpl, _ := template.New("template").Parse(templateString)
	var b bytes.Buffer
	tmpl.Execute(&b, service)
	return b.String()
}
