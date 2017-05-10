// +build !integration

package proxy

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"os"
	"strings"
	"testing"
	"time"
)

// Setup

type HaProxyTestSuite struct {
	suite.Suite
	TemplatesPath   string
	ConfigsPath     string
	Pid             string
	TemplateContent string
	ServicesContent string
}

// Suite

func TestHaProxyUnitTestSuite(t *testing.T) {
	logPrintf = func(format string, v ...interface{}) {}
	s := new(HaProxyTestSuite)
	s.TemplateContent = `global
    pidfile /var/run/haproxy.pid
    tune.ssl.default-dh-param 2048

    # disable sslv3, prefer modern ciphers
    ssl-default-bind-options no-sslv3
    ssl-default-bind-ciphers ECDH+AESGCM:DH+AESGCM:ECDH+AES256:DH+AES256:ECDH+AES128:DH+AES:RSA+AESGCM:RSA+AES:!aNULL:!MD5:!DSS

    ssl-default-server-options no-sslv3
    ssl-default-server-ciphers ECDH+AESGCM:DH+AESGCM:ECDH+AES256:DH+AES256:ECDH+AES128:DH+AES:RSA+AESGCM:RSA+AES:!aNULL:!MD5:!DSS

defaults
    mode    http
    balance roundrobin

    option  dontlognull
    option  dontlog-normal
    option  http-server-close
    option  forwardfor
    option  redispatch

    errorfile 400 /errorfiles/400.http
    errorfile 403 /errorfiles/403.http
    errorfile 405 /errorfiles/405.http
    errorfile 408 /errorfiles/408.http
    errorfile 429 /errorfiles/429.http
    errorfile 500 /errorfiles/500.http
    errorfile 502 /errorfiles/502.http
    errorfile 503 /errorfiles/503.http
    errorfile 504 /errorfiles/504.http

    maxconn 5000
    timeout connect 5s
    timeout client  20s
    timeout server  20s
    timeout queue   30s
    timeout tunnel  3600s
    timeout http-request 5s
    timeout http-keep-alive 15s


frontend services
    bind *:80
    bind *:443
    mode http
`
	s.ServicesContent = `

config1 fe content

config2 fe content

config1 be content

config2 be content`
	os.Setenv("STATS_USER_ENV", "STATS_USER")
	os.Setenv("STATS_PASS_ENV", "STATS_PASS")
	os.Setenv("STATS_URI_ENV", "STATS_URI")
	reloadPauseMillisecondsOrig := reloadPauseMilliseconds
	defer func() { reloadPauseMilliseconds = reloadPauseMillisecondsOrig }()
	reloadPauseMilliseconds = 1
	suite.Run(t, s)
}

func (s *HaProxyTestSuite) SetupTest() {
	s.Pid = "123"
	s.TemplatesPath = "test_configs/tmpl"
	s.ConfigsPath = "test_configs"
	os.Setenv("DEFAULT_PORTS", "80,443:ssl")
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		return nil
	}
	readPidFile = func(fileName string) ([]byte, error) {
		return []byte(s.Pid), nil
	}
}

// GetCertPaths

func (s HaProxyTestSuite) Test_GetCertPaths_ReturnsCerts() {
	readDirOrig := ReadDir
	defer func() {
		ReadDir = readDirOrig
	}()
	p := HaProxy{}
	expected := []string{}
	mockedFiles := []os.FileInfo{}
	for i := 1; i <= 3; i++ {
		certName := fmt.Sprintf("my-cert-%d", i)
		path := fmt.Sprintf("/certs/%s", certName)
		expected = append(expected, path)
		file := FileInfoMock{
			NameMock: func() string {
				return certName
			},
			IsDirMock: func() bool {
				return false
			},
		}
		mockedFiles = append(mockedFiles, file)
	}
	ReadDir = func(dir string) ([]os.FileInfo, error) {
		if dir == "/certs" {
			return mockedFiles, nil
		}
		return []os.FileInfo{}, nil
	}
	dir := FileInfoMock{
		IsDirMock: func() bool {
			return true
		},
	}
	mockedFiles = append(mockedFiles, dir)

	actual := p.GetCertPaths()

	s.EqualValues(expected, actual)
}

func (s HaProxyTestSuite) Test_GetCertPaths_ReturnsSecrets() {
	readDirOrig := ReadDir
	defer func() {
		ReadDir = readDirOrig
	}()
	p := HaProxy{}
	expected := []string{}
	mockedFiles := []os.FileInfo{}
	expected = append(expected, "/run/secrets/cert-anything")
	mockedFiles = append(mockedFiles, FileInfoMock{
		NameMock: func() string {
			return "cert-anything"
		},
		IsDirMock: func() bool {
			return false
		},
	})
	expected = append(expected, "/run/secrets/cert_anything")
	mockedFiles = append(mockedFiles, FileInfoMock{
		NameMock: func() string {
			return "cert_anything"
		},
		IsDirMock: func() bool {
			return false
		},
	})
	mockedFiles = append(mockedFiles, FileInfoMock{
		NameMock: func() string {
			return "not-a-cert"
		},
		IsDirMock: func() bool {
			return false
		},
	})

	ReadDir = func(dir string) ([]os.FileInfo, error) {
		if dir == "/run/secrets" {
			return mockedFiles, nil
		}
		return []os.FileInfo{}, nil
	}

	actual := p.GetCertPaths()

	s.EqualValues(expected, actual)
}

