package proxy

import (
	"github.com/stretchr/testify/suite"
	"testing"
	"strings"
	"strconv"
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

	users := ExtractUsersFromString("sn", "u:p", false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "p", Username: "u"},
	})

	users = ExtractUsersFromString("sn", "u:p", true, false)
	s.Equal(users, []*User{
		{PassEncrypted: true, Password: "p", Username: "u"},
	})

	users = ExtractUsersFromString("sn", "u:p:2", true, false)
	s.Equal(users, []*User{
		{PassEncrypted: true, Password: "p:2", Username: "u"},
	})

	users = ExtractUsersFromString("sn", "u", false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "", Username: "u"},
	})

	users = ExtractUsersFromString("sn", "u:p,ww", false, true)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "p", Username: "u"},
	})

	users = ExtractUsersFromString("sn", "u:p,ww:,:asd", false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "p", Username: "u"},
	})

	users = ExtractUsersFromString("sn", "u   ,    uu     ", false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "", Username: "u"},
		{PassEncrypted: false, Password: "", Username: "uu"},
	})

	users = ExtractUsersFromString("sn", "", false, false)
	s.Equal(users, []*User{})

	users = ExtractUsersFromString("sn", `u   ,
	 uu     `, false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "", Username: "u"},
		{PassEncrypted: false, Password: "", Username: "uu"},
	})
	users = ExtractUsersFromString("sn", `u
uu`, false, false)
	s.Equal(users, []*User{
		{PassEncrypted: false, Password: "", Username: "u"},
		{PassEncrypted: false, Password: "", Username: "uu"},
	})

	users = ExtractUsersFromString("sn",
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
	expected := Service{
		AclName:               "aclName",
		AddHeader:             []string{"add-header-1", "add-header-2"},
		ConsulTemplateFePath:  "consulTemplateFePath",
		ConsulTemplateBePath:  "consulTemplateBePath",
		Distribute:            true,
		HttpsOnly:             true,
		HttpsPort:             1234,
		OutboundHostname:      "outboundHostname",
		PathType:              "pathType",
		RedirectWhenHttpProto: true,
		ReqMode:               "reqMode",
		ReqPathReplace:        "reqPathReplace",
		ReqPathSearch:         "reqPathSearch",
		ServiceCert:           "serviceCert",
		ServiceColor:          "serviceColor",
		ServiceDest:           []ServiceDest{{ServicePath: []string{}}},
		ServiceDomain:         []string{"domain1", "domain2"},
		ServiceDomainMatchAll: true,
		ServiceName:           "serviceName",
		SetHeader:             []string{"set-header-1", "set-header-2"},
		SkipCheck:             true,
		SslVerifyNone:         true,
		TemplateBePath:        "templateBePath",
		TemplateFePath:        "templateFePath",
		TimeoutServer:         "timeoutServer",
		TimeoutTunnel:         "timeoutTunnel",
		XForwardedProto:       true,
		Users: []User{{Username: "user1", Password: "pass1", PassEncrypted: true, },
			      {Username: "user2", Password: "pass2", PassEncrypted: true, }},
	}
	serviceMap := map[string]string{


		"serviceName":           expected.ServiceName,
		"users":                 "user1:pass1,user2:pass2",
		"usersPassEncrypted":    "true",
		"aclName":               expected.AclName,
		"serviceColor":          expected.ServiceColor,
		"serviceCert":           expected.ServiceCert,
		"outboundHostname":      expected.OutboundHostname,
		"consulTemplateFePath":  expected.ConsulTemplateFePath,
		"consulTemplateBePath":  expected.ConsulTemplateBePath,
		"pathType":              expected.PathType,
		"reqPathSearch":         expected.ReqPathSearch,
		"reqPathReplace":        expected.ReqPathReplace,
		"templateFePath":        expected.TemplateFePath,
		"templateBePath":        expected.TemplateBePath,
		"timeoutServer":         expected.TimeoutServer,
		"timeoutTunnel":         expected.TimeoutTunnel,
		"reqMode":               expected.ReqMode,
		"httpsOnly":             strconv.FormatBool(expected.HttpsOnly),
		"xForwardedProto":       strconv.FormatBool(expected.XForwardedProto),
		"redirectWhenHttpProto": strconv.FormatBool(expected.RedirectWhenHttpProto),
		"httpsPort":             strconv.Itoa(expected.HttpsPort),
		"serviceDomain":         strings.Join(expected.ServiceDomain, ","),
		"skipCheck":             strconv.FormatBool(expected.SkipCheck),
		"distribute":            strconv.FormatBool(expected.Distribute),
		"sslVerifyNone":         strconv.FormatBool(expected.SslVerifyNone),
		"serviceDomainMatchAll": strconv.FormatBool(expected.ServiceDomainMatchAll),
		"addHeader":             strings.Join(expected.AddHeader, ","),
		"setHeader":             strings.Join(expected.SetHeader, ","),
	}
	actual := GetServiceFromMap(&serviceMap)
	s.Equal(expected, *actual)
}


// Suite

func TestRunUnitTestSuite(t *testing.T) {
	suite.Run(t, new(TypesTestSuite))
}

