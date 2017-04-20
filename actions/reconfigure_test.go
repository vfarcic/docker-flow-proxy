// +build !integration

package actions

import (
	"../proxy"
	"../registry"
	"encoding/json"
	"fmt"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

type ReconfigureTestSuite struct {
	suite.Suite
	proxy.Service
	ConsulAddress     string
	ConsulTemplateFe  string
	ConsulTemplateBe  string
	ConfigsPath       string
	TemplatesPath     string
	reconfigure       Reconfigure
	Server            *httptest.Server
	PutPathResponse   string
	ConsulRequestBody proxy.Service
	InstanceName      string
	SkipCheck         bool
}

func (s *ReconfigureTestSuite) SetupTest() {
	sd := proxy.ServiceDest{
		ServicePath: []string{"path/to/my/service/api", "path/to/my/other/service/api"},
	}
	s.InstanceName = "proxy-test-instance"
	s.ServiceDest = []proxy.ServiceDest{sd}
	s.ConfigsPath = "path/to/configs/dir"
	s.TemplatesPath = "test_configs/tmpl"
	s.SkipCheck = false
	s.PathType = "path_beg"
	s.ConsulTemplateFe = `
    acl url_myService path_beg path/to/my/service/api path_beg path/to/my/other/service/api
    use_backend myService-be if url_myService`
	s.ConsulTemplateBe = `
backend myService-be
    mode http
    {{range $i, $e := service "myService" "any"}}
    server {{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}} check
    {{end}}`
	s.ConsulAddress = s.Server.URL
	s.reconfigure = Reconfigure{
		BaseReconfigure: BaseReconfigure{
			ConsulAddresses: []string{s.ConsulAddress},
			TemplatesPath:   s.TemplatesPath,
			ConfigsPath:     s.ConfigsPath,
			InstanceName:    s.InstanceName,
		},
		Service: proxy.Service{
			ServiceName: s.ServiceName,
			ServiceDest: []proxy.ServiceDest{sd},
			PathType:    s.PathType,
			SkipCheck:   false,
		},
	}
	os.Setenv("SKIP_ADDRESS_VALIDATION", "true")
}

// Suite

func TestReconfigureUnitTestSuite(t *testing.T) {
	logPrintf = func(format string, v ...interface{}) {}
	s := new(ReconfigureTestSuite)
	s.ServiceName = "myService"
	s.PutPathResponse = "PUT_PATH_OK"
	s.Server = GetTestServer(s.Service, s.InstanceName)
	defer s.Server.Close()
	registryInstanceOrig := registryInstance
	defer func() { registryInstance = registryInstanceOrig }()
	registryInstance = getRegistrarableMock("")
	writeFeTemplateOrig := writeFeTemplate
	defer func() { writeFeTemplate = writeFeTemplateOrig }()
	writeFeTemplate = func(filename string, data []byte, perm os.FileMode) error {
		return nil
	}
	writeBeTemplateOrig := writeBeTemplate
	defer func() { writeBeTemplate = writeBeTemplateOrig }()
	writeBeTemplate = func(filename string, data []byte, perm os.FileMode) error {
		return nil
	}
	mockObj := getProxyMock("")
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = mockObj
	suite.Run(t, s)
}

// GetTemplate

func (s ReconfigureTestSuite) Test_GetTemplates_ReturnsFormattedContent() {
	front, back, _ := s.reconfigure.GetTemplates()

	s.Equal("", front)
	s.Equal(s.ConsulTemplateBe, back)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpAuth_WhenUsersEnvIsPresent() {
	usersOrig := os.Getenv("USERS")
	defer func() { os.Setenv("USERS", usersOrig) }()
	os.Setenv("USERS", "anything")
	expected := `
backend myService-be
    mode http
    {{range $i, $e := service "myService" "any"}}
    server {{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}} check
    {{end}}
    acl defaultUsersAcl http_auth(defaultUsers)
    http-request auth realm defaultRealm if !defaultUsersAcl
    http-request del-header Authorization`

	_, back, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, back)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpAuth_WhenUsersIsPresent() {
	s.reconfigure.Users = []proxy.User{
		{Username: "user-1", Password: "pass-1"},
		{Username: "user-2", Password: "pass-2"},
	}
	expected := `userlist myServiceUsers
    user user-1 insecure-password pass-1
    user user-2 insecure-password pass-2


backend myService-be
    mode http
    {{range $i, $e := service "myService" "any"}}
    server {{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}} check
    {{end}}
    acl myServiceUsersAcl http_auth(myServiceUsers)
    http-request auth realm myServiceRealm if !myServiceUsersAcl
    http-request del-header Authorization`

	_, back, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, back)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpAuth_WhenUsersIsPresentAndPasswordsEncrypted() {
	s.reconfigure.Users = []proxy.User{
		{Username: "user-1", Password: "pass-1", PassEncrypted: true},
		{Username: "user-2", Password: "pass-2", PassEncrypted: false},
	}
	expected := `userlist myServiceUsers
    user user-1 password pass-1
    user user-2 insecure-password pass-2


backend myService-be
    mode http
    {{range $i, $e := service "myService" "any"}}
    server {{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}} check
    {{end}}
    acl myServiceUsersAcl http_auth(myServiceUsers)
    http-request auth realm myServiceRealm if !myServiceUsersAcl
    http-request del-header Authorization`

	_, back, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, back)
}

func (s ReconfigureTestSuite) Test_GetTemplates_ReturnsFormattedContent_WhenModeIsSwarm() {
	modes := []string{"service", "sWARm"}
	for _, mode := range modes {
		s.reconfigure.Mode = mode
		s.reconfigure.Service.ServiceDest[0].Port = "1234"
		expected := `
backend myService-be1234
    mode http
    server myService myService:1234`

		_, actual, _ := s.reconfigure.GetTemplates()

		s.Equal(expected, actual)
	}
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsLoggin_WhenDebug() {
	debugOrig := os.Getenv("DEBUG")
	defer func() { os.Setenv("DEBUG", debugOrig) }()
	os.Setenv("DEBUG", "true")
	expected := strings.Replace(
		s.ConsulTemplateBe,
		"mode http",
		`mode http
    log global`,
		-1,
	)

	_, back, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, back)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddSllVerifyNone_WhenSslVerifyNoneIsSet() {
	modes := []string{"service", "sWARm"}
	for _, mode := range modes {
		s.reconfigure.Mode = mode
		s.reconfigure.Service.ServiceDest[0].Port = "1234"
		s.reconfigure.SslVerifyNone = true
		expected := `
backend myService-be1234
    mode http
    server myService myService:1234 ssl verify none`

		_, actual, _ := s.reconfigure.GetTemplates()

		s.Equal(expected, actual)
	}
}

func (s ReconfigureTestSuite) Test_GetTemplates_ReturnsFormattedContent_WhenReqModeIsTcp() {
	s.reconfigure.Mode = "swarm"
	s.reconfigure.ReqMode = "tcp"
	s.reconfigure.Service.ServiceDest[0].Port = "1234"
	expected := `
backend myService-be1234
    mode tcp
    server myService myService:1234`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpAuth_WhenModeIsSwarmAndUsersEnvIsPresent() {
	usersOrig := os.Getenv("USERS")
	defer func() { os.Setenv("USERS", usersOrig) }()
	os.Setenv("USERS", "anything")
	s.reconfigure.Mode = "swarm"
	s.reconfigure.Service.ServiceDest[0].Port = "1234"
	expected := `
backend myService-be1234
    mode http
    server myService myService:1234
    acl defaultUsersAcl http_auth(defaultUsers)
    http-request auth realm defaultRealm if !defaultUsersAcl
    http-request del-header Authorization`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpAuth_WhenModeIsSwarmAndUsersIsPresent() {
	s.reconfigure.Users = []proxy.User{
		{Username: "user-1", Password: "pass-1"},
		{Username: "user-2", Password: "pass-2"},
	}
	s.reconfigure.Mode = "swarm"
	s.reconfigure.Service.ServiceDest[0].Port = "1234"
	expected := `userlist myServiceUsers
    user user-1 insecure-password pass-1
    user user-2 insecure-password pass-2


backend myService-be1234
    mode http
    server myService myService:1234
    acl myServiceUsersAcl http_auth(myServiceUsers)
    http-request auth realm myServiceRealm if !myServiceUsersAcl
    http-request del-header Authorization`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpsPort_WhenPresent() {
	expectedBack := `
backend myService-be1234
    mode http
    server myService myService:1234

backend https-myService-be1234
    mode http
    server myService myService:4321`
	s.reconfigure.ServiceDest[0].Port = "1234"
	s.reconfigure.Mode = "service"
	s.reconfigure.HttpsPort = 4321
	actualFront, actualBack, _ := s.reconfigure.GetTemplates()

	s.Equal("", actualFront)
	s.Equal(expectedBack, actualBack)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsTimeoutServer_WhenPresent() {
	expectedBack := `
backend myService-be1234
    mode http
    timeout server 9999s
    server myService myService:1234`
	s.reconfigure.ServiceDest[0].Port = "1234"
	s.reconfigure.TimeoutServer = "9999"
	s.reconfigure.Mode = "service"
	actualFront, actualBack, _ := s.reconfigure.GetTemplates()

	s.Equal("", actualFront)
	s.Equal(expectedBack, actualBack)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsTimeoutTunnel_WhenPresent() {
	expectedBack := `
backend myService-be1234
    mode http
    timeout tunnel 9999s
    server myService myService:1234`
	s.reconfigure.ServiceDest[0].Port = "1234"
	s.reconfigure.TimeoutTunnel = "9999"
	s.reconfigure.Mode = "service"
	actualFront, actualBack, _ := s.reconfigure.GetTemplates()

	s.Equal("", actualFront)
	s.Equal(expectedBack, actualBack)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsMultipleDestinations() {
	sd := []proxy.ServiceDest{
		{Port: "1111", ServicePath: []string{"path-1"}, SrcPort: 2222},
		{Port: "3333", ServicePath: []string{"path-2"}, SrcPort: 4444},
		{Port: "5555", ServicePath: []string{"path-3"}},
	}
	expectedBack := `
backend myService-be1111
    mode http
    server myService myService:1111
backend myService-be3333
    mode http
    server myService myService:3333
backend myService-be5555
    mode http
    server myService myService:5555`
	s.reconfigure.ServiceDest = sd
	s.reconfigure.Mode = "service"
	actualFront, actualBack, _ := s.reconfigure.GetTemplates()

	s.Equal("", actualFront)
	s.Equal(expectedBack, actualBack)
}

// TODO: Deprecated (dec. 2016).
func (s ReconfigureTestSuite) Test_GetTemplates_AddsReqRep_WhenReqRepSearchAndReqRepReplaceArePresent() {
	s.reconfigure.ReqRepSearch = "this"
	s.reconfigure.ReqRepReplace = "that"
	expected := fmt.Sprintf(`
backend myService-be
    mode http
    reqrep %s     %s
    {{range $i, $e := service "%s" "any"}}
    server {{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}} check
    {{end}}`,
		s.reconfigure.ReqRepSearch,
		s.reconfigure.ReqRepReplace,
		s.reconfigure.ServiceName,
	)

	_, backend, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, backend)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpRequestSetPath_WhenReqPathSearchAndReqPathReplaceArePresent() {
	s.reconfigure.ReqPathSearch = "this"
	s.reconfigure.ReqPathReplace = "that"
	expected := fmt.Sprintf(`
backend myService-be
    mode http
    http-request set-path %%[path,regsub(%s,%s)]
    {{range $i, $e := service "%s" "any"}}
    server {{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}} check
    {{end}}`,
		s.reconfigure.ReqPathSearch,
		s.reconfigure.ReqPathReplace,
		s.reconfigure.ServiceName,
	)

	_, backend, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, backend)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsColor() {
	s.reconfigure.ServiceColor = "black"
	expected := fmt.Sprintf(`service "%s-%s"`, s.ServiceName, s.reconfigure.ServiceColor)

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Contains(actual, expected)
}

func (s ReconfigureTestSuite) Test_GetTemplates_DoesNotSetCheckWhenSkipCheckIsTrue() {
	s.ConsulTemplateBe = strings.Replace(s.ConsulTemplateBe, " check", "", -1)
	s.reconfigure.SkipCheck = true
	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(s.ConsulTemplateBe, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_ReturnsFileContent_WhenConsulTemplatePathIsSet() {
	expected := "This is content of a template"
	readTemplateFileOrig := readTemplateFile
	defer func() { readTemplateFile = readTemplateFileOrig }()
	readTemplateFile = func(dirname string) ([]byte, error) {
		return []byte(expected), nil
	}
	s.reconfigure.Service.ConsulTemplateFePath = "/path/to/my/consul/fe/template"
	s.reconfigure.Service.ConsulTemplateBePath = "/path/to/my/consul/be/template"

	_, actual, _ := s.reconfigure.GetTemplates()//tu było s.Service

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_ProcessesTemplateFromTemplatePath_WhenSpecified() {
	expectedFeFile := "/path/to/my/fe/template"
	expectedBeFile := "/path/to/my/be/template"
	expectedFe := fmt.Sprintf("This is service %s", s.reconfigure.ServiceName)
	expectedBe := fmt.Sprintf("This is path %s", s.reconfigure.ServiceDest[0].ServicePath)
	readTemplateFileOrig := readTemplateFile
	defer func() { readTemplateFile = readTemplateFileOrig }()
	readTemplateFile = func(filename string) ([]byte, error) {
		if filename == expectedFeFile {
			return []byte("This is service {{.ServiceName}}"), nil
		} else if filename == expectedBeFile {
			return []byte("This is path {{range .ServiceDest}}{{.ServicePath}}{{end}}"), nil
		}
		return []byte(""), fmt.Errorf("This is an error")
	}
	s.reconfigure.Service.TemplateFePath = expectedFeFile
	s.reconfigure.Service.TemplateBePath = expectedBeFile

	actualFe, actualBe, _ := s.reconfigure.GetTemplates()//tu było s.Service

	s.Equal(expectedFe, actualFe)
	s.Equal(expectedBe, actualBe)
}

func (s ReconfigureTestSuite) Test_GetTemplates_ReturnsError_WhenTemplateFePathIsNotPresent() {
	testFilename := "/path/to/my/template"
	readTemplateFileOrig := readTemplateFile
	defer func() { readTemplateFile = readTemplateFileOrig }()
	readTemplateFile = func(filename string) ([]byte, error) {
		if filename == testFilename {
			return []byte(""), fmt.Errorf("This is an error")
		}
		return []byte(""), nil
	}
	s.reconfigure.Service.TemplateFePath = testFilename
	s.reconfigure.Service.TemplateBePath = "not/under/test"

	_, _, err := s.reconfigure.GetTemplates()//tu było s.Service

	s.Error(err)
}

func (s ReconfigureTestSuite) Test_GetTemplates_ReturnsError_WhenTemplateBePathIsNotPresent() {
	testFilename := "/path/to/my/template"
	readTemplateFileOrig := readTemplateFile
	defer func() { readTemplateFile = readTemplateFileOrig }()
	readTemplateFile = func(filename string) ([]byte, error) {
		if filename == testFilename {
			return []byte(""), fmt.Errorf("This is an error")
		}
		return []byte(""), nil
	}

	s.reconfigure.Service.TemplateFePath = "not/under/test"
	s.reconfigure.Service.TemplateBePath = testFilename

	_, _, err := s.reconfigure.GetTemplates()//tu było s.Service

	s.Error(err)
}

func (s ReconfigureTestSuite) Test_GetTemplates_ReturnsError_WhenConsulTemplateFileIsNotAvailable() {
	readTemplateFileOrig := readTemplateFile
	defer func() { readTemplateFile = readTemplateFileOrig }()
	readTemplateFile = func(filename string) ([]byte, error) {
		return nil, fmt.Errorf("This is an error")
	}
	s.reconfigure.Service.ConsulTemplateFePath = "/path/to/my/consul/fe/template"
	s.reconfigure.Service.ConsulTemplateBePath = "/path/to/my/consul/be/template"

	_, _, actual := s.reconfigure.GetTemplates()//tu było s.Service

	s.Error(actual)
}

// Execute

func (s ReconfigureTestSuite) Test_Execute_InvokesRegistrarableCreateConfigs() {
	mockObj := getRegistrarableMock("")
	registryInstanceOrig := registryInstance
	defer func() { registryInstance = registryInstanceOrig }()
	registryInstance = mockObj
	expectedArgs := registry.CreateConfigsArgs{
		Addresses:     []string{s.ConsulAddress},
		TemplatesPath: s.TemplatesPath,
		FeFile:        serviceTemplateFeFilename,
		FeTemplate:    "",
		BeFile:        serviceTemplateBeFilename,
		BeTemplate:    s.ConsulTemplateBe,
		ServiceName:   s.ServiceName,
	}

	s.reconfigure.Execute(true)

	mockObj.AssertCalled(s.T(), "CreateConfigs", &expectedArgs)
}

func (s ReconfigureTestSuite) Test_Execute_WritesBeTemplate_WhenModeIsService() {
	s.reconfigure.Mode = "SerVIce"
	s.reconfigure.ServiceDest[0].Port = "1234"
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s
    mode http
    server %s %s:%s`,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
		s.ServiceName,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
	)
	writeBeTemplateOrig := writeBeTemplate
	defer func() { writeBeTemplate = writeBeTemplateOrig }()
	writeBeTemplate = func(filename string, data []byte, perm os.FileMode) error {
		actualFilename = filename
		actualData = string(data)
		return nil
	}

	s.reconfigure.Execute(true)

	s.Equal(expectedFilename, actualFilename)
	s.Equal(expectedData, actualData)
}

func (s ReconfigureTestSuite) Test_Execute_WritesBeTemplate_WhenModeIsSwarm() {
	s.reconfigure.Mode = "sWArm"
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s
    mode http
    server %s %s:%s`,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
		s.ServiceName,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
	)
	writeBeTemplateOrig := writeBeTemplate
	defer func() { writeBeTemplate = writeBeTemplateOrig }()
	writeBeTemplate = func(filename string, data []byte, perm os.FileMode) error {
		actualFilename = filename
		actualData = string(data)
		return nil
	}

	s.reconfigure.Execute(true)

	s.Equal(expectedFilename, actualFilename)
	s.Equal(expectedData, actualData)
}

func (s ReconfigureTestSuite) Test_Execute_AddsXForwardedProto_WhenTrue() {
	s.reconfigure.Mode = "swarm"
	s.reconfigure.XForwardedProto = true
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s
    mode http
    http-request add-header X-Forwarded-Proto https if { ssl_fc }
    server %s %s:%s`,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
		s.ServiceName,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
	)
	writeBeTemplateOrig := writeBeTemplate
	defer func() { writeBeTemplate = writeBeTemplateOrig }()
	writeBeTemplate = func(filename string, data []byte, perm os.FileMode) error {
		actualFilename = filename
		actualData = string(data)
		return nil
	}

	s.reconfigure.Execute(true)

	s.Equal(expectedFilename, actualFilename)
	s.Equal(expectedData, actualData)
}

func (s ReconfigureTestSuite) Test_Execute_AddsReqHeader_WhenAddReqHeaderIsSet() {
	s.reconfigure.Mode = "swarm"
	s.reconfigure.AddReqHeader = []string{"header-1", "header-2"}
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s
    mode http
    http-request add-header header-1
    http-request add-header header-2
    server %s %s:%s`,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
		s.ServiceName,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
	)
	writeBeTemplateOrig := writeBeTemplate
	defer func() { writeBeTemplate = writeBeTemplateOrig }()
	writeBeTemplate = func(filename string, data []byte, perm os.FileMode) error {
		actualFilename = filename
		actualData = string(data)
		return nil
	}

	s.reconfigure.Execute(true)

	s.Equal(expectedFilename, actualFilename)
	s.Equal(expectedData, actualData)
}

func (s ReconfigureTestSuite) Test_Execute_AddsResHeader_WhenAddResHeaderIsSet() {
	s.reconfigure.Mode = "swarm"
	s.reconfigure.AddResHeader = []string{"header-1", "header-2"}
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s
    mode http
    http-response add-header header-1
    http-response add-header header-2
    server %s %s:%s`,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
		s.ServiceName,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
	)
	writeBeTemplateOrig := writeBeTemplate
	defer func() { writeBeTemplate = writeBeTemplateOrig }()
	writeBeTemplate = func(filename string, data []byte, perm os.FileMode) error {
		actualFilename = filename
		actualData = string(data)
		return nil
	}

	s.reconfigure.Execute(true)

	s.Equal(expectedFilename, actualFilename)
	s.Equal(expectedData, actualData)
}

func (s ReconfigureTestSuite) Test_Execute_AddsReqHeader_WhenSetReqHeaderIsSet() {
	s.reconfigure.Mode = "swarm"
	s.reconfigure.SetReqHeader = []string{"header-1", "header-2"}
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s
    mode http
    http-request set-header header-1
    http-request set-header header-2
    server %s %s:%s`,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
		s.ServiceName,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
	)
	writeBeTemplateOrig := writeBeTemplate
	defer func() { writeBeTemplate = writeBeTemplateOrig }()
	writeBeTemplate = func(filename string, data []byte, perm os.FileMode) error {
		actualFilename = filename
		actualData = string(data)
		return nil
	}

	s.reconfigure.Execute(true)

	s.Equal(expectedFilename, actualFilename)
	s.Equal(expectedData, actualData)
}

func (s ReconfigureTestSuite) Test_Execute_AddsResHeader_WhenSetResHeaderIsSet() {
	s.reconfigure.Mode = "swarm"
	s.reconfigure.SetResHeader = []string{"header-1", "header-2"}
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s
    mode http
    http-response set-header header-1
    http-response set-header header-2
    server %s %s:%s`,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
		s.ServiceName,
		s.ServiceName,
		s.reconfigure.ServiceDest[0].Port,
	)
	writeBeTemplateOrig := writeBeTemplate
	defer func() { writeBeTemplate = writeBeTemplateOrig }()
	writeBeTemplate = func(filename string, data []byte, perm os.FileMode) error {
		actualFilename = filename
		actualData = string(data)
		return nil
	}

	s.reconfigure.Execute(true)

	s.Equal(expectedFilename, actualFilename)
	s.Equal(expectedData, actualData)
}

func (s ReconfigureTestSuite) Test_Execute_DoesNotInvokeRegistrarableCreateConfigs_WhenModeIsService() {
	mockObj := getRegistrarableMock("")
	registryInstanceOrig := registryInstance
	defer func() { registryInstance = registryInstanceOrig }()
	registryInstance = mockObj
	s.reconfigure.Mode = "seRviCe"

	s.reconfigure.Execute(true)

	mockObj.AssertNotCalled(s.T(), "CreateConfigs", mock.Anything)
}

func (s ReconfigureTestSuite) Test_Execute_DoesNotInvokeRegistrarableCreateConfigs_WhenModeIsSwarm() {
	mockObj := getRegistrarableMock("")
	registryInstanceOrig := registryInstance
	defer func() { registryInstance = registryInstanceOrig }()
	registryInstance = mockObj
	s.reconfigure.Mode = "sWaRm"

	s.reconfigure.Execute(true)

	mockObj.AssertNotCalled(s.T(), "CreateConfigs", mock.Anything)
}

func (s ReconfigureTestSuite) Test_Execute_ReturnsError_WhenRegistrarableCreateConfigsFails() {
	mockObj := getRegistrarableMock("CreateConfigs")
	mockObj.On(
		"CreateConfigs",
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
		mock.Anything,
	).Return(fmt.Errorf("This is an error"))
	registryInstanceOrig := registryInstance
	defer func() { registryInstance = registryInstanceOrig }()
	registryInstance = mockObj

	actual := s.reconfigure.Execute(true)

	s.Error(actual)
}

func (s ReconfigureTestSuite) Test_Execute_InvokesProxyCreateConfigFromTemplates() {
	mockObj := getProxyMock("")
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = mockObj

	s.reconfigure.Execute(true)

	mockObj.AssertCalled(s.T(), "CreateConfigFromTemplates")
}

func (s ReconfigureTestSuite) Test_Execute_ReturnsError_WhenProxyFails() {
	mockObj := getProxyMock("CreateConfigFromTemplates")
	mockObj.On("CreateConfigFromTemplates", mock.Anything, mock.Anything).Return(fmt.Errorf("This is an error"))
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = mockObj

	err := s.reconfigure.Execute(true)

	s.Error(err)
}

func (s ReconfigureTestSuite) Test_Execute_AddsService() {
	mockObj := getProxyMock("")
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = mockObj
	sd := proxy.ServiceDest{
		ServicePath: []string{"path/to/my/service/api", "path/to/my/other/service/api"},
	}
	expected := proxy.Service{
		ServiceName: "s.ServiceName",
		ServiceDest: []proxy.ServiceDest{sd},
		PathType:    s.PathType,
		SkipCheck:   false,
	}
	r := NewReconfigure(
		BaseReconfigure{
			ConsulAddresses: []string{s.ConsulAddress},
			TemplatesPath:   s.TemplatesPath,
			ConfigsPath:     s.ConfigsPath,
			InstanceName:    s.InstanceName,
		},
		expected,
		"",
	)

	r.Execute(true)

	mockObj.AssertCalled(s.T(), "AddService", mock.Anything)
}

func (s ReconfigureTestSuite) Test_Execute_DoesNotInvokeAddService_WhenTemplatesAreSet() {
	mockObj := getProxyMock("")
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = mockObj
	readTemplateFileOrig := readTemplateFile
	defer func() { readTemplateFile = readTemplateFileOrig }()
	readTemplateFile = func(filename string) ([]byte, error) {
		return []byte(""), nil
	}
	expected := proxy.Service{
		TemplateBePath: "something",
		TemplateFePath: "something",
	}
	r := NewReconfigure(
		BaseReconfigure{},
		expected,
		"",
	)

	r.Execute(true)

	mockObj.AssertNotCalled(s.T(), "AddService", mock.Anything)
}

func (s ReconfigureTestSuite) Test_Execute_InvokesHaProxyReload() {
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	mock := getProxyMock("")
	proxy.Instance = mock

	s.reconfigure.Execute(true)

	mock.AssertCalled(s.T(), "Reload")
}

func (s *ReconfigureTestSuite) Test_Execute_PutsDataToConsul() {
	s.SkipCheck = true
	s.reconfigure.SkipCheck = true
	s.reconfigure.ServiceDomain = s.ServiceDomain
	s.reconfigure.ConsulTemplateFePath = s.ConsulTemplateFePath
	s.reconfigure.ConsulTemplateBePath = s.ConsulTemplateBePath
	mockObj := getRegistrarableMock("")
	registryInstanceOrig := registryInstance
	defer func() { registryInstance = registryInstanceOrig }()
	registryInstance = mockObj
	r := registry.Registry{
		ServiceName:          s.ServiceName,
		ServiceColor:         s.ServiceColor,
		ServicePath:          s.ServiceDest[0].ServicePath,
		ServiceDomain:        s.ServiceDomain,
		OutboundHostname:     s.OutboundHostname,
		PathType:             s.PathType,
		SkipCheck:            s.SkipCheck,
		ConsulTemplateFePath: s.ConsulTemplateFePath,
		ConsulTemplateBePath: s.ConsulTemplateBePath,
	}
	proxyMockObj := getProxyMock("")
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = proxyMockObj

	s.reconfigure.Execute(true)

	mockObj.AssertCalled(s.T(), "PutService", []string{s.ConsulAddress}, s.InstanceName, r)
}

func (s *ReconfigureTestSuite) Test_Execute_DoesNotPutDataToConsul_WhenModeIsServiceAndConsulAddressIsEmpty() {
	s.verifyDoesNotPutDataToConsul("seRViCe")
}

func (s *ReconfigureTestSuite) Test_Execute_DoesNotPutDataToConsul_WhenModeIsSwarmAndConsulAddressIsEmpty() {
	s.verifyDoesNotPutDataToConsul("SWARm")
}

func (s *ReconfigureTestSuite) Test_Execute_ReturnsError_WhenPutToConsulFails() {
	registryInstanceOrig := registryInstance
	defer func() { registryInstance = registryInstanceOrig }()
	mockObj := getRegistrarableMock("PutService")
	mockObj.On("PutService", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("This is an error"))
	registryInstance = mockObj

	actual := s.reconfigure.Execute(true)

	s.Error(actual)
}

func (s *ReconfigureTestSuite) Test_Execute_AddsHttpIfNotPresentInPutToConsul() {
	s.reconfigure.ConsulAddresses = []string{strings.Replace(s.ConsulAddress, "http://", "", -1)}

	s.reconfigure.Execute(true)

	s.Equal(s.ServiceColor, s.ConsulRequestBody.ServiceColor)
}

func (s *ReconfigureTestSuite) Test_Execute_SendsServicePathToConsul() {
	s.reconfigure.Execute(true)

	s.Equal(s.reconfigure.ServiceColor, s.ConsulRequestBody.ServiceColor)
}

func (s *ReconfigureTestSuite) Test_Execute_ReturnsError_WhenConsulTemplateFileIsNotAvailable() {
	readTemplateFileOrig := readTemplateFile
	defer func() { readTemplateFile = readTemplateFileOrig }()
	readTemplateFile = func(dirname string) ([]byte, error) {
		return nil, fmt.Errorf("This is an error")
	}
	s.reconfigure.Service.ConsulTemplateFePath = "/path/to/my/consul/fe/template"
	s.reconfigure.Service.ConsulTemplateBePath = "/path/to/my/consul/be/template"

	err := s.reconfigure.Execute(true)

	s.Error(err)
}

func (s *ReconfigureTestSuite) Test_Execute_ReturnsError_WhenAddressIsNotAccessible() {
	s.reconfigure.Mode = "swarm"
	s.reconfigure.ServiceName = "this-service-does-not-exist"
	defer func() { os.Setenv("SKIP_ADDRESS_VALIDATION", "true") }()
	os.Setenv("SKIP_ADDRESS_VALIDATION", "false")

	err := s.reconfigure.Execute(true)

	s.Error(err)
	//	s.NoError(err)
}

// NewReconfigure

func (s *ReconfigureTestSuite) Test_NewReconfigure_AddsBaseAndService() {
	br := BaseReconfigure{ConsulAddresses: []string{"myConsulAddress"}}
	sr := proxy.Service{ServiceName: "myService"}

	r := NewReconfigure(br, sr, "")

	actualBr, actualSr := r.GetData()
	s.Equal(br, actualBr)
	s.Equal(sr, actualSr)
}

func (s *ReconfigureTestSuite) Test_NewReconfigure_CreatesNewStruct() {
	r1 := NewReconfigure(
		BaseReconfigure{ConsulAddresses: []string{"myConsulAddress"}},
		proxy.Service{ServiceName: "myService"},
		"",
	)
	r2 := NewReconfigure(BaseReconfigure{}, proxy.Service{}, "")

	actualBr1, actualSr1 := r1.GetData()
	actualBr2, actualSr2 := r2.GetData()
	s.NotEqual(actualBr1, actualBr2)
	s.NotEqual(actualSr1, actualSr2)
}





// Mock

type ReconfigureMock struct {
	mock.Mock
}

func (m *ReconfigureMock) Execute(reloadAfter bool) error {
	params := m.Called(reloadAfter)
	return params.Error(0)
}

func (m *ReconfigureMock) GetData() (BaseReconfigure, proxy.Service) {
	m.Called()
	return BaseReconfigure{}, proxy.Service{}
}


func (m *ReconfigureMock) GetTemplates() (front, back string, err error) {
	params := m.Called()
	return params.String(0), params.String(1), params.Error(2)
}


func getReconfigureMock(skipMethod string) *ReconfigureMock {
	mockObj := new(ReconfigureMock)
	if skipMethod != "Execute" {
		mockObj.On("Execute", mock.Anything).Return(nil)
	}
	if skipMethod != "GetData" {
		mockObj.On("GetData").Return(nil)
	}
	if skipMethod != "GetTemplates" {
		mockObj.On("GetTemplates").Return("", "", nil)
	}
	return mockObj
}

type RegistrarableMock struct {
	mock.Mock
}

func (m *RegistrarableMock) PutService(addresses []string, instanceName string, r registry.Registry) error {
	params := m.Called(addresses, instanceName, r)
	return params.Error(0)
}

func (m *RegistrarableMock) SendPutRequest(addresses []string, serviceName, key, value, instanceName string, c chan error) {
	m.Called(addresses, serviceName, key, value, instanceName, c)
}

func (m *RegistrarableMock) DeleteService(addresses []string, serviceName, instanceName string) error {
	params := m.Called(addresses, serviceName, instanceName)
	return params.Error(0)
}

func (m *RegistrarableMock) SendDeleteRequest(addresses []string, serviceName, key, value, instanceName string, c chan error) {
	m.Called(addresses, serviceName, key, value, instanceName, c)
}

func (m *RegistrarableMock) CreateConfigs(args *registry.CreateConfigsArgs) error {
	params := m.Called(args)
	return params.Error(0)
}

func (m *RegistrarableMock) GetServiceAttribute(addresses []string, instanceName, serviceName, key string) (string, error) {
	params := m.Called(addresses, instanceName, serviceName, key)
	if serviceName == "path" {
		return "path/to/my/service/api,path/to/my/other/service/api", params.Error(0)
	}
	return "something", params.Error(0)
}

func getRegistrarableMock(skipMethod string) *RegistrarableMock {
	mockObj := new(RegistrarableMock)
	if skipMethod != "PutService" {
		mockObj.On("PutService", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	}
	if skipMethod != "SendPutRequest" {
		mockObj.On("SendPutRequest", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	}
	if skipMethod != "DeleteService" {
		mockObj.On("DeleteService", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	}
	if skipMethod != "SendDeleteRequest" {
		mockObj.On("SendDeleteRequest", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything)
	}
	if skipMethod != "CreateConfigs" {
		mockObj.On("CreateConfigs", mock.Anything).Return(nil)
	}
	if skipMethod != "GetServiceAttribute" {
		mockObj.On("GetServiceAttribute", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	}
	return mockObj
}

type ProxyMock struct {
	mock.Mock
}

func (m *ProxyMock) RunCmd(extraArgs []string) error {
	params := m.Called(extraArgs)
	return params.Error(0)
}

func (m *ProxyMock) CreateConfigFromTemplates() error {
	params := m.Called()
	return params.Error(0)
}

func (m *ProxyMock) ReadConfig() (string, error) {
	params := m.Called()
	return params.String(0), params.Error(1)
}

func (m *ProxyMock) Reload() error {
	params := m.Called()
	return params.Error(0)
}

func (m *ProxyMock) AddCert(certName string) {
	m.Called(certName)
}

func (m *ProxyMock) GetCerts() map[string]string {
	params := m.Called()
	return params.Get(0).(map[string]string)
}

func (m *ProxyMock) AddService(service proxy.Service) {
	m.Called(service)
}

func (m *ProxyMock) RemoveService(service string) {
	m.Called(service)
}

func (m *ProxyMock) GetCertPaths() []string {
	params := m.Called()
	return params.Get(0).([]string)
}

func getProxyMock(skipMethod string) *ProxyMock {
	mockObj := new(ProxyMock)
	if skipMethod != "RunCmd" {
		mockObj.On("RunCmd", mock.Anything).Return(nil)
	}
	if skipMethod != "CreateConfigFromTemplates" {
		mockObj.On("CreateConfigFromTemplates").Return(nil)
	}
	if skipMethod != "ReadConfig" {
		mockObj.On("ReadConfig").Return("", nil)
	}
	if skipMethod != "Reload" {
		mockObj.On("Reload").Return(nil)
	}
	if skipMethod != "AddCert" {
		mockObj.On("AddCert", mock.Anything).Return(nil)
	}
	if skipMethod != "GetCerts" {
		mockObj.On("GetCerts").Return(map[string]string{})
	}
	if skipMethod != "AddService" {
		mockObj.On("AddService", mock.Anything)
	}
	if skipMethod != "RemoveService" {
		mockObj.On("RemoveService", mock.Anything)
	}
	if skipMethod != "GetCertPaths" {
		mockObj.On("GetCertPaths")
	}
	return mockObj
}

// Util

func GetTestServer(s proxy.Service, InstanceName string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actualPath := r.URL.Path
		header := http.StatusOK
		if r.Method == "GET" && actualPath == "/v1/catalog/services" {
			w.Header().Set("Content-Type", "application/json")
			data := map[string][]string{"service1": {}, "service2": {}, s.ServiceName: {}}
			js, _ := json.Marshal(data)
			w.Write(js)
		} else if r.Method == "GET" && r.URL.RawQuery == "raw" {
			switch actualPath {
			case fmt.Sprintf("/v1/kv/%s/%s/%s", InstanceName, s.ServiceName, registry.PATH_KEY):
				w.Write([]byte(strings.Join(s.ServiceDest[0].ServicePath, ",")))
			case fmt.Sprintf("/v1/kv/%s/%s/%s", InstanceName, s.ServiceName, registry.COLOR_KEY):
				w.Write([]byte("orange"))
			case fmt.Sprintf("/v1/kv/%s/%s/%s", InstanceName, s.ServiceName, registry.DOMAIN_KEY):
				w.Write([]byte(strings.Join(s.ServiceDomain, ",")))
			case fmt.Sprintf("/v1/kv/%s/%s/%s", InstanceName, s.ServiceName, registry.HOSTNAME_KEY):
				w.Write([]byte(s.OutboundHostname))
			case fmt.Sprintf("/v1/kv/%s/%s/%s", InstanceName, s.ServiceName, registry.PATH_TYPE_KEY):
				w.Write([]byte(s.PathType))
			case fmt.Sprintf("/v1/kv/%s/%s/%s", InstanceName, s.ServiceName, registry.SKIP_CHECK_KEY):
				w.Write([]byte(fmt.Sprintf("%t", s.SkipCheck)))
			default:
				header = http.StatusNotFound
			}
		}
		w.WriteHeader(header)
	}))
}

func (s ReconfigureTestSuite) verifyDoesNotPutDataToConsul(mode string) {
	s.reconfigure.Mode = mode
	mockObj := getRegistrarableMock("")
	registryInstanceOrig := registryInstance
	defer func() { registryInstance = registryInstanceOrig }()
	registryInstance = mockObj
	consulAddress := s.ConsulAddress
	defer func() { s.reconfigure.ConsulAddresses = []string{consulAddress} }()
	s.reconfigure.ConsulAddresses = []string{}

	s.reconfigure.Execute(true)

	mockObj.AssertNotCalled(s.T(), "PutService", mock.Anything, mock.Anything, mock.Anything)
}
