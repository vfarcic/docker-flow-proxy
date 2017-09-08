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
	expected.ServiceDest[0].Index = 0
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

func (s *TypesTestSuite) Test_GetServiceFromProvider_AddsTasksWhenSessionTypeIsNotEmpty() {
	expected := s.getExpectedService()
	expected.SessionType = "sticky-server"
	expected.Tasks = []string{"1.2.3.4", "4.3.2.1"}
	serviceMap := s.getServiceMap(expected, ".1")
	provider := mapParameterProvider{&serviceMap}
	actualHost := ""
	lookupHostOrig := lookupHost
	defer func() { lookupHost = lookupHostOrig }()
	lookupHost = func(host string) (addrs []string, err error) {
		actualHost = host
		return expected.Tasks, nil
	}

	actual := GetServiceFromProvider(&provider)

	s.Equal("tasks."+expected.ServiceName, actualHost)
	s.Equal(expected, *actual)
}

func (s *TypesTestSuite) Test_GetServiceFromProvider_MovesServiceDomainToIndexedEntries_WhenPortIsEmpty() {
	expected := Service{
		ServiceDest: []ServiceDest{{
			AllowedMethods: []string{},
			DeniedMethods:  []string{},
			ServiceDomain:  []string{"domain1", "domain2"},
			ServiceHeader:  map[string]string{},
			ServicePath:    []string{"/"},
			Port:           "1234",
			ReqMode:        "reqMode",
			Index:          1,
		}},
		ServiceName: "serviceName",
	}
	serviceMap := map[string]string{
		"serviceDomain": strings.Join(expected.ServiceDest[0].ServiceDomain, ","),
		"serviceName":   expected.ServiceName,
		"port.1":        expected.ServiceDest[0].Port,
		"reqMode.1":     expected.ServiceDest[0].ReqMode,
		"servicePath.1": strings.Join(expected.ServiceDest[0].ServicePath, ","),
	}
	provider := mapParameterProvider{&serviceMap}
	actual := GetServiceFromProvider(&provider)
	s.Equal(expected, *actual)
}

func (s *TypesTestSuite) Test_GetServiceFromProvider_MovesHttpsOnlyToIndexedEntries_WhenEmpty() {
	expected := Service{
		ServiceDest: []ServiceDest{{
			AllowedMethods: []string{},
			DeniedMethods:  []string{},
			HttpsOnly:      true,
			ServiceDomain:  []string{},
			ServiceHeader:  map[string]string{},
			ServicePath:    []string{"/"},
			Port:           "1234",
			ReqMode:        "reqMode",
			Index:          1,
		}},
		ServiceName: "serviceName",
	}
	serviceMap := map[string]string{
		//		"serviceDomain": strings.Join(expected.ServiceDest[0].ServiceDomain, ","),
		"httpsOnly":     strconv.FormatBool(expected.ServiceDest[0].HttpsOnly),
		"serviceName":   expected.ServiceName,
		"port.1":        expected.ServiceDest[0].Port,
		"reqMode.1":     expected.ServiceDest[0].ReqMode,
		"servicePath.1": strings.Join(expected.ServiceDest[0].ServicePath, ","),
	}
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
	header := ""
	for key, value := range expected.ServiceDest[0].ServiceHeader {
		header += key + ":" + value + ","
	}
	header = strings.TrimRight(header, ",")
	return map[string]string{
		"aclName":               expected.AclName,
		"addReqHeader":          strings.Join(expected.AddReqHeader, ","),
		"addResHeader":          strings.Join(expected.AddResHeader, ","),
		"backendExtra":          expected.BackendExtra,
		"delReqHeader":          strings.Join(expected.DelReqHeader, ","),
		"delResHeader":          strings.Join(expected.DelResHeader, ","),
		"distribute":            strconv.FormatBool(expected.Distribute),
		"httpsPort":             strconv.Itoa(expected.HttpsPort),
		"isDefaultBackend":      strconv.FormatBool(expected.IsDefaultBackend),
		"outboundHostname":      expected.OutboundHostname,
		"pathType":              expected.PathType,
		"redirectWhenHttpProto": strconv.FormatBool(expected.RedirectWhenHttpProto),
		"reqPathReplace":        expected.ReqPathReplace,
		"reqPathSearch":         expected.ReqPathSearch,
		"serviceCert":           expected.ServiceCert,
		"serviceDomainAlgo":     expected.ServiceDomainAlgo,
		"serviceName":           expected.ServiceName,
		"sessionType":           expected.SessionType,
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
		"allowedMethods" + indexSuffix:      strings.Join(expected.ServiceDest[0].AllowedMethods, ","),
		"deniedMethods" + indexSuffix:       strings.Join(expected.ServiceDest[0].DeniedMethods, ","),
		"denyHttp" + indexSuffix:            strconv.FormatBool(expected.ServiceDest[0].DenyHttp),
		"httpsOnly" + indexSuffix:           strconv.FormatBool(expected.ServiceDest[0].HttpsOnly),
		"ignoreAuthorization" + indexSuffix: strconv.FormatBool(expected.ServiceDest[0].IgnoreAuthorization),
		"port" + indexSuffix:                expected.ServiceDest[0].Port,
		"reqMode" + indexSuffix:             expected.ServiceDest[0].ReqMode,
		"serviceDomain" + indexSuffix:       strings.Join(expected.ServiceDest[0].ServiceDomain, ","),
		"serviceHeader" + indexSuffix:       header,
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
		HttpsPort:             1234,
		IsDefaultBackend:      true,
		OutboundHostname:      "outboundHostname",
		PathType:              "pathType",
		RedirectWhenHttpProto: true,
		ReqPathReplace:        "reqPathReplace",
		ReqPathSearch:         "reqPathSearch",
		ServiceCert:           "serviceCert",
		ServiceDomainAlgo:     "hdr_dom",
		ServiceDest: []ServiceDest{{
			AllowedMethods:      []string{"GET", "DELETE"},
			DeniedMethods:       []string{"PUT", "POST"},
			DenyHttp:            true,
			HttpsOnly:           true,
			IgnoreAuthorization: true,
			ServiceDomain:       []string{"domain1", "domain2"},
			ServiceHeader:       map[string]string{"X-Version": "3", "name": "Viktor"},
			ServicePath:         []string{"/"},
			Port:                "1234",
			ReqMode:             "reqMode",
			UserAgent:           UserAgent{Value: []string{"agent-1", "agent-2/replace-with_"}, AclName: "agent_1_agent_2_replace_with_"},
			VerifyClientSsl:     true,
			Index:               1,
		}},
		ServiceName:     "serviceName",
		SetReqHeader:    []string{"set-header-1", "set-header-2"},
		SetResHeader:    []string{"set-header-1", "set-header-2"},
		SslVerifyNone:   true,
		TemplateBePath:  "templateBePath",
		TemplateFePath:  "templateFePath",
		TimeoutServer:   "timeoutServer",
		TimeoutTunnel:   "timeoutTunnel",
		XForwardedProto: true,
		Users: []User{
			{Username: "user1", Password: "pass1", PassEncrypted: true},
			{Username: "user2", Password: "pass2", PassEncrypted: true},
		},
	}
}
