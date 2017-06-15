package proxy

import (
	"github.com/stretchr/testify/suite"
	"strconv"
	"strings"
	"testing"
)

type TypesTestSuite struct {
	suite.Suite
}

func (s *TypesTestSuite) SetupTest() {
	logPrintf = func(format string, v ...interface{}) {}
}

// mergeUsers

func (s *TypesTestSuite) Test_UsersMerge_AllCases() {
	usersBasePathOrig := usersBasePath
	defer func() { usersBasePath = usersBasePathOrig }()
	usersBasePath = "../test_configs/%s.txt"
	users := mergeUsers("someService", "user1:pass1,user2:pass2", "", false, "", false)
	s.Equal(users, []User{
		{PassEncrypted: false, Password: "pass1", Username: "user1"},
		{PassEncrypted: false, Password: "pass2", Username: "user2"},
	})
	users = mergeUsers("someService", "user1:pass1,user2", "", false, "", false)
	//user without password will not be included
	s.Equal(users, []User{
		{PassEncrypted: false, Password: "pass1", Username: "user1"},
	})
	users = mergeUsers("someService", "user1:passWoRd,user2", "users", false, "", false)
	//user2 password will come from users file
	s.Equal(users, []User{
		{PassEncrypted: false, Password: "passWoRd", Username: "user1"},
		{PassEncrypted: false, Password: "pass2", Username: "user2"},
	})

	users = mergeUsers("someService", "user1:passWoRd,user2", "users", true, "", false)
	//user2 password will come from users file, all encrypted
	s.Equal(users, []User{
		{PassEncrypted: true, Password: "passWoRd", Username: "user1"},
		{PassEncrypted: true, Password: "pass2", Username: "user2"},
	})

	users = mergeUsers("someService", "user1:passWoRd,user2", "users", false, "user1:pass1,user2:pass2", false)
	//user2 password will come from users file, but not from global one
	s.Equal(users, []User{
		{PassEncrypted: false, Password: "passWoRd", Username: "user1"},
		{PassEncrypted: false, Password: "pass2", Username: "user2"},
	})

	users = mergeUsers("someService", "user1:passWoRd,user2", "", false, "user1:pass1,user2:pass2", false)
	//user2 password will come from global file
	s.Equal(users, []User{
		{PassEncrypted: false, Password: "passWoRd", Username: "user1"},
		{PassEncrypted: false, Password: "pass2", Username: "user2"},
	})

	users = mergeUsers("someService", "user1:passWoRd,user2", "", false, "user1:pass1,user2:pass2", true)
	//user2 password will come from global file, globals encrypted only
	s.Equal(users, []User{
		{PassEncrypted: false, Password: "passWoRd", Username: "user1"},
		{PassEncrypted: true, Password: "pass2", Username: "user2"},
	})

	users = mergeUsers("someService", "user1:passWoRd,user2", "", true, "user1:pass1,user2:pass2", true)
	//user2 password will come from global file, all encrypted
	s.Equal(users, []User{
		{PassEncrypted: true, Password: "passWoRd", Username: "user1"},
		{PassEncrypted: true, Password: "pass2", Username: "user2"},
	})

	users = mergeUsers("someService", "user1,user2", "", false, "", false)
	//no users found dummy one generated
	s.Equal(len(users), 1)
	s.Equal(users[0].Username, "dummyUser")

	users = mergeUsers("someService", "", "users", false, "", false)
	//Users from file only
	s.Equal(users, []User{
		{PassEncrypted: false, Password: "pass1", Username: "user1"},
		{PassEncrypted: false, Password: "pass2", Username: "user2"},
	})

	users = mergeUsers("someService", "", "", false, "user1:pass1,user2:pass2", false)
	//No users when only globals present
	s.Equal(len(users), 0)

}

// NewRun

func (s TypesTestSuite) Test_ExtractUsersFromString() {

	users := extractUsersFromString("sn", "u:p", false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "p", Username: "u"},
	})

	users = extractUsersFromString("sn", "u:p", true, false)
	s.Equal(users, []*User{
		{PassEncrypted: true, Password: "p", Username: "u"},
	})

	users = extractUsersFromString("sn", "u:p:2", true, false)
	s.Equal(users, []*User{
		{PassEncrypted: true, Password: "p:2", Username: "u"},
	})

	users = extractUsersFromString("sn", "u", false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "", Username: "u"},
	})

	users = extractUsersFromString("sn", "u:p,ww", false, true)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "p", Username: "u"},
	})

	users = extractUsersFromString("sn", "u:p,ww:,:asd", false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "p", Username: "u"},
	})

	users = extractUsersFromString("sn", "u   ,    uu     ", false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "", Username: "u"},
		{PassEncrypted: false, Password: "", Username: "uu"},
	})

	users = extractUsersFromString("sn", "", false, false)
	s.Equal(users, []*User{})

	users = extractUsersFromString("sn", `u   ,
	 uu     `, false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "", Username: "u"},
		{PassEncrypted: false, Password: "", Username: "uu"},
	})
	users = extractUsersFromString("sn", `u
uu`, false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "", Username: "u"},
		{PassEncrypted: false, Password: "", Username: "uu"},
	})

	users = extractUsersFromString("sn",
		`u:p
uu:pp,
uuu:ppp

,

x:X`, false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "p", Username: "u"},
		{PassEncrypted: false, Password: "pp", Username: "uu"},
		{PassEncrypted: false, Password: "ppp", Username: "uuu"},
		{PassEncrypted: false, Password: "X", Username: "x"},
	})
}

// GetServiceFromMap

