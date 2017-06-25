package proxy

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"text/template"
	"time"
)

type HaProxy struct {
	templatesPath string
	configsPath   string
	configData    ConfigData
}

// TODO: Change to pointer
var Instance Proxy
var reloadPauseMilliseconds time.Duration = 1000

// TODO: Move to data from proxy.go when static (e.g. env. vars.)
type ConfigData struct {
	CertsString          string
	ConnectionMode       string
	ExtraDefaults        string
	ExtraFrontend        string
	ExtraGlobal          string
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
	DefaultBinds         string
	ContentFrontend      string
	ContentFrontendTcp   string
	ContentFrontendSNI   string
}

func NewHaProxy(templatesPath, configsPath string) Proxy {
	data.Services = map[string]Service{}
	return HaProxy{
		templatesPath: templatesPath,
		configsPath:   configsPath,
	}
}

func (m HaProxy) GetCertPaths() []string {
	paths := []string{}
	files, _ := readDir("/certs")
	for _, file := range files {
		if !file.IsDir() {
			path := fmt.Sprintf("/certs/%s", file.Name())
			paths = append(paths, path)
		}
	}
	files, _ = readDir("/run/secrets")
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
	if err := cmdRunHa(args); err != nil {
		configData, _ := readConfigsFile("/cfg/haproxy.cfg")
		return fmt.Errorf(
			"Command %s\n%s\n%s",
			strings.Join(args, " "),
			err.Error(),
			string(configData),
		)
	}
	return nil
}

func (m HaProxy) CreateConfigFromTemplates() error {
	configsContent, err := m.getConfigs()
	if err != nil {
		return err
	}
	configPath := fmt.Sprintf("%s/haproxy.cfg", m.configsPath)
	return writeFile(configPath, []byte(configsContent), 0664)
}

func (m HaProxy) ReadConfig() (string, error) {
	configPath := fmt.Sprintf("%s/haproxy.cfg", m.configsPath)
	out, err := ReadFile(configPath)
	if err != nil {
		return "", err
	}
	return string(out[:]), nil
}

func (m HaProxy) Reload() error {
	logPrintf("Reloading the proxy")
	var reloadErr error
	for i := 0; i < 10; i++ {
		pidPath := "/var/run/haproxy.pid"
		pid, err := readPidFile(pidPath)
		if err != nil {
			return fmt.Errorf("Could not read the %s file\n%s", pidPath, err.Error())
		}
		reloadStrategy := m.getReloadStrategy()
		cmdArgs := []string{reloadStrategy, string(pid)}
		reloadErr = HaProxy{}.RunCmd(cmdArgs)
		if reloadErr == nil {
			logPrintf("Proxy config was reloaded")
			break
		}
		time.Sleep(time.Millisecond * reloadPauseMilliseconds)
	}
	return reloadErr
}

func (m HaProxy) AddService(service Service) {
	data.Services[service.ServiceName] = service
}

func (m HaProxy) RemoveService(service string) {
	delete(data.Services, service)
}