// GetCerts

func (s HaProxyTestSuite) Test_GetCerts_ReturnsAllCerts() {
	readDirOrig := ReadDir
	readFileOrig := ReadFile
	defer func() {
		ReadDir = readDirOrig
		ReadFile = readFileOrig
	}()
	p := HaProxy{}
	expected := map[string]string{}
	mockedFiles := []os.FileInfo{}
	for i := 1; i <= 3; i++ {
		certName := fmt.Sprintf("my-cert-%d", i)
		path := fmt.Sprintf("/certs/%s", certName)
		expected[path] = fmt.Sprintf("content of the certificate %s", path)
		file := FileInfoMock{
			NameMock: func() string {
				return certName
			},
			IsDirMock: func() bool {
				return false
			},
		}
		mockedFiles = append(mockedFiles, file)
	}
	ReadDir = func(dir string) ([]os.FileInfo, error) {
		if dir == "/certs" {
			return mockedFiles, nil
		}
		return []os.FileInfo{}, nil
	}
	ReadFile = func(filename string) ([]byte, error) {
		content := fmt.Sprintf("content of the certificate %s", filename)
		return []byte(content), nil
	}
	dir := FileInfoMock{
		IsDirMock: func() bool {
			return true
		},
	}
	mockedFiles = append(mockedFiles, dir)

	actual := p.GetCerts()

	s.EqualValues(expected, actual)
}

func (s HaProxyTestSuite) Test_GetCerts_ReturnsEmptyMap_WhenReadDirFails() {
	readDirOrig := ReadDir
	defer func() {
		ReadDir = readDirOrig
	}()
	p := HaProxy{}
	ReadDir = func(dir string) ([]os.FileInfo, error) {
		return nil, fmt.Errorf("This is an error")
	}

	actual := p.GetCerts()

	s.Empty(actual)
}

