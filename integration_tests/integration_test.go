package integration_test

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os/exec"
	"strings"
	"testing"
)

type IntegrationTestSuite struct {
	suite.Suite
	hostIp      string
	serviceName string
}

func (s *IntegrationTestSuite) SetupTest() {
	s.serviceName = "test-service"
}

// Integration

func (s IntegrationTestSuite) Test_Reconfigure_MultipleInstances() {
	s.reconfigure("", "", "", "/v1/test")

	s.verifyReconfigure(1)
}

//func (s IntegrationTestSuite) Test_Reconfigure_MultipleServices() {
//	s.reconfigure("", "", "", "/v1/test")
//	s.serviceName = "test-other-service"
//	s.reconfigure("", "", "", "/v2/test")
//
//	s.verifyReconfigure(1)
//	s.verifyReconfigure(2)
//}

func (s IntegrationTestSuite) Test_Reconfigure_PathReg() {
	s.reconfigure("path_reg", "", "", "/.*/test")

	s.verifyReconfigure(1)
}

func (s IntegrationTestSuite) Test_Reconfigure_MultiplePaths() {
	s.reconfigure("", "", "/v1/test", "", "/v2/test")

	s.verifyReconfigure(2)
}

func (s IntegrationTestSuite) Test_GlobalAuth() {
	s.reconfigure("", "", "", "/v1/test")

	// Returns status 401 if no auth is provided

	testAddr := "http://staging-dep/v1/test"
	log.Printf(">> Sending verify request to %s", testAddr)
	client := &http.Client{}
	request, _ := http.NewRequest("GET", testAddr, nil)
	resp, err := client.Do(request)

	s.NoError(err)
	s.Equal(401, resp.StatusCode)

	// Returns status 200 if auth is provided

	request.SetBasicAuth("user1", "pass1")
	resp, err = client.Do(request)

	s.NoError(err)
	s.Equal(200, resp.StatusCode)
}

func (s IntegrationTestSuite) Test_Reconfigure_Auth() {
	address := fmt.Sprintf(
		"http://staging-dep:8080/v1/docker-flow-proxy/reconfigure?serviceName=%s&servicePath=%s&users=%s",
		s.serviceName,
		"/v1/test",
		"serv-user-1:serv-pass-1",
	)
	log.Printf(">> Sending reconfigure request to %s", address)
	_, err := http.Get(address)
	s.NoError(err)

	// Returns status 401 if no auth is provided

	testAddr := "http://staging-dep/v1/test"
	log.Printf(">> Sending verify request to %s", testAddr)
	client := &http.Client{}
	request, _ := http.NewRequest("GET", testAddr, nil)
	resp, err := client.Do(request)

	s.NoError(err)
	s.Equal(401, resp.StatusCode)

	// Returns status 200 if auth is provided

	request.SetBasicAuth("serv-user-1", "serv-pass-1")
	resp, err = client.Do(request)

	s.NoError(err)
	s.Equal(200, resp.StatusCode)
}

//func (s IntegrationTestSuite) Test_Reconfigure_MultipleDestinations() {
//	address := fmt.Sprintf(
//		"http://staging-dep:8080/v1/docker-flow-proxy/reconfigure?serviceName=%s&servicePath=%s&port=1111&servicePath.1=%s&port.1=2222",
//		s.serviceName,
//		"/v1/test",
//		"/v1/test",
//	)
//	log.Printf(">> Sending reconfigure request to %s", address)
//	_, err := http.Get(address)
//	s.NoError(err)
//
//	// Returns status 200
//
//	testAddr := "http://staging-dep/v1/test"
//	log.Printf(">> Sending verify request to %s", testAddr)
//	client := &http.Client{}
//	request, _ := http.NewRequest("GET", testAddr, nil)
//	request.SetBasicAuth("serv-user-1", "serv-pass-1")
//	resp, err := client.Do(request)
//
//	s.NoError(err)
//	s.Equal(200, resp.StatusCode)
//	println(s.getConf())
//	s.True(false)
//}