func (m HaProxy) getConfigs() (string, error) {
	contentArr := []string{}
	tmplPath := "haproxy.tmpl"
	if len(os.Getenv("CFG_TEMPLATE_PATH")) > 0 {
		tmplPath = os.Getenv("CFG_TEMPLATE_PATH")
	}
	configsFiles := []string{tmplPath}
	configs, err := readConfigsDir(m.templatesPath)
	if err != nil {
		return "", fmt.Errorf("Could not read the directory %s\n%s", m.templatesPath, err.Error())
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
		path := fmt.Sprintf("%s/%s", m.templatesPath, file)
		if strings.HasPrefix(file, "/") {
			path = file
		}
		templateBytes, err := readConfigsFile(path)
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
	d := ConfigData{
		CertsString: strings.Join(m.getCerts(), " "),
	}
	d.ConnectionMode = getSecretOrEnvVar("CONNECTION_MODE", "http-server-close")
	d.SslBindCiphers = getSecretOrEnvVar("SSL_BIND_CIPHERS", "ECDH+AESGCM:DH+AESGCM:ECDH+AES256:DH+AES256:ECDH+AES128:DH+AES:RSA+AESGCM:RSA+AES:!aNULL:!MD5:!DSS")
	d.SslBindOptions = getSecretOrEnvVar("SSL_BIND_OPTIONS", "no-sslv3")
	d.TimeoutConnect = getSecretOrEnvVar("TIMEOUT_CONNECT", "5")
	d.TimeoutClient = getSecretOrEnvVar("TIMEOUT_CLIENT", "20")
	d.TimeoutServer = getSecretOrEnvVar("TIMEOUT_SERVER", "20")
	d.TimeoutQueue = getSecretOrEnvVar("TIMEOUT_QUEUE", "30")
	d.TimeoutTunnel = getSecretOrEnvVar("TIMEOUT_TUNNEL", "3600")
	d.TimeoutHttpRequest = getSecretOrEnvVar("TIMEOUT_HTTP_REQUEST", "5")
	d.TimeoutHttpKeepAlive = getSecretOrEnvVar("TIMEOUT_HTTP_KEEP_ALIVE", "15")
	m.putStats(&d)
	m.getUserList(&d)
	d.ExtraFrontend = getSecretOrEnvVarSplit("EXTRA_FRONTEND", "")
	if len(d.ExtraFrontend) > 0 {
		d.ExtraFrontend = fmt.Sprintf("    %s", d.ExtraFrontend)
	}
	if value, err := strconv.ParseBool(os.Getenv("CHECK_RESOLVERS")); err == nil && value {
		d.ExtraDefaults += `
    default-server init-addr last,libc,none`
	}
	m.addCompression(&d)
	m.addDebug(&d)

	defaultPortsString := getSecretOrEnvVar("DEFAULT_PORTS", "")
	defaultPorts := strings.Split(defaultPortsString, ",")
	for _, bindPort := range defaultPorts {
		formattedPort := strings.Replace(bindPort, ":ssl", d.CertsString, -1)
		d.DefaultBinds += fmt.Sprintf("\n    bind *:%s", formattedPort)
	}
	extraGlobal := getSecretOrEnvVarSplit("EXTRA_GLOBAL", "")
	if len(extraGlobal) > 0 {
		d.ExtraGlobal += fmt.Sprintf("\n    %s", extraGlobal)
	}
	bindPortsString := getSecretOrEnvVar("BIND_PORTS", "")
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
		for i := range s.ServiceDest {
			if len(s.ServiceDest[i].ReqMode) == 0 {
				s.ServiceDest[i].ReqMode = "http"
			}
		}
	}
	m.getSni(&services, &d)
	return d
}

func (m *HaProxy) getCerts() []string {
	certPaths := m.GetCertPaths()
	certs := []string{}
	if len(certPaths) > 0 {
		certs = append(certs, " ssl crt-list /cfg/crt-list.txt")
		mu.Lock()
		defer mu.Unlock()
		writeFile("/cfg/crt-list.txt", []byte(strings.Join(certPaths, "\n")), 0664)
	}
	if len(os.Getenv("CA_FILE")) > 0 {
		if len(certs) == 0 {
			certs = append(certs, " ssl")
		}
		cf := "ca-file " + os.Getenv("CA_FILE") + " verify optional"
		certs = append(certs, cf)
	}
	return certs
}

func (m *HaProxy) addCompression(data *ConfigData) {
	if len(os.Getenv("COMPRESSION_ALGO")) > 0 {
		data.ExtraDefaults += fmt.Sprintf(`
    compression algo %s`,
			os.Getenv("COMPRESSION_ALGO"),
		)
		if len(os.Getenv("COMPRESSION_TYPE")) > 0 {
			data.ExtraDefaults += fmt.Sprintf(`
    compression type %s`,
				os.Getenv("COMPRESSION_TYPE"),
			)
		}
	}
}

func (m *HaProxy) addDebug(data *ConfigData) {
	if strings.EqualFold(getSecretOrEnvVar("DEBUG", ""), "true") {
		data.ExtraGlobal += `
    log 127.0.0.1:1514 local0`
		data.ExtraFrontend += `
    option httplog
    log global`
		format := getSecretOrEnvVar("DEBUG_HTTP_FORMAT", "")
		if len(format) > 0 {
			data.ExtraFrontend += fmt.Sprintf(`
    log-format %s`,
				format,
			)
		}
		if strings.EqualFold(getSecretOrEnvVar("DEBUG_ERRORS_ONLY", ""), "true") {
			data.ExtraDefaults += `
    option  dontlog-normal`
		}
	} else {
		data.ExtraDefaults += `
    option  dontlognull
    option  dontlog-normal`
	}
}