// CreateConfigFromTemplates

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_ReturnsError_WhenReadDirFails() {
	readConfigsDirOrig := readConfigsDir
	defer func() {
		readConfigsDir = readConfigsDirOrig
	}()
	readConfigsDir = func(dirname string) ([]os.FileInfo, error) {
		return nil, fmt.Errorf("Could not read the directory")
	}

	err := NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Error(err)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_WritesCfgContentsIntoFile() {
	var actualData string
	expectedData := fmt.Sprintf(
		"%s%s",
		s.TemplateContent,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsLogging_WhenDebug() {
	debugOrig := os.Getenv("DEBUG")
	defer func() { os.Setenv("DEBUG", debugOrig) }()
	os.Setenv("DEBUG", "true")
	var actualData string
	expectedData := fmt.Sprintf(
		"%s%s",
		s.getTemplateWithLogs(),
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsDefaultServer_WhenCheckResolversIsSetToTrue() {
	checkResolversOrig := os.Getenv("CHECK_RESOLVERS")
	defer func() { os.Setenv("CHECK_RESOLVERS", checkResolversOrig) }()
	os.Setenv("CHECK_RESOLVERS", "true")
	var actualData string
	tmpl := strings.Replace(
		s.TemplateContent,
		"balance roundrobin\n",
		"balance roundrobin\n\n    default-server init-addr last,libc,none",
		-1,
	)
	expectedData := fmt.Sprintf(
		"%s%s",
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsCompressionAlgo_WhenSet() {
	compressionAlgoOrig := os.Getenv("COMPRESSION_ALGO")
	defer func() { os.Setenv("COMPRESSION_ALGO", compressionAlgoOrig) }()
	os.Setenv("COMPRESSION_ALGO", "gzip")
	var actualData string
	tmpl := strings.Replace(
		s.TemplateContent,
		"balance roundrobin\n",
		"balance roundrobin\n\n    compression algo gzip",
		-1,
	)
	expectedData := fmt.Sprintf(
		"%s%s",
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsCompressionType_WhenCompressionAlgoAndTypeAreSet() {
	compressionAlgoOrig := os.Getenv("COMPRESSION_ALGO")
	compressionTypeOrig := os.Getenv("COMPRESSION_TYPE")
	defer func() {
		os.Setenv("COMPRESSION_ALGO", compressionAlgoOrig)
		os.Setenv("COMPRESSION_TYPE", compressionTypeOrig)
	}()
	os.Setenv("COMPRESSION_ALGO", "gzip")
	os.Setenv("COMPRESSION_TYPE", "text/css text/html text/javascript application/javascript text/plain text/xml application/json")
	var actualData string
	tmpl := strings.Replace(
		s.TemplateContent,
		"balance roundrobin\n",
		`balance roundrobin

    compression algo gzip
    compression type text/css text/html text/javascript application/javascript text/plain text/xml application/json`,
		-1,
	)
	expectedData := fmt.Sprintf(
		"%s%s",
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsStats_WhenStatsUserAndPassArePresent() {
	var actualData string
	statUserOrig := os.Getenv("STATS_USER")
	statPassOrig := os.Getenv("STATS_PASS")
	statUserEnvOrig := os.Getenv("STATS_USER_ENV")
	statPassEnvOrig := os.Getenv("STATS_PASS_ENV")
	defer func() {
		os.Setenv("STATS_USER", statUserOrig)
		os.Setenv("STATS_PASS", statPassOrig)
		os.Setenv("STATS_USER_ENV", statUserEnvOrig)
		os.Setenv("STATS_PASS_ENV", statPassEnvOrig)
	}()
	os.Setenv("STATS_USER", "my-user")
	os.Setenv("STATS_PASS", "my-pass")
	os.Setenv("STATS_USER_ENV", "STATS_USER")
	os.Setenv("STATS_PASS_ENV", "STATS_PASS")
	statsAuth := `    stats enable
    stats refresh 30s
    stats realm Strictly\ Private
    stats uri /admin?stats
    stats auth my-user:my-pass

frontend services`
	expectedData := fmt.Sprintf(
		"%s%s",
		strings.Replace(s.TemplateContent, "\nfrontend services", statsAuth, -1),
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_RemovesStatsAuth_WhenUserIsNone() {
	var actualData string
	statUserOrig := os.Getenv("STATS_USER")
	statPassOrig := os.Getenv("STATS_PASS")
	statUserEnvOrig := os.Getenv("STATS_USER_ENV")
	statPassEnvOrig := os.Getenv("STATS_PASS_ENV")
	defer func() {
		os.Setenv("STATS_USER", statUserOrig)
		os.Setenv("STATS_PASS", statPassOrig)
		os.Setenv("STATS_USER_ENV", statUserEnvOrig)
		os.Setenv("STATS_PASS_ENV", statPassEnvOrig)
	}()
	os.Setenv("STATS_USER", "none")
	os.Setenv("STATS_PASS", "none")
	os.Setenv("STATS_USER_ENV", "STATS_USER")
	os.Setenv("STATS_PASS_ENV", "STATS_PASS")
	statsAuth := `    stats enable
    stats refresh 30s
    stats realm Strictly\ Private
    stats uri /admin?stats

frontend services`
	expectedData := fmt.Sprintf(
		"%s%s",
		strings.Replace(s.TemplateContent, "\nfrontend services", statsAuth, -1),
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsHttpLogFormat_WhenSpecified() {
	debugOrig := os.Getenv("DEBUG")
	debugHttpFormatOrig := os.Getenv("DEBUG_HTTP_FORMAT")
	defer func() {
		os.Setenv("DEBUG", debugOrig)
		os.Setenv("DEBUG_HTTP_FORMAT", debugHttpFormatOrig)
	}()
	os.Setenv("DEBUG", "true")
	os.Setenv("DEBUG_HTTP_FORMAT", "something")
	var actualData string
	expectedData := fmt.Sprintf(
		`%s
    log-format something%s`,
		s.getTemplateWithLogs(),
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsDoNotLogNormal_WhenDebugErrorsOnlyIsSet() {
	debugOrig := os.Getenv("DEBUG")
	debugErrorsOnlyOrig := os.Getenv("DEBUG")
	defer func() {
		os.Setenv("DEBUG", debugOrig)
		os.Setenv("DEBUG_ERRORS_ONLY", debugErrorsOnlyOrig)
	}()
	os.Setenv("DEBUG", "true")
	os.Setenv("DEBUG_ERRORS_ONLY", "true")
	var actualData string
	expectedData := fmt.Sprintf(
		"%s%s",
		s.getTemplateWithLogsAndErrorsOnly(),
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsExtraGlobal() {
	globalOrig := os.Getenv("EXTRA_GLOBAL")
	defer func() { os.Setenv("EXTRA_GLOBAL", globalOrig) }()
	os.Setenv("EXTRA_GLOBAL", "this is extra content,this is a new line")
	var actualData string
	tmpl := strings.Replace(
		s.TemplateContent,
		"tune.ssl.default-dh-param 2048",
		"tune.ssl.default-dh-param 2048\n    this is extra content\n    this is a new line",
		-1,
	)
	expectedData := fmt.Sprintf(
		"%s%s",
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsExtraFrontEnd() {
	extraFrontendOrig := os.Getenv("EXTRA_FRONTEND")
	defer func() { os.Setenv("EXTRA_FRONTEND", extraFrontendOrig) }()
	os.Setenv("EXTRA_FRONTEND", "this is an extra content,and a new line,and another")
	var actualData string
	expectedData := fmt.Sprintf(
		`%s    this is an extra content
    and a new line
    and another%s`,
		s.TemplateContent,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEnd() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_my-service-11111 path_beg /path-1 path_beg /path-2 port1111Acl
    acl url_my-service-12222 path_beg /path-3 port2222Acl
    use_backend my-service-1-be1111 if url_my-service-11111 my-src-port
    use_backend my-service-1-be2222 if url_my-service-12222
    acl url_my-service-23333 path_beg /path-4 port3333Acl
    use_backend my-service-2-be3333 if url_my-service-23333%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service-1"] = Service{
		ServiceName: "my-service-1",
		PathType:    "path_beg",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path-1", "/path-2"}, SrcPortAcl: " port1111Acl", SrcPortAclName: " my-src-port"},
			{Port: "2222", ServicePath: []string{"/path-3"}, SrcPortAcl: " port2222Acl"},
		},
	}
	data.Services["my-service-2"] = Service{
		ServiceName: "my-service-2",
		PathType:    "path_beg",
		ServiceDest: []ServiceDest{
			{Port: "3333", ServicePath: []string{"/path-4"}, SrcPortAcl: " port3333Acl"},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsSortedContentFrontEnd() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_acl11111 path_beg /path
    use_backend my-second-service-be1111 if url_acl11111
    acl url_acl21111 path_beg /path
    use_backend my-first-service-be1111 if url_acl21111
    acl url_the-last-service1111 path_beg /path
    use_backend the-last-service-be1111 if url_the-last-service1111%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	// Will be listed second because of AclName
	data.Services["my-first-service"] = Service{
		ServiceName: "my-first-service",
		AclName:     "acl2",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}
	// Will be listed last because of ServiceName (there is no AclName)
	data.Services["the-last-service"] = Service{
		ServiceName: "the-last-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}
	// Will be listed first because of AclName
	data.Services["my-second-service"] = Service{
		ServiceName: "my-second-service",
		AclName:     "acl1",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_PutsServicesWithRootPathToTheEnd() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_01-first-service1111 path_beg /path
    use_backend 01-first-service-be1111 if url_01-first-service1111
    acl url_03-third-service1111 path_beg /path
    use_backend 03-third-service-be1111 if url_03-third-service1111
    acl url_02-another-root-service1111 path_beg /
    use_backend 02-another-root-service-be1111 if url_02-another-root-service1111
    acl url_02-root-service1111 path_beg /
    use_backend 02-root-service-be1111 if url_02-root-service1111%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	// Will be listed first
	data.Services["01-first-service"] = Service{
		ServiceName: "01-first-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}
	// Will be listed last bacause of the root path and service name
	data.Services["02-root-service"] = Service{
		ServiceName: "02-root-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/"}},
		},
	}
	// Will be listed third because of the root path and service name
	data.Services["02-another-root-service"] = Service{
		ServiceName: "02-another-root-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/"}},
		},
	}
	// Will be listed second
	data.Services["03-third-service"] = Service{
		ServiceName: "03-third-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_PutsServicesWellKnownPathToTheBeginning() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_02-another-well-known-service1111 path_beg /.well-known/and/somrthing/else
    use_backend 02-another-well-known-service-be1111 if url_02-another-well-known-service1111
    acl url_02-well-known-service1111 path_beg /.well-known
    use_backend 02-well-known-service-be1111 if url_02-well-known-service1111
    acl url_01-first-service1111 path_beg /path
    use_backend 01-first-service-be1111 if url_01-first-service1111
    acl url_03-third-service1111 path_beg /path
    use_backend 03-third-service-be1111 if url_03-third-service1111%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	// Will be listed third
	data.Services["01-first-service"] = Service{
		ServiceName: "01-first-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}
	// Will be listed second bacause of the well-known path and service name
	data.Services["02-well-known-service"] = Service{
		ServiceName: "02-well-known-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/.well-known"}},
		},
	}
	// Will be listed first because of the well-known path and service name
	data.Services["02-another-well-known-service"] = Service{
		ServiceName: "02-another-well-known-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/.well-known/and/somrthing/else"}},
		},
	}
	// Will be listed last
	data.Services["03-third-service"] = Service{
		ServiceName: "03-third-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndTcp() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s

frontend tcpFE_1234
    bind *:1234
    mode tcp
    default_backend my-service-1-be4321%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service-1"] = Service{
		ServiceName: "my-service-1",
		ServiceDest: []ServiceDest{
			{SrcPort: 1234, Port: "4321", ReqMode: "tcp"},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsMultipleFrontends() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_my-service-12222 path_beg /path
    use_backend my-service-1-be2222 if url_my-service-12222

frontend tcpFE_3333
    bind *:3333
    mode tcp
    default_backend my-service-1-be4444%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service-1"] = Service{
		ServiceName: "my-service-1",
		ServiceDest: []ServiceDest{
			{SrcPort: 1111, Port: "2222", ReqMode: "http", ServicePath: []string{"/path"}},
			{SrcPort: 3333, Port: "4444", ReqMode: "tcp"},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_GroupsTcpFrontendsByPort() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s

frontend tcpFE_1234
    bind *:1234
    mode tcp
    acl domain_my-service-1 hdr(host) -i my-domain.com
    use_backend my-service-1-be4321 if domain_my-service-1
    acl domain_my-service-2 hdr(host) -i my-domain-1.com my-domain-2.com
    use_backend my-service-2-be4321 if domain_my-service-2%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service-1"] = Service{
		ServiceName: "my-service-1",
		ServiceDest: []ServiceDest{
			{SrcPort: 1234, Port: "4321", ReqMode: "tcp"},
		},
		ServiceDomain: []string{"my-domain.com"},
	}
	data.Services["my-service-2"] = Service{
		ServiceName: "my-service-2",
		ServiceDest: []ServiceDest{
			{SrcPort: 1234, Port: "4321", ReqMode: "tcp"},
		},
		ServiceDomain: []string{"my-domain-1.com", "my-domain-2.com"},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsLoggingToTcpFrontends() {
	debugOrig := os.Getenv("DEBUG")
	defer func() { os.Setenv("DEBUG", debugOrig) }()
	os.Setenv("DEBUG", "true")
	var actualData string
	expectedData := fmt.Sprintf(
		`%s

frontend tcpFE_1234
    bind *:1234
    mode tcp
    option tcplog
    log global
    default_backend my-service-1-be4321%s`,
		s.getTemplateWithLogs(),
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service-1"] = Service{
		ServiceName: "my-service-1",
		ServiceDest: []ServiceDest{
			{SrcPort: 1234, Port: "4321", ReqMode: "tcp"},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsTcpLoggingFormat() {
	debugOrig := os.Getenv("DEBUG")
	debugTcpFormatOrig := os.Getenv("DEBUG_TCP_FORMAT")
	defer func() {
		os.Setenv("DEBUG", debugOrig)
		os.Setenv("DEBUG_TCP_FORMAT", debugTcpFormatOrig)
	}()
	os.Setenv("DEBUG", "true")
	os.Setenv("DEBUG_TCP_FORMAT", "something-tcp-related")
	var actualData string
	expectedData := fmt.Sprintf(
		`%s

frontend tcpFE_1234
    bind *:1234
    mode tcp
    option tcplog
    log global
    log-format something-tcp-related
    default_backend my-service-1-be4321%s`,
		s.getTemplateWithLogs(),
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service-1"] = Service{
		ServiceName: "my-service-1",
		ServiceDest: []ServiceDest{
			{SrcPort: 1234, Port: "4321", ReqMode: "tcp"},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndSNI() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s

frontend service_1234
    bind *:1234
    mode tcp
    tcp-request inspect-delay 5s
    tcp-request content accept if { req_ssl_hello_type 1 }
    acl sni_my-service-14321
    use_backend my-service-1-be4321 if sni_my-service-14321%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service-1"] = Service{
		ServiceName: "my-service-1",
		ServiceDest: []ServiceDest{
			{SrcPort: 1234, Port: "4321", ReqMode: "sni"},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndSNI443() {
	defaultPortsOrig := os.Getenv("DEFAULT_PORTS")
	defer func() { os.Setenv("DEFAULT_PORTS", defaultPortsOrig) }()
	os.Setenv("DEFAULT_PORTS", "80")
	var actualData string
	tmpl := strings.Replace(
		s.TemplateContent,
		"\n    bind *:443",
		"",
		-1)
	expectedData := fmt.Sprintf(
		`%s

frontend service_443
    bind *:443
    mode tcp
    tcp-request inspect-delay 5s
    tcp-request content accept if { req_ssl_hello_type 1 }
    acl sni_my-service-14321
    use_backend my-service-1-be4321 if sni_my-service-14321%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service-1"] = Service{
		ServiceName: "my-service-1",
		ServiceDest: []ServiceDest{
			{SrcPort: 443, Port: "4321", ReqMode: "sni"},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndWithDomain() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_my-service1111 path_beg /path
    acl domain_my-service hdr(host) -i domain-1 domain-2
    use_backend my-service-be1111 if url_my-service1111 domain_my-service%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service"] = Service{
		ServiceName:   "my-service",
		ServiceDomain: []string{"domain-1", "domain-2"},
		AclName:       "my-service",
		PathType:      "path_beg",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndUserAgent() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_my-service1111 path_beg /path
    acl user_agent_my-service hdr_sub(User-Agent) -i agent-1 agent-2
    use_backend my-service-be1111 if url_my-service1111 user_agent_my-service%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service"] = Service{
		AclName:       "my-service",
		PathType:      "path_beg",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}, UserAgent: []string{"agent-1", "agent-2"}},
		},
		ServiceName:   "my-service",
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndWithDefaultBackend_WhenIsDefaultBackendIsTrue() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_my-service1111 path_beg /path
    use_backend my-service-be1111 if url_my-service1111
    default_backend my-service-be1111%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service"] = Service{
		ServiceName:      "my-service",
		IsDefaultBackend: true,
		AclName:          "my-service",
		PathType:         "path_beg",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndWithHdrDom_WhenServiceDomainMatchAllIsSet() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_my-service1111 path_beg /path
    acl domain_my-service hdr_dom(host) -i domain-1 domain-2
    use_backend my-service-be1111 if url_my-service1111 domain_my-service%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service"] = Service{
		ServiceName:           "my-service",
		ServiceDomain:         []string{"domain-1", "domain-2"},
		ServiceDomainMatchAll: true,
		AclName:               "my-service",
		PathType:              "path_beg",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndWithDomainWildcard() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl domain_my-service hdr_end(host) -i domain-1%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service"] = Service{
		ServiceName:   "my-service",
		ServiceDomain: []string{"*domain-1"},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsContentFrontEndWithHttpsPort() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_my-service1111 path_beg /path
    acl http_my-service src_port 80
    acl https_my-service src_port 443
    use_backend my-service-be1111 if url_my-service1111 http_my-service
    use_backend https-my-service-be1111 if url_my-service1111 https_my-service%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service"] = Service{
		ServiceName: "my-service",
		PathType:    "path_beg",
		HttpsPort:   2222,
		AclName:     "my-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_ForwardsToHttpsWhenHttpsOnlyIsTrue() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_my-service1111 path_beg /path
    redirect scheme https if !{ ssl_fc } url_my-service1111
    use_backend my-service-be1111 if url_my-service1111%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service"] = Service{
		ServiceName: "my-service",
		PathType:    "path_beg",
		HttpsOnly:   true,
		AclName:     "my-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_ForwardsToHttpsWhenRedirectWhenHttpProtoIsTrue() {
	var actualData string
	tmpl := s.TemplateContent
	expectedData := fmt.Sprintf(
		`%s
    acl url_my-service1111 path_beg /path
    acl is_my-service_http hdr(X-Forwarded-Proto) http
    redirect scheme https if is_my-service_http url_my-service1111
    use_backend my-service-be1111 if url_my-service1111%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}
	p := NewHaProxy(s.TemplatesPath, s.ConfigsPath)
	data.Services["my-service"] = Service{
		ServiceName:           "my-service",
		PathType:              "path_beg",
		RedirectWhenHttpProto: true,
		AclName:               "my-service",
		ServiceDest: []ServiceDest{
			{Port: "1111", ServicePath: []string{"/path"}},
		},
	}

	p.CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsBindPorts() {
	bindPortsOrig := os.Getenv("BIND_PORTS")
	defer func() { os.Setenv("BIND_PORTS", bindPortsOrig) }()
	os.Setenv("BIND_PORTS", "1234,4321")
	var actualData string
	expectedData := fmt.Sprintf(
		`%s
    bind *:1234
    bind *:4321%s`,
		s.TemplateContent,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsDefaultPorts() {
	defaultPortsOrig := os.Getenv("DEFAULT_PORTS")
	defer func() { os.Setenv("DEFAULT_PORTS", defaultPortsOrig) }()
	os.Setenv("DEFAULT_PORTS", "1234,4321 ssl crt /certs/my-cert.pem")
	var actualData string
	tmpl := strings.Replace(
		s.TemplateContent,
		"\n    bind *:80\n    bind *:443",
		"\n    bind *:1234\n    bind *:4321 ssl crt /certs/my-cert.pem",
		-1)
	expectedData := fmt.Sprintf(
		`%s%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsCerts() {
	readDirOrig := ReadDir
	defer func() {
		ReadDir = readDirOrig
	}()
	expected := "ssl"
	mockedFiles := []os.FileInfo{}
	for i := 1; i <= 3; i++ {
		certName := fmt.Sprintf("my-cert-%d", i)
		path := fmt.Sprintf("/certs/%s", certName)
		expected = fmt.Sprintf("%s crt %s", expected, path)
		file := FileInfoMock{
			NameMock: func() string {
				return certName
			},
			IsDirMock: func() bool {
				return false
			},
		}
		mockedFiles = append(mockedFiles, file)
	}
	ReadDir = func(dir string) ([]os.FileInfo, error) {
		if dir == "/certs" {
			return mockedFiles, nil
		}
		return []os.FileInfo{}, nil
	}
	var actualData string
	tmpl := strings.Replace(
		s.TemplateContent,
		"\n    bind *:80\n    bind *:443",
		"\n    bind *:80\n    bind *:443 "+expected,
		-1)
	expectedData := fmt.Sprintf(
		`%s%s`,
		tmpl,
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsUserList() {
	var actualData string
	usersOrig := os.Getenv("USERS")
	defer func() { os.Setenv("USERS", usersOrig) }()
	os.Setenv("USERS", "my-user-1:my-password-1,my-user-2:my-password-2")
	expectedData := fmt.Sprintf(
		"%s%s",
		strings.Replace(
			s.TemplateContent,
			"frontend services",
			`userlist defaultUsers
    user my-user-1 insecure-password my-password-1
    user my-user-2 insecure-password my-password-2

frontend services`,
			-1,
		),
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_AddsUserListWithEncryptedPasswordsOn() {
	var actualData string
	usersOrig := os.Getenv("USERS")
	encOrig := os.Getenv("USERS_PASS_ENCRYPTED")
	defer func() { os.Setenv("USERS", usersOrig); os.Setenv("USERS_PASS_ENCRYPTED", encOrig) }()
	os.Setenv("USERS", "my-user-1:my-password-1,my-user-2:my-password-2")
	os.Setenv("USERS_PASS_ENCRYPTED", "true")
	expectedData := fmt.Sprintf(
		"%s%s",
		strings.Replace(
			s.TemplateContent,
			"frontend services",
			`userlist defaultUsers
    user my-user-1 password my-password-1
    user my-user-2 password my-password-2

frontend services`,
			-1,
		),
		s.ServicesContent,
	)
	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_ReplacesValuesWithEnvVars() {
	tests := []struct {
		envKey string
		before string
		after  string
		value  string
	}{
		{"CONNECTION_MODE", "option  http-server-close", "option  different-connection-mode", "different-connection-mode"},
		{"TIMEOUT_CONNECT", "timeout connect 5s", "timeout connect 999s", "999"},
		{"TIMEOUT_CLIENT", "timeout client  20s", "timeout client  999s", "999"},
		{"TIMEOUT_SERVER", "timeout server  20s", "timeout server  999s", "999"},
		{"TIMEOUT_QUEUE", "timeout queue   30s", "timeout queue   999s", "999"},
		{"TIMEOUT_HTTP_REQUEST", "timeout http-request 5s", "timeout http-request 999s", "999"},
		{"TIMEOUT_HTTP_KEEP_ALIVE", "timeout http-keep-alive 15s", "timeout http-keep-alive 999s", "999"},
		{"TIMEOUT_TUNNEL", "timeout tunnel  3600s", "timeout tunnel  999s", "999"},
		{"STATS_USER", "stats auth admin:admin", "stats auth my-user:admin", "my-user"},
		{"STATS_PASS", "stats auth admin:admin", "stats auth admin:my-pass", "my-pass"},
		{"STATS_URI", "stats uri /admin?stats", "stats uri /proxyStats", "/proxyStats"},
	}
	for _, t := range tests {
		envOrig := os.Getenv(t.envKey)
		os.Setenv(t.envKey, t.value)
		var actualData string
		expectedData := fmt.Sprintf(
			"%s%s",
			strings.Replace(s.TemplateContent, t.before, t.after, -1),
			s.ServicesContent,
		)
		writeFile = func(filename string, data []byte, perm os.FileMode) error {
			actualData = string(data)
			return nil
		}

		NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

		s.Equal(expectedData, actualData)

		os.Setenv(t.envKey, envOrig)
	}
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_UsersStatsEnv() {
	tests := []struct {
		envKey     string
		before     string
		after      string
		value      string
		envKeyName string
	}{
		{"MY_USER", "stats auth admin:admin", "stats auth my-user:admin", "my-user", "STATS_USER_ENV"},
		{"MY_PASS", "stats auth admin:admin", "stats auth admin:my-pass", "my-pass", "STATS_PASS_ENV"},
		{"MY_STATS_URI", "stats uri /admin?stats", "stats uri /proxyStats", "/proxyStats", "STATS_URI_ENV"},
	}
	for _, t := range tests {
		os.Setenv(t.envKeyName, t.envKey)
		envOrig := os.Getenv(t.envKey)
		envKeyOrig := os.Getenv(t.envKeyName)
		os.Setenv(t.envKey, t.value)
		var actualData string
		expectedData := fmt.Sprintf(
			"%s%s",
			strings.Replace(s.TemplateContent, t.before, t.after, -1),
			s.ServicesContent,
		)
		writeFile = func(filename string, data []byte, perm os.FileMode) error {
			actualData = string(data)
			return nil
		}

		NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

		s.Equal(expectedData, actualData)

		os.Setenv(t.envKey, envOrig)
		os.Setenv(t.envKeyName, envKeyOrig)
	}
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_ReplacesValuesWithSecrets() {
	tests := []struct {
		secretFile string
		before     string
		after      string
		value      string
	}{
		{"dfp_connection_mode", "option  http-server-close", "option  different-connection-mode", "different-connection-mode"},
		{"dfp_timeout_connect", "timeout connect 5s", "timeout connect 999s", "999"},
		{"dfp_timeout_client", "timeout client  20s", "timeout client  999s", "999"},
		{"dfp_timeout_server", "timeout server  20s", "timeout server  999s", "999"},
		{"dfp_timeout_queue", "timeout queue   30s", "timeout queue   999s", "999"},
		{"dfp_timeout_http_request", "timeout http-request 5s", "timeout http-request 999s", "999"},
		{"dfp_timeout_http_keep_alive", "timeout http-keep-alive 15s", "timeout http-keep-alive 999s", "999"},
		{"dfp_timeout_tunnel", "timeout tunnel  3600s", "timeout tunnel  999s", "999"},
		{"dfp_stats_user", "stats auth admin:admin", "stats auth my-user:admin", "my-user"},
		{"dfp_stats_pass", "stats auth admin:admin", "stats auth admin:my-pass", "my-pass"},
		{"dfp_stats_uri", "stats uri /admin?stats", "stats uri /proxyStats", "/proxyStats"},
	}
	for _, t := range tests {
		timeoutOrig := os.Getenv(t.secretFile)
		var actualData string
		expectedData := fmt.Sprintf(
			"%s%s",
			strings.Replace(s.TemplateContent, t.before, t.after, -1),
			s.ServicesContent,
		)
		readSecretsFileOrig := readSecretsFile
		defer func() {
			readSecretsFile = readSecretsFileOrig
		}()
		readSecretsFile = func(dirname string) ([]byte, error) {
			if strings.HasSuffix(dirname, t.secretFile) {
				return []byte(t.value), nil
			}
			return []byte(""), fmt.Errorf("This is an error")
		}

		writeFile = func(filename string, data []byte, perm os.FileMode) error {
			actualData = string(data)
			return nil
		}

		NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

		s.Equal(expectedData, actualData, "Secret file %s is incorrect. It should be %s.", t.secretFile, t.value)

		os.Setenv(t.secretFile, timeoutOrig)
	}
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_WritesMockDataIfConfigsAreNotPresent() {
	var actualData string
	readConfigsDirOrig := readConfigsDir
	defer func() {
		readConfigsDir = readConfigsDirOrig
	}()
	readConfigsDir = func(dirname string) ([]os.FileInfo, error) {
		return []os.FileInfo{}, nil
	}
	expectedData := fmt.Sprintf(
		"%s%s",
		s.TemplateContent,
		`

    acl url_dummy path_beg /dummy
    use_backend dummy-be if url_dummy

backend dummy-be
    server dummy 1.1.1.1:1111 check`,
	)

	writeFile = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Equal(expectedData, actualData)
}

func (s HaProxyTestSuite) Test_CreateConfigFromTemplates_ReturnsError_WhenReadConfigsFileFails() {
	readConfigsFileOrig := readConfigsFile
	defer func() {
		readConfigsFile = readConfigsFileOrig
	}()
	readConfigsFile = func(dirname string) ([]byte, error) {
		return nil, fmt.Errorf("Could not read the directory")
	}

	err := NewHaProxy(s.TemplatesPath, s.ConfigsPath).CreateConfigFromTemplates()

	s.Error(err)
}

// ReadConfig

func (s *HaProxyTestSuite) Test_ReadConfig_ReturnsConfig() {
	var actualFilename string
	expectedFilename := fmt.Sprintf("%s/haproxy.cfg", s.ConfigsPath)
	expectedData := `template content

config1 content

config2 content`
	readFileOrig := ReadFile
	defer func() { ReadFile = readFileOrig }()
	ReadFile = func(filename string) ([]byte, error) {
		actualFilename = filename
		return []byte(expectedData), nil
	}

	actualData, _ := NewHaProxy(s.TemplatesPath, s.ConfigsPath).ReadConfig()

	s.Equal(expectedFilename, actualFilename)
	s.Equal(expectedData, actualData)
}

func (s *HaProxyTestSuite) Test_ReadConfig_ReturnsError_WhenReadFileFails() {
	readFileOrig := ReadFile
	defer func() { ReadFile = readFileOrig }()
	ReadFile = func(filename string) ([]byte, error) {
		return []byte{}, fmt.Errorf("This is an error")
	}

	_, actual := HaProxy{}.ReadConfig()

	s.Error(actual)
}

// Reload

func (s *HaProxyTestSuite) Test_Reload_ReadsPidFile() {
	var actual string
	readPidFile = func(fileName string) ([]byte, error) {
		actual = fileName
		return []byte("12345"), nil
	}

	HaProxy{}.Reload()

	s.Equal("/var/run/haproxy.pid", actual)
}

func (s *HaProxyTestSuite) Test_Reload_ReturnsError_WhenHaCommandFails() {
	cmdRunHa = func(args []string) error {
		return fmt.Errorf("This is an error")
	}

	err := HaProxy{}.Reload()

	s.Error(err)
}

func (s *HaProxyTestSuite) Test_Reload_ReturnsError_WhenReadPidFails() {
	readPidFile = func(fileName string) ([]byte, error) {
		return []byte(""), fmt.Errorf("This is an error")
	}

	err := HaProxy{}.Reload()

	s.Error(err)
}

func (s *HaProxyTestSuite) Test_Reload_RunsRunCmd() {
	actual := HaProxyTestSuite{}.mockHaExecCmd()
	expected := []string{
		"-f",
		"/cfg/haproxy.cfg",
		"-D",
		"-p",
		"/var/run/haproxy.pid",
		"-sf",
		s.Pid,
	}

	HaProxy{}.Reload()

	s.Equal(expected, *actual)
}

// AddService

func (s *HaProxyTestSuite) Test_AddService_AddsService() {
	s1 := Service{ServiceName: "my-service-1"}
	s2 := Service{ServiceName: "my-service-2"}
	s3 := Service{ServiceName: "my-service-2", ServiceDomain: []string{"domain-1", "domain-2"}}
	p := NewHaProxy("anything", "doesn't").(HaProxy)

	p.AddService(s1)
	p.AddService(s2)
	p.AddService(s3)

	s.Len(data.Services, 2)
	s.Equal(data.Services[s1.ServiceName], s1)
	s.Equal(data.Services[s3.ServiceName], s3)
}

// RemoveService

func (s *HaProxyTestSuite) Test_AddService_RemovesService() {
	s1 := Service{ServiceName: "my-service-1"}
	s2 := Service{ServiceName: "my-service-2"}
	s3 := Service{ServiceName: "my-service-2", ServiceDomain: []string{"domain-1", "domain-2"}}
	p := NewHaProxy("anything", "doesn't").(HaProxy)

	p.AddService(s1)
	p.AddService(s2)
	p.AddService(s3)
	p.RemoveService("my-service-1")

	s.Len(data.Services, 1)
	s.Equal(data.Services[s3.ServiceName], s3)
}

// Util

func (s *HaProxyTestSuite) getTemplateWithLogs() string {
	tmpl := strings.Replace(s.TemplateContent, "tune.ssl.default-dh-param 2048", "tune.ssl.default-dh-param 2048\n    log 127.0.0.1:1514 local0", -1)
	tmpl = strings.Replace(tmpl, "    option  dontlognull\n    option  dontlog-normal\n", "", -1)
	tmpl = strings.Replace(
		tmpl,
		`frontend services
    bind *:80
    bind *:443
    mode http
`,
		`frontend services
    bind *:80
    bind *:443
    mode http

    option httplog
    log global`,
		-1,
	)
	return tmpl
}

func (s *HaProxyTestSuite) getTemplateWithLogsAndErrorsOnly() string {
	tmpl := strings.Replace(s.TemplateContent, "tune.ssl.default-dh-param 2048", "tune.ssl.default-dh-param 2048\n    log 127.0.0.1:1514 local0", -1)
	tmpl = strings.Replace(tmpl, "    option  dontlognull\n", "", -1)
	tmpl = strings.Replace(
		tmpl,
		`frontend services
    bind *:80
    bind *:443
    mode http
`,
		`frontend services
    bind *:80
    bind *:443
    mode http

    option httplog
    log global`,
		-1,
	)
	return tmpl
}

// Mocks

type FileInfoMock struct {
	NameMock    func() string
	SizeMock    func() int64
	ModeMock    func() os.FileMode
	ModTimeMock func() time.Time
	IsDirMock   func() bool
	SysMock     func() interface{}
}

func (m FileInfoMock) Name() string {
	return m.NameMock()
}

func (m FileInfoMock) Size() int64 {
	return m.SizeMock()
}

func (m FileInfoMock) Mode() os.FileMode {
	return m.ModeMock()
}

func (m FileInfoMock) ModTime() time.Time {
	return m.ModTimeMock()
}

func (m FileInfoMock) IsDir() bool {
	return m.IsDirMock()
}

func (m FileInfoMock) Sys() interface{} {
	return m.SizeMock()
}

func (s HaProxyTestSuite) mockHaExecCmd() *[]string {
	var actualCommand []string
	cmdRunHa = func(args []string) error {
		actualCommand = args
		return nil
	}
	return &actualCommand
}