// Deprecated (dec. 2016)
func (s IntegrationTestSuite) Test_Reconfigure_ReqRep() {
	urlObj, _ := url.Parse(
		"http://staging-dep:8080/v1/docker-flow-proxy/reconfigure",
	)
	parameters := url.Values{}
	parameters.Add("serviceName", s.serviceName)
	parameters.Add("servicePath", "/v99/test")
	parameters.Add("reqRepSearch", `^([^\ ]*)\ /v99/(.*)`)
	parameters.Add("reqRepReplace", `\1\ /v1/\2`)
	urlObj.RawQuery = parameters.Encode()
	log.Printf(">> Sending reconfigure request to %s", urlObj.String())
	_, err := http.Get(urlObj.String())
	s.NoError(err)

	// Returns status 200

	testAddr := "http://staging-dep/v99/test"
	log.Printf(">> Sending verify request to %s", testAddr)
	client := &http.Client{}
	request, _ := http.NewRequest("GET", testAddr, nil)
	request.SetBasicAuth("user1", "pass1")
	resp, err := client.Do(request)

	s.NoError(err)
	s.Equal(200, resp.StatusCode)
}

func (s IntegrationTestSuite) Test_Reconfigure_ReqPath() {
	urlObj, _ := url.Parse(
		"http://staging-dep:8080/v1/docker-flow-proxy/reconfigure",
	)
	parameters := url.Values{}
	parameters.Add("serviceName", s.serviceName)
	parameters.Add("servicePath", "/v99/test")
	parameters.Add("reqPathSearch", `/v99/`)
	parameters.Add("reqPathReplace", `/v1/`)
	urlObj.RawQuery = parameters.Encode()
	log.Printf(">> Sending reconfigure request to %s", urlObj.String())
	_, err := http.Get(urlObj.String())
	s.NoError(err)

	// Returns status 200

	testAddr := "http://staging-dep/v99/test"
	log.Printf(">> Sending verify request to %s", testAddr)
	client := &http.Client{}
	request, _ := http.NewRequest("GET", testAddr, nil)
	request.SetBasicAuth("user1", "pass1")
	resp, err := client.Do(request)

	s.NoError(err)
	s.Equal(200, resp.StatusCode)
}

func (s IntegrationTestSuite) Test_Stats_Auth() {
	// Returns status 401 if no auth is provided

	testAddr := "http://staging-dep/admin?stats"
	log.Printf(">> Sending verify request to %s", testAddr)
	client := &http.Client{}
	request, _ := http.NewRequest("GET", testAddr, nil)
	resp, err := client.Do(request)

	s.NoError(err)
	s.Equal(401, resp.StatusCode)

	// Returns status 200 if auth is provided

	request.SetBasicAuth("stats", "pass")
	resp, err = client.Do(request)

	s.NoError(err)
	s.Equal(200, resp.StatusCode)
}

func (s IntegrationTestSuite) Test_Remove() {
	aclName := "my-acl"
	address := fmt.Sprintf(
		"http://staging-dep:8080/v1/docker-flow-proxy/reconfigure?serviceName=%s&servicePath=%s&aclName=%s",
		s.serviceName,
		"/v1/test",
		aclName,
	)

	// Remove by serviceName

	log.Printf(">> Sending reconfigure request to %s", address)
	_, err := http.Get(address)
	s.NoError(err)
	s.verifyReconfigure(1)

	url := "http://staging-dep:8080/v1/docker-flow-proxy/remove?serviceName=test-service"
	log.Printf(">> Sending remove request to %s", url)
	_, err = http.Get(url)

	s.NoError(err)
	url = "http://staging-dep/v1/test"
	resp, err := http.Get(url)
	s.NoError(err)
	s.NotEqual(200, resp.StatusCode)
}

func (s IntegrationTestSuite) Test_PutToConsul() {
	s.reconfigure("", "", "", "/v1/test")

	url := fmt.Sprintf(
		"http://consul:8500/v1/kv/proxy-test-instance/%s/path?raw",
		s.serviceName,
	)
	resp, _ := http.Get(url)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	s.Equal("/v1/test", string(body))
}

