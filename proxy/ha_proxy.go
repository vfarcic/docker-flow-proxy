package proxy

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"text/template"
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
	SslBindOptions       string
	SslBindCiphers       string
	Stats                string
	UserList             string
	ExtraGlobal          string
	ExtraDefaults        string
	DefaultBinds         string
	ExtraFrontend        string
	ContentFrontend      string
	ContentFrontendTcp   string
	ContentFrontendSNI   string
}

func NewHaProxy(templatesPath, configsPath string) Proxy {
	data.Services = map[string]Service{}
	return HaProxy{
		TemplatesPath: templatesPath,
		ConfigsPath:   configsPath,
	}
}

func (m HaProxy) GetCertPaths() []string {
	paths := []string{}
	files, _ := ReadDir("/certs")
	for _, file := range files {
		if !file.IsDir() {
			path := fmt.Sprintf("/certs/%s", file.Name())
			paths = append(paths, path)
		}
	}
	files, _ = ReadDir("/run/secrets")
	for _, file := range files {
		if !file.IsDir() {
			lName := strings.ToLower(file.Name())
			if strings.HasPrefix(lName, "cert-") || strings.HasPrefix(lName, "cert_") {
				path := fmt.Sprintf("/run/secrets/%s", file.Name())
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func (m HaProxy) GetCerts() map[string]string {
	certs := map[string]string{}
	paths := m.GetCertPaths()
	for _, path := range paths {
		content, _ := ReadFile(path)
		certs[path] = string(content)
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
	certPaths := m.GetCertPaths()
	certsString := []string{}
	if len(certPaths) > 0 {
		certsString = append(certsString, " ssl")
		for _, certPath := range certPaths {
			certsString = append(certsString, fmt.Sprintf("crt %s", certPath))
		}
	}
	d := ConfigData{
		CertsString: strings.Join(certsString, " "),
	}
	d.ConnectionMode = GetSecretOrEnvVar("CONNECTION_MODE", "http-server-close")
	d.SslBindCiphers = GetSecretOrEnvVar("SSL_BIND_CIPHERS", "ECDH+AESGCM:DH+AESGCM:ECDH+AES256:DH+AES256:ECDH+AES128:DH+AES:RSA+AESGCM:RSA+AES:!aNULL:!MD5:!DSS")
	d.SslBindOptions = GetSecretOrEnvVar("SSL_BIND_OPTIONS", "no-sslv3")
	d.TimeoutConnect = GetSecretOrEnvVar("TIMEOUT_CONNECT", "5")
	d.TimeoutClient = GetSecretOrEnvVar("TIMEOUT_CLIENT", "20")
	d.TimeoutServer = GetSecretOrEnvVar("TIMEOUT_SERVER", "20")
	d.TimeoutQueue = GetSecretOrEnvVar("TIMEOUT_QUEUE", "30")
	d.TimeoutTunnel = GetSecretOrEnvVar("TIMEOUT_TUNNEL", "3600")
	d.TimeoutHttpRequest = GetSecretOrEnvVar("TIMEOUT_HTTP_REQUEST", "5")
	d.TimeoutHttpKeepAlive = GetSecretOrEnvVar("TIMEOUT_HTTP_KEEP_ALIVE", "15")
	statsUser := GetSecretOrEnvVar(os.Getenv("STATS_USER_ENV"), "")
	statsPass := GetSecretOrEnvVar(os.Getenv("STATS_PASS_ENV"), "")
	statsUri := GetSecretOrEnvVar(os.Getenv("STATS_URI_ENV"), "/admin?stats")
	if len(statsUser) > 0 && len(statsPass) > 0 {
		d.Stats = fmt.Sprintf(`
    stats enable
    stats refresh 30s
    stats realm Strictly\ Private
    stats uri %s`,
			statsUri,
		)
		if !strings.EqualFold(statsUser, "none") && !strings.EqualFold(statsPass, "none") {
			d.Stats += fmt.Sprintf(`
    stats auth %s:%s`,
			statsUser,
				statsPass,
			)
		}
	}
	m.getUserList(&d)
	d.ExtraFrontend = GetSecretOrEnvVarSplit("EXTRA_FRONTEND", "")
	if len(d.ExtraFrontend) > 0 {
		d.ExtraFrontend = fmt.Sprintf("    %s", d.ExtraFrontend)
	}
	if strings.EqualFold(GetSecretOrEnvVar("DEBUG", ""), "true") {
		d.ExtraGlobal += `
    log 127.0.0.1:1514 local0`
		d.ExtraFrontend += `
    option httplog
    log global`
		format := GetSecretOrEnvVar("DEBUG_HTTP_FORMAT", "")
		if len(format) > 0 {
			d.ExtraFrontend += fmt.Sprintf(`
    log-format %s`,
				format,
			)
		}
		if strings.EqualFold(GetSecretOrEnvVar("DEBUG_ERRORS_ONLY", ""), "true") {
			d.ExtraDefaults += `
    option  dontlog-normal`
		}
	} else {
		d.ExtraDefaults += `
    option  dontlognull
    option  dontlog-normal`
	}

	defaultPortsString := GetSecretOrEnvVar("DEFAULT_PORTS", "")
	defaultPorts := strings.Split(defaultPortsString, ",")
	for _, bindPort := range defaultPorts {
		formattedPort := strings.Replace(bindPort, ":ssl", d.CertsString, -1)
		d.DefaultBinds += fmt.Sprintf("\n    bind *:%s", formattedPort)
	}
	extraGlobal := GetSecretOrEnvVarSplit("EXTRA_GLOBAL", "")
	if len(extraGlobal) > 0 {
		d.ExtraGlobal += fmt.Sprintf("\n    %s", extraGlobal)
	}
	bindPortsString := GetSecretOrEnvVar("BIND_PORTS", "")
	if len(bindPortsString) > 0 {
		bindPorts := strings.Split(bindPortsString, ",")
		for _, bindPort := range bindPorts {
			d.ExtraFrontend += fmt.Sprintf("\n    bind *:%s", bindPort)
		}
	}
	services := Services{}
	for _, s := range data.Services {
		if len(s.AclName) == 0 {
			s.AclName = s.ServiceName
		}
		services = append(services, s)
		for i, _ := range s.ServiceDest {
			if len(s.ServiceDest[i].ReqMode) == 0 {
				s.ServiceDest[i].ReqMode = "http"
			}
		}
	}
	m.getSni(&services, &d)
	return d
}

func (m *HaProxy) getUserList(data *ConfigData) {
	usersString := GetSecretOrEnvVar("USERS", "")
	encryptedString := GetSecretOrEnvVar("USERS_PASS_ENCRYPTED", "")
	if len(usersString) > 0 {
		data.UserList = "\nuserlist defaultUsers\n"
		encrypted := strings.EqualFold(encryptedString, "true")
		users := ExtractUsersFromString("globalUsers", usersString, encrypted, true)
		// TODO: Test
		if len(users) == 0 {
			users = append(users, RandomUser())
		}
		for _, user := range users {
			passwordType := "insecure-password"
			if user.PassEncrypted {
				passwordType = "password"
			}
			data.UserList = fmt.Sprintf("%s    user %s %s %s\n", data.UserList, user.Username, passwordType, user.Password)
		}
	}
}

func (m *HaProxy) getSni(services *Services, config *ConfigData) {
	sort.Sort(services)
	snimap := make(map[int]string)
	tcpFEs := make(map[int]Services)
	for _, s := range *services {
		if len(s.ServiceDest) == 0 {
			s.ServiceDest = []ServiceDest{ServiceDest{ReqMode: "http"}}
		}
		httpDone := false
		for _, sd := range s.ServiceDest {
			if strings.EqualFold(sd.ReqMode, "http") {
				if !httpDone {
					config.ContentFrontend += m.getFrontTemplate(s)
				}
				httpDone = true
			} else if strings.EqualFold(sd.ReqMode, "sni") {
				_, headerExists := snimap[sd.SrcPort]
				snimap[sd.SrcPort] += m.getFrontTemplateSNI(s, !headerExists)
			} else  {
				tcpService := s
				tcpService.ServiceDest = []ServiceDest{sd}
				tcpFEs[sd.SrcPort] = append(tcpFEs[sd.SrcPort], tcpService)
			}
		}
	}
	for port, tcpServices := range tcpFEs {
		config.ContentFrontendTcp += m.getFrontTemplateTcp(port, tcpServices)
	}

	// Merge the SNI entries into one single string. Sorted by port.
	var sniports []int
	for k := range snimap {
		sniports = append(sniports, k)
	}
	sort.Ints(sniports)
	for _, k := range sniports {
		config.ContentFrontendSNI += snimap[k]
	}
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

func (m *HaProxy) getFrontTemplateTcp(port int, services Services) string {
	sort.Sort(services)
	tmpl := fmt.Sprintf(
		`

frontend tcpFE_%d
    bind *:%d
    mode tcp`,
		port,
		port,
	)
	if strings.EqualFold(GetSecretOrEnvVar("DEBUG", ""), "true") {
		tmpl += `
    option tcplog
    log global`
		format := GetSecretOrEnvVar("DEBUG_TCP_FORMAT", "")
		if len(format) > 0 {
			tmpl += fmt.Sprintf(`
    log-format %s`,
				format,
			)
		}
	}
	for _, s := range services {
		backendPort := port
		if len(s.ServiceDest) > 0 {
			backendPort, _ = strconv.Atoi(s.ServiceDest[0].Port)
		}
		var backend string
		if len(s.ServiceDomain) > 0 {
			backend = fmt.Sprintf(`
    use_backend %s-be%d if domain_%s`,
				s.ServiceName,
				backendPort,
				s.ServiceName,
			)
		} else {
			backend = fmt.Sprintf(
				`
    default_backend %s-be%d`,
				s.ServiceName,
				backendPort,
			)
		}
		aclDomain := m.templateToString(m.getAclDomain(&s), s)
		tmpl += fmt.Sprintf(`%s%s`, aclDomain, backend)
	}
	return tmpl
}

func (m *HaProxy) getFrontTemplate(s Service) string {
	if len(s.PathType) == 0 {
		s.PathType = "path_beg"
	}
	tmplString := fmt.Sprintf(
		`{{range .ServiceDest}}{{if eq .ReqMode "http"}}{{if ne .Port ""}}
    acl url_{{$.AclName}}{{.Port}}{{range .ServicePath}} {{$.PathType}} {{.}}{{end}}{{.SrcPortAcl}}{{end}}{{end}}{{end}}%s`,
		m.getAclDomain(&s),
	)
	if s.HttpsPort > 0 {
		tmplString += `
    acl http_{{.ServiceName}} src_port 80
    acl https_{{.ServiceName}} src_port 443`
	}
	if s.RedirectWhenHttpProto {
		tmplString += `{{range .ServiceDest}}{{if eq .ReqMode "http"}}{{if ne .Port ""}}
    acl is_{{$.AclName}}_http hdr(X-Forwarded-Proto) http
    redirect scheme https if is_{{$.AclName}}_http url_{{$.AclName}}{{.Port}}{{$.AclCondition}}{{.SrcPortAclName}}{{end}}{{end}}{{end}}`
	} else if s.HttpsOnly {
		tmplString += `{{range .ServiceDest}}{{if eq .ReqMode "http"}}{{if ne .Port ""}}
    redirect scheme https if !{ ssl_fc } url_{{$.AclName}}{{.Port}}{{$.AclCondition}}{{.SrcPortAclName}}{{end}}{{end}}{{end}}`
	}
	tmplString += `{{range .ServiceDest}}{{if eq .ReqMode "http"}}{{if ne .Port ""}}
    `
	if s.HttpsPort > 0 {
		tmplString += `use_backend {{$.ServiceName}}-be{{.Port}} if url_{{$.AclName}}{{.Port}}{{$.AclCondition}}{{.SrcPortAclName}} http_{{$.ServiceName}}
    use_backend https-{{$.ServiceName}}-be{{.Port}} if url_{{$.AclName}}{{.Port}}{{$.AclCondition}} https_{{$.ServiceName}}`
	} else {
		tmplString += `use_backend {{$.ServiceName}}-be{{.Port}} if url_{{$.AclName}}{{.Port}}{{$.AclCondition}}{{.SrcPortAclName}}`
	}
	if s.IsDefaultBackend {
		tmplString += fmt.Sprintf(
			`
    default_backend {{$.ServiceName}}-be{{.Port}}`)
	}
	tmplString += "{{end}}{{end}}{{end}}"
	return m.templateToString(tmplString, s)
}

func (m *HaProxy) getAclDomain(s *Service) string {
	if len(s.ServiceDomain) > 0 {
		domFunc := "hdr"
		if s.ServiceDomainMatchAll {
			domFunc = "hdr_dom"
		} else {
			for i, domain := range s.ServiceDomain {
				if strings.HasPrefix(domain, "*") {
					s.ServiceDomain[i] = strings.Trim(domain, "*")
					domFunc = "hdr_end"
				}
			}
		}
		acl := fmt.Sprintf(
			`
    acl domain_{{.AclName}} %s(host) -i{{range .ServiceDomain}} {{.}}{{end}}`,
			domFunc,
		)
		s.AclCondition = fmt.Sprintf(" domain_%s", s.AclName)
		return acl
	}
	return ""
}

func (m *HaProxy) templateToString(templateString string, service Service) string {
	tmpl, _ := template.New("template").Parse(templateString)
	var b bytes.Buffer
	tmpl.Execute(&b, service)
	return b.String()
}