func (s *TypesTestSuite) Test_GetServiceFromMap_ReturnsProxyService() {
	expected := s.getExpectedService()
	serviceMap := s.getServiceMap(expected, "")
	actual := GetServiceFromMap(&serviceMap)
	s.Equal(expected, *actual)
}

// GetServiceFromProvider

func (s *TypesTestSuite) Test_GetServiceFromProvider_ReturnsProxyServiceWithIndexedData() {
	expected := s.getExpectedService()
	serviceMap := s.getServiceMap(expected, ".1")
	provider := mapParameterProvider{&serviceMap}
	actual := GetServiceFromProvider(&provider)
	s.Equal(expected, *actual)
}

// Suite

func TestRunUnitTestSuite(t *testing.T) {
	suite.Run(t, new(TypesTestSuite))
}

// Util

func (s *TypesTestSuite) getServiceMap(expected Service, indexSuffix string) map[string]string {
	return map[string]string{
		"aclName":               expected.AclName,
		"addReqHeader":          strings.Join(expected.AddReqHeader, ","),
		"addResHeader":          strings.Join(expected.AddResHeader, ","),
		"backendExtra":          expected.BackendExtra,
		"delReqHeader":          strings.Join(expected.DelReqHeader, ","),
		"delResHeader":          strings.Join(expected.DelResHeader, ","),
		"distribute":            strconv.FormatBool(expected.Distribute),
		"httpsOnly":             strconv.FormatBool(expected.HttpsOnly),
		"httpsPort":             strconv.Itoa(expected.HttpsPort),
		"isDefaultBackend":      strconv.FormatBool(expected.IsDefaultBackend),
		"outboundHostname":      expected.OutboundHostname,
		"pathType":              expected.PathType,
		"redirectWhenHttpProto": strconv.FormatBool(expected.RedirectWhenHttpProto),
		"reqPathReplace":        expected.ReqPathReplace,
		"reqPathSearch":         expected.ReqPathSearch,
		"serviceCert":           expected.ServiceCert,
		"serviceColor":          expected.ServiceColor,
		"serviceDomain":         strings.Join(expected.ServiceDomain, ","),
		"serviceDomainMatchAll": strconv.FormatBool(expected.ServiceDomainMatchAll),
		"serviceName":           expected.ServiceName,
		"setReqHeader":          strings.Join(expected.SetReqHeader, ","),
		"setResHeader":          strings.Join(expected.SetResHeader, ","),
		"sslVerifyNone":         strconv.FormatBool(expected.SslVerifyNone),
		"templateBePath":        expected.TemplateBePath,
		"templateFePath":        expected.TemplateFePath,
		"timeoutServer":         expected.TimeoutServer,
		"timeoutTunnel":         expected.TimeoutTunnel,
		"users":                 "user1:pass1,user2:pass2",
		"usersPassEncrypted":    "true",
		"xForwardedProto":       strconv.FormatBool(expected.XForwardedProto),
		// ServiceDest
		"ignoreAuthorization" + indexSuffix: strconv.FormatBool(expected.ServiceDest[0].IgnoreAuthorization),
		"port" + indexSuffix:                expected.ServiceDest[0].Port,
		"reqMode" + indexSuffix:             expected.ServiceDest[0].ReqMode,
		"serviceDomain" + indexSuffix:       strings.Join(expected.ServiceDest[0].ServiceDomain, ","),
		"servicePath" + indexSuffix:         strings.Join(expected.ServiceDest[0].ServicePath, ","),
		"userAgent" + indexSuffix:           strings.Join(expected.ServiceDest[0].UserAgent.Value, ","),
		"verifyClientSsl" + indexSuffix:     strconv.FormatBool(expected.ServiceDest[0].VerifyClientSsl),
	}
}

func (s *TypesTestSuite) getExpectedService() Service {
	return Service{
		AclName:               "aclName",
		AddReqHeader:          []string{"add-header-1", "add-header-2"},
		AddResHeader:          []string{"add-header-1", "add-header-2"},
		BackendExtra:          "additional backend config",
		DelReqHeader:          []string{"del-header-1", "del-header-2"},
		DelResHeader:          []string{"del-header-1", "del-header-2"},
		Distribute:            true,
		HttpsOnly:             true,
		HttpsPort:             1234,
		IsDefaultBackend:      true,
		OutboundHostname:      "outboundHostname",
		PathType:              "pathType",
		RedirectWhenHttpProto: true,
		ReqPathReplace:        "reqPathReplace",
		ReqPathSearch:         "reqPathSearch",
		ServiceCert:           "serviceCert",
		ServiceColor:          "serviceColor",
		ServiceDest: []ServiceDest{{
			IgnoreAuthorization: true,
			ServiceDomain:       []string{"domain1", "domain2"},
			ServicePath:         []string{"/"},
			Port:                "1234",
			ReqMode:             "reqMode",
			UserAgent:           UserAgent{Value: []string{"agent-1", "agent-2/replace-with_"}, AclName: "agent_1_agent_2_replace_with_"},
			VerifyClientSsl:     true,
		}},
		ServiceDomain:         []string{"domain1", "domain2"},
		ServiceDomainMatchAll: true,
		ServiceName:           "serviceName",
		SetReqHeader:          []string{"set-header-1", "set-header-2"},
		SetResHeader:          []string{"set-header-1", "set-header-2"},
		SslVerifyNone:         true,
		TemplateBePath:        "templateBePath",
		TemplateFePath:        "templateFePath",
		TimeoutServer:         "timeoutServer",
		TimeoutTunnel:         "timeoutTunnel",
		XForwardedProto:       true,
		Users: []User{
			{Username: "user1", Password: "pass1", PassEncrypted: true},
			{Username: "user2", Password: "pass2", PassEncrypted: true},
		},
	}
}