func (s IntegrationTestSuite) Test_Reconfigure_ConsulTemplatePath() {
	http.Get(
		"http://staging-dep:8080/v1/docker-flow-proxy/remove?serviceName=test-service",
	)
	s.reconfigure("", "/test_configs/tmpl/my-service-fe.tmpl", "/test_configs/tmpl/my-service-be.tmpl", "")

	s.verifyReconfigure(1)
}

func (s IntegrationTestSuite) Test_Config() {
	resp, _ := http.Get("http://staging-dep:8080/v1/docker-flow-proxy/config")
	cmdString := "docker cp dockerflowproxy_staging-dep_1:/cfg/haproxy.cfg /tmp/"
	exec.Command("/bin/sh", "-c", cmdString).Output()

	expected, _ := ioutil.ReadFile("/tmp/haproxy.cfg")

	body, _ := ioutil.ReadAll(resp.Body)

	s.Equal(string(expected[:]), string(body))
}

func (s IntegrationTestSuite) Test_Certs() {
	// Body is mandatory
	url := "http://staging-dep:8080/v1/docker-flow-proxy/cert?certName=xip.io.pem"
	req, _ := http.NewRequest("PUT", url, nil)
	client := &http.Client{}

	resp, _ := client.Do(req)

	s.Equal(400, resp.StatusCode)

	// certName is mandatory
	url = "http://staging-dep:8080/v1/docker-flow-proxy/cert"
	req, _ = http.NewRequest("PUT", url, strings.NewReader("THIS IS A CERTIFICATE"))
	client = &http.Client{}

	resp, _ = client.Do(req)

	s.Equal(400, resp.StatusCode)

	// Stores certs
	url = "http://staging-dep:8080/v1/docker-flow-proxy/cert?certName=xip.io.pem"
	certContent, _ := ioutil.ReadFile("../certs/xip.io.pem")
	req, _ = http.NewRequest("PUT", url, strings.NewReader(string(certContent)))
	client = &http.Client{}

	resp, _ = client.Do(req)

	s.Equal(200, resp.StatusCode)

	// Can retrieve certs
	url = "http://staging-dep:8080/v1/docker-flow-proxy/certs"
	req, _ = http.NewRequest("GET", url, nil)
	client = &http.Client{}

	resp, _ = client.Do(req)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	s.Equal(200, resp.StatusCode)
	s.Contains(strings.Replace(string(body), "\\n", "\n", -1), string(certContent))
}

// Util

func (s IntegrationTestSuite) verifyReconfigure(version int) {
	address := fmt.Sprintf("http://staging-dep/v%d/test", version)
	log.Printf(">> Sending verify request to %s", address)
	client := &http.Client{}
	request, _ := http.NewRequest("GET", address, nil)
	request.SetBasicAuth("user1", "pass1")
	resp, err := client.Do(request)

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getConf())
}

func (s IntegrationTestSuite) reconfigure(pathType, consulTemplateFePath, consulTemplateBePath string, paths ...string) {
	var address string
	if len(consulTemplateFePath) > 0 {
		address = fmt.Sprintf(
			"http://staging-dep:8080/v1/docker-flow-proxy/reconfigure?serviceName=%s&consulTemplateFePath=%s&consulTemplateBePath=%s",
			s.serviceName,
			consulTemplateFePath,
			consulTemplateBePath,
		)
	} else {
		address = fmt.Sprintf(
			"http://staging-dep:8080/v1/docker-flow-proxy/reconfigure?serviceName=%s&servicePath=%s&pathType=%s",
			s.serviceName,
			strings.Join(paths, ","),
			pathType,
		)
	}
	log.Printf(">> Sending reconfigure request to %s", address)
	resp, err := http.Get(address)
	if err != nil {
		s.Fail(err.Error())
	} else {
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		s.Equal(200, resp.StatusCode, string(body))
	}
}

func (s IntegrationTestSuite) getConf() string {
	configAddr := "http://staging-dep:8080/v1/docker-flow-proxy/config"
	resp, _ := http.Get(configAddr)
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body)
}

// Suite

func TestGeneralIntegrationTestSuite(t *testing.T) {
	s := new(IntegrationTestSuite)
	suite.Run(t, s)
}
