package actions

import (
	"../proxy"
	"fmt"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"os"
	"testing"
)

type ReconfigureTestSuite struct {
	suite.Suite
	proxy.Service
	ConfigsPath     string
	TemplatesPath   string
	reconfigure     Reconfigure
	PutPathResponse string
	InstanceName    string
}

func (s *ReconfigureTestSuite) SetupTest() {
	sd := proxy.ServiceDest{
		ServicePath: []string{"path/to/my/service/api", "path/to/my/other/service/api"},
		Index:       0,
	}
	s.InstanceName = "proxy-test-instance"
	s.ServiceDest = []proxy.ServiceDest{sd}
	s.ConfigsPath = "path/to/configs/dir"
	s.TemplatesPath = "test_configs/tmpl"
	s.PathType = "path_beg"
	s.reconfigure = Reconfigure{
		BaseReconfigure: BaseReconfigure{
			TemplatesPath: s.TemplatesPath,
			ConfigsPath:   s.ConfigsPath,
			InstanceName:  s.InstanceName,
		},
		Service: proxy.Service{
			ServiceName: s.ServiceName,
			ServiceDest: []proxy.ServiceDest{sd},
			PathType:    s.PathType,
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

// GetTemplates

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpAuth_WhenUsersEnvIsPresent() {
	usersOrig := os.Getenv("USERS")
	defer func() { os.Setenv("USERS", usersOrig) }()
	os.Setenv("USERS", "anything")
	s.reconfigure.Service.ServiceDest[0].Port = "1234"
	s.reconfigure.Service.ServiceDest[0].Index = 0
	expected := `
backend myService-be1234_0
    mode http
    server myService myService:1234
    acl defaultUsersAcl http_auth(defaultUsers)
    http-request auth realm defaultRealm if !defaultUsersAcl
    http-request del-header Authorization`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpAuth_WhenUsersIsPresent() {
	s.reconfigure.Users = []proxy.User{
		{Username: "user-1", Password: "pass-1"},
		{Username: "user-2", Password: "pass-2"},
	}
	s.reconfigure.HttpsPort = 3333
	sd := []proxy.ServiceDest{
		{Port: "1111", Index: 0},
		{Port: "2222", IgnoreAuthorization: true, Index: 1},
	}
	s.reconfigure.Service.ServiceDest = sd
	expected := `userlist myServiceUsers
    user user-1 insecure-password pass-1
    user user-2 insecure-password pass-2


backend myService-be1111_0
    mode http
    server myService myService:1111
    acl myServiceUsersAcl http_auth(myServiceUsers)
    http-request auth realm myServiceRealm if !myServiceUsersAcl
    http-request del-header Authorization
backend myService-be2222_1
    mode http
    server myService myService:2222
backend https-myService-be1111_0
    mode http
    server myService myService:3333
    acl myServiceUsersAcl http_auth(myServiceUsers)
    http-request auth realm myServiceRealm if !myServiceUsersAcl
    http-request del-header Authorization
backend https-myService-be2222_1
    mode http
    server myService myService:3333`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpAuth_WhenUsersIsPresentAndPasswordsEncrypted() {
	s.reconfigure.Users = []proxy.User{
		{Username: "user-1", Password: "pass-1", PassEncrypted: true},
		{Username: "user-2", Password: "pass-2", PassEncrypted: false},
	}
	s.reconfigure.ServiceDest = []proxy.ServiceDest{{Port: "1234", Index: 6}}
	expected := `userlist myServiceUsers
    user user-1 password pass-1
    user user-2 insecure-password pass-2


backend myService-be1234_6
    mode http
    server myService myService:1234
    acl myServiceUsersAcl http_auth(myServiceUsers)
    http-request auth realm myServiceRealm if !myServiceUsersAcl
    http-request del-header Authorization`

	_, back, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, back)
}

func (s ReconfigureTestSuite) Test_GetTemplates_ReturnsFormattedContent() {
	s.reconfigure.Service.ServiceDest[0].Port = "1234"
	s.reconfigure.Service.ServiceDest[0].Index = 0
	expected := `
backend myService-be1234_0
    mode http
    server myService myService:1234`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsCheckResolversDocker_WhenCheckResolversIsTrue() {
	checkResolversOrig := os.Getenv("CHECK_RESOLVERS")
	defer func() { os.Setenv("CHECK_RESOLVERS", checkResolversOrig) }()
	os.Setenv("CHECK_RESOLVERS", "true")

	s.reconfigure.Service.ServiceDest[0].Port = "1234"
	s.reconfigure.Service.ServiceDest[0].Index = 0
	expected := `
backend myService-be1234_0
    mode http
    server myService myService:1234 check resolvers docker`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsRequestDeny_WhenVerifyClientSslIsTrue() {
	s.reconfigure.Service.ServiceDest[0].Port = "1234"
	s.reconfigure.Service.ServiceDest[0].VerifyClientSsl = true
	s.reconfigure.Service.ServiceDest[0].Index = 3
	expected := `
backend myService-be1234_3
    mode http
    acl valid_client_cert_myService1234 ssl_c_used ssl_c_verify 0
    http-request deny unless valid_client_cert_myService1234
    server myService myService:1234`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsRequestDeny_WhenNotOneOfAllowedMethods() {
	s.reconfigure.Service.HttpsPort = 4321
	s.reconfigure.Service.ServiceDest[0].Port = "1234"
	s.reconfigure.Service.ServiceDest[0].AllowedMethods = []string{"GET", "DELETE"}
	s.reconfigure.Service.ServiceDest[0].Index = 2
	expected := `
backend myService-be1234_2
    mode http
    acl valid_allowed_method method GET DELETE
    http-request deny unless valid_allowed_method
    server myService myService:1234
backend https-myService-be1234_2
    mode http
    acl valid_allowed_method method GET DELETE
    http-request deny unless valid_allowed_method
    server myService myService:4321`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsRequestDeny_WhenOneOfDeniedMethods() {
	s.reconfigure.Service.HttpsPort = 4321
	s.reconfigure.Service.ServiceDest[0].Port = "1234"
	s.reconfigure.Service.ServiceDest[0].Index = 5
	s.reconfigure.Service.ServiceDest[0].DeniedMethods = []string{"GET", "DELETE"}
	expected := `
backend myService-be1234_5
    mode http
    acl valid_denied_method method GET DELETE
    http-request deny if valid_denied_method
    server myService myService:1234
backend https-myService-be1234_5
    mode http
    acl valid_denied_method method GET DELETE
    http-request deny if valid_denied_method
    server myService myService:4321`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpDeny() {
	s.reconfigure.Service.HttpsPort = 4321
	s.reconfigure.Service.ServiceDest[0].Port = "1234"
	s.reconfigure.Service.ServiceDest[0].DenyHttp = true
	s.reconfigure.Service.ServiceDest[0].Index = 32
	expected := `
backend myService-be1234_32
    mode http
    http-request deny if !{ ssl_fc }
    server myService myService:1234
backend https-myService-be1234_32
    mode http
    server myService myService:4321`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddSllVerifyNone_WhenSslVerifyNoneIsSet() {
	s.reconfigure.Service.ServiceDest[0].Port = "1234"
	s.reconfigure.Service.ServiceDest[0].Index = 6
	s.reconfigure.SslVerifyNone = true
	expected := `
backend myService-be1234_6
    mode http
    server myService myService:1234 ssl verify none`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_ReturnsFormattedContent_WhenReqModeIsTcp() {
	s.reconfigure.Service.ServiceDest[0].ReqMode = "tcp"
	s.reconfigure.Service.ServiceDest[0].Port = "1234"
	s.reconfigure.Service.ServiceDest[0].Index = 12
	expected := `
backend myService-be1234_12
    mode tcp
    server myService myService:1234`

	_, actual, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, actual)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpsPort_WhenPresent() {
	expectedBack := `
backend myService-be1234_3
    mode http
    server myService myService:1234
backend https-myService-be1234_3
    mode http
    server myService myService:4321`
	s.reconfigure.ServiceDest[0].Port = "1234"
	s.reconfigure.ServiceDest[0].Index = 3
	s.reconfigure.HttpsPort = 4321
	actualFront, actualBack, _ := s.reconfigure.GetTemplates()

	s.Equal("", actualFront)
	s.Equal(expectedBack, actualBack)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsConnectionMode_WhenPresent() {
	expectedBack := `
backend myService-be1234_0
    mode http
    option my-connection-mode
    server myService myService:1234`
	s.reconfigure.ServiceDest[0].Port = "1234"
	s.reconfigure.ServiceDest[0].Index = 0
	s.reconfigure.ConnectionMode = "my-connection-mode"
	actualFront, actualBack, _ := s.reconfigure.GetTemplates()

	s.Equal("", actualFront)
	s.Equal(expectedBack, actualBack)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsTimeoutServer_WhenPresent() {
	expectedBack := `
backend myService-be1234_4
    mode http
    timeout server 9999s
    server myService myService:1234`
	s.reconfigure.ServiceDest[0].Port = "1234"
	s.reconfigure.ServiceDest[0].Index = 4
	s.reconfigure.TimeoutServer = "9999"
	actualFront, actualBack, _ := s.reconfigure.GetTemplates()

	s.Equal("", actualFront)
	s.Equal(expectedBack, actualBack)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsTimeoutTunnel_WhenPresent() {
	expectedBack := `
backend myService-be1234_3
    mode http
    timeout tunnel 9999s
    server myService myService:1234`
	s.reconfigure.ServiceDest[0].Port = "1234"
	s.reconfigure.ServiceDest[0].Index = 3
	s.reconfigure.TimeoutTunnel = "9999"
	actualFront, actualBack, _ := s.reconfigure.GetTemplates()

	s.Equal("", actualFront)
	s.Equal(expectedBack, actualBack)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsMultipleDestinations() {
	sd := []proxy.ServiceDest{
		{Port: "1111", Index: 0},
		{Port: "3333", ReqMode: "tcp", Index: 1},
		{Port: "5555", Index: 2},
	}
	expectedBack := `
backend myService-be1111_0
    mode http
    server myService myService:1111
backend myService-be3333_1
    mode tcp
    server myService myService:3333
backend myService-be5555_2
    mode http
    server myService myService:5555`
	s.reconfigure.ServiceDest = sd
	actualFront, actualBack, _ := s.reconfigure.GetTemplates()

	s.Equal("", actualFront)
	s.Equal(expectedBack, actualBack)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsHttpRequestSetPath_WhenReqPathSearchAndReqPathReplaceArePresent() {
	s.reconfigure.ReqPathSearch = "this"
	s.reconfigure.ReqPathReplace = "that"
	s.reconfigure.ServiceDest = []proxy.ServiceDest{{Port: "1234", Index: 0}}
	expected := fmt.Sprintf(`
backend myService-be1234_0
    mode http
    http-request set-path %%[path,regsub(%s,%s)]
    server myService myService:1234`,
		s.reconfigure.ReqPathSearch,
		s.reconfigure.ReqPathReplace,
	)

	_, backend, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, backend)
}

func (s ReconfigureTestSuite) Test_GetTemplates_AddsBackendExtra() {
	s.reconfigure.BackendExtra = "Additional backend"
	s.reconfigure.ServiceDest = []proxy.ServiceDest{{Port: "1234", Index: 0}}
	expected := fmt.Sprintf(`
backend myService-be1234_0
    mode http
    server myService myService:1234
    %s`,
		s.reconfigure.BackendExtra,
	)

	_, backend, _ := s.reconfigure.GetTemplates()

	s.Equal(expected, backend)
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

	actualFe, actualBe, _ := s.reconfigure.GetTemplates() //tu było s.Service

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

	_, _, err := s.reconfigure.GetTemplates() //tu było s.Service

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

	_, _, err := s.reconfigure.GetTemplates() //tu było s.Service

	s.Error(err)
}

// Execute

func (s ReconfigureTestSuite) Test_Execute_WritesBeTemplate() {
	s.reconfigure.ServiceDest[0].Port = "1234"
	s.reconfigure.ServiceDest[0].Index = 9
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s_9
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

func (s ReconfigureTestSuite) Test_Execute_WritesBeTemplateWithRedirectToHttps_WhenHttpsOnlyIsTrue() {
	s.reconfigure.ServiceDest[0].HttpsOnly = true
	s.reconfigure.ServiceDest[0].Index = 0
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s_0
    mode http
    redirect scheme https if !{ ssl_fc }
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

func (s ReconfigureTestSuite) Test_Execute_WritesServerSession() {
	s.reconfigure.ServiceName = "my-service"
	s.reconfigure.ServiceDest[0].Port = "1111"
	s.reconfigure.HttpsPort = 2222
	s.reconfigure.Tasks = []string{"1.2.3.4", "4.3.2.1"}
	s.reconfigure.SessionType = "sticky-server"
	var actualData string
	expectedData := `
backend my-service-be1111_0
    mode http
    balance roundrobin
    cookie my-service insert indirect nocache
    server my-service_0 1.2.3.4:1111 check cookie my-service_0
    server my-service_1 4.3.2.1:1111 check cookie my-service_1
backend https-my-service-be1111_0
    mode http
    balance roundrobin
    cookie my-service insert indirect nocache
    server my-service_0 1.2.3.4:2222 check cookie my-service_0
    server my-service_1 4.3.2.1:2222 check cookie my-service_1`
	writeBeTemplateOrig := writeBeTemplate
	defer func() { writeBeTemplate = writeBeTemplateOrig }()
	writeBeTemplate = func(filename string, data []byte, perm os.FileMode) error {
		actualData = string(data)
		return nil
	}

	s.reconfigure.Execute(true)

	s.Equal(expectedData, actualData)
}

func (s ReconfigureTestSuite) Test_Execute_AddsXForwardedProto_WhenTrue() {
	s.reconfigure.XForwardedProto = true
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s_0
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
	s.reconfigure.AddReqHeader = []string{"header-1", "header-2"}
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s_0
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
	s.reconfigure.AddResHeader = []string{"header-1", "header-2"}
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s_0
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
	s.reconfigure.SetReqHeader = []string{"header-1", "header-2"}
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s_0
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
	s.reconfigure.SetResHeader = []string{"header-1", "header-2"}
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s_0
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

func (s ReconfigureTestSuite) Test_Execute_DelReqHeader_WhenDelReqHeaderIsSet() {
	s.reconfigure.DelReqHeader = []string{"header-1", "header-2"}
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s_0
    mode http
    http-request del-header header-1
    http-request del-header header-2
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

func (s ReconfigureTestSuite) Test_Execute_DelResHeader_WhenDelResHeaderIsSet() {
	s.reconfigure.DelResHeader = []string{"header-1", "header-2"}
	var actualFilename, actualData string
	expectedFilename := fmt.Sprintf("%s/%s-be.cfg", s.TemplatesPath, s.ServiceName)
	expectedData := fmt.Sprintf(
		`
backend %s-be%s_0
    mode http
    http-response del-header header-1
    http-response del-header header-2
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

func (s ReconfigureTestSuite) Test_Execute_RemovesService_WhenProxyFails() {
	mockObj := getProxyMock("CreateConfigFromTemplates")
	mockObj.On("CreateConfigFromTemplates", mock.Anything, mock.Anything).Return(fmt.Errorf("This is an error"))
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = mockObj

	s.reconfigure.Execute(true)

	mockObj.AssertCalled(s.T(), "RemoveService", s.ServiceName)
}

func (s ReconfigureTestSuite) Test_Execute_ReloadsAgain_WhenProxyFails() {
	mockObj := getProxyMock("Reload")
	mockObj.On("Reload").Return(fmt.Errorf("This is an error"))
	proxyOrig := proxy.Instance
	defer func() { proxy.Instance = proxyOrig }()
	proxy.Instance = mockObj

	s.reconfigure.Execute(true)

	mockObj.AssertNumberOfCalls(s.T(), "Reload", 2)
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
	}
	r := NewReconfigure(
		BaseReconfigure{
			TemplatesPath: s.TemplatesPath,
			ConfigsPath:   s.ConfigsPath,
			InstanceName:  s.InstanceName,
		},
		expected,
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

func (s *ReconfigureTestSuite) Test_Execute_ReturnsError_WhenAddressIsNotAccessible() {
	s.reconfigure.ServiceName = "this-service-does-not-exist"
	lookupHostOrig := lookupHost
	defer func() {
		os.Setenv("SKIP_ADDRESS_VALIDATION", "true")
		lookupHost = lookupHostOrig
	}()
	os.Setenv("SKIP_ADDRESS_VALIDATION", "false")
	lookupHost = func(host string) (addrs []string, err error) {
		return []string{}, fmt.Errorf("This is an error")
	}

	err := s.reconfigure.Execute(true)

	s.Error(err)
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

func (m *ProxyMock) GetServices() map[string]proxy.Service {
	params := m.Called()
	return params.Get(0).(map[string]proxy.Service)
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
	if skipMethod != "GetServices" {
		mockObj.On("GetServices")
	}
	if skipMethod != "GetCertPaths" {
		mockObj.On("GetCertPaths")
	}
	return mockObj
}