func (m *HaProxy) putStats(data *ConfigData) {
	statsUser := getSecretOrEnvVar(os.Getenv("STATS_USER_ENV"), "")
	statsPass := getSecretOrEnvVar(os.Getenv("STATS_PASS_ENV"), "")
	statsUri := getSecretOrEnvVar(os.Getenv("STATS_URI_ENV"), "/admin?stats")
	if len(statsUser) > 0 && len(statsPass) > 0 {
		data.Stats = fmt.Sprintf(`
    stats enable
    stats refresh 30s
    stats realm Strictly\ Private
    stats uri %s`,
			statsUri,
		)
		if !strings.EqualFold(statsUser, "none") && !strings.EqualFold(statsPass, "none") {
			data.Stats += fmt.Sprintf(`
    stats auth %s:%s`,
				statsUser,
				statsPass,
			)
		}
	}
}

func (m *HaProxy) getUserList(data *ConfigData) {
	usersString := getSecretOrEnvVar("USERS", "")
	encryptedString := getSecretOrEnvVar("USERS_PASS_ENCRYPTED", "")
	if len(usersString) > 0 {
		data.UserList = "\nuserlist defaultUsers\n"
		encrypted := strings.EqualFold(encryptedString, "true")
		users := extractUsersFromString("globalUsers", usersString, encrypted, true)
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
			s.ServiceDest = []ServiceDest{{ReqMode: "http"}}
		}
		httpDone := false
		putDomainFunction(&s)
		for i, sd := range s.ServiceDest {
			if strings.EqualFold(sd.ReqMode, "http") {
				if !httpDone {
					config.ContentFrontend += getFrontTemplate(s)
				}
				httpDone = true
			} else if strings.EqualFold(sd.ReqMode, "sni") {
				_, headerExists := snimap[sd.SrcPort]
				snimap[sd.SrcPort] += m.getFrontTemplateSNI(s, i, !headerExists)
			} else {
				tcpService := s
				tcpService.ServiceDest = []ServiceDest{sd}
				tcpService.AclCondition = fmt.Sprintf(" domain_%s", s.AclName)
				if strings.EqualFold(os.Getenv("DEBUG"), "true") {
					tcpService.Debug = true
					tcpService.DebugFormat = getSecretOrEnvVar("DEBUG_TCP_FORMAT", "")
				}
				tcpFEs[sd.SrcPort] = append(tcpFEs[sd.SrcPort], tcpService)
			}
		}
	}
	config.ContentFrontendTcp += getFrontTemplateTcp(tcpFEs)

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

// TODO: Refactor into template
func (m *HaProxy) getFrontTemplateSNI(s Service, si int, gen_header bool) string {
	tmplString := ``
	if gen_header {
		tmplString += fmt.Sprintf(`{{$sd1 := index $.ServiceDest %d}}

frontend service_{{$sd1.SrcPort}}
    bind *:{{$sd1.SrcPort}}
    mode tcp
    tcp-request inspect-delay 5s
    tcp-request content accept if { req_ssl_hello_type 1 }`, si)
	}
	tmplString += fmt.Sprintf(`{{$sd := index $.ServiceDest %d}}
    acl sni_{{.AclName}}{{$sd.Port}}-%d{{range $sd.ServicePath}} {{$.PathType}} {{.}}{{end}}{{$sd.SrcPortAcl}}
    use_backend {{$.ServiceName}}-be{{$sd.Port}} if sni_{{$.AclName}}{{$sd.Port}}-%d{{$.AclCondition}}{{$sd.SrcPortAclName}}`, si, si+1, si+1)
	return templateToString(tmplString, s)
}

func (m *HaProxy) getReloadStrategy() string {
	reloadStrategy := "-sf"
	terminateOnReload := strings.EqualFold(os.Getenv("TERMINATE_ON_RELOAD"), "true")
	if terminateOnReload {
		reloadStrategy = "-st"
	}
	return reloadStrategy
}
