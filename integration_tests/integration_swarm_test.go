package integration_test

import (
	"fmt"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// Setup

type IntegrationSwarmTestSuite struct {
	suite.Suite
	hostIP        string
	dockerHubUser string
}

func (s *IntegrationSwarmTestSuite) SetupTest() {
}

func TestGeneralIntegrationSwarmTestSuite(t *testing.T) {
	s := new(IntegrationSwarmTestSuite)
	s.hostIP = os.Getenv("HOST_IP")
	s.dockerHubUser = os.Getenv("DOCKER_HUB_USER")

	s.removeServices("go-demo-api", "go-demo-db", "proxy", "proxy-env", "redis")
	//	exec.Command("/bin/sh", "-c", "docker system prune -f").Output()

	cmd := fmt.Sprintf("docker swarm init --advertise-addr %s", s.hostIP)
	exec.Command("/bin/sh", "-c", cmd).Output()

	exec.Command("/bin/sh", "-c", "docker network create --driver overlay proxy").Output()

	exec.Command("/bin/sh", "-c", "docker network create --driver overlay go-demo").Output()

	cmd = fmt.Sprintf(
		`docker service create --name proxy \
    -p 80:80 \
    -p 443:443 \
    -p 8080:8080 \
    -p 6379:6379 \
    --network proxy \
    -e DEFAULT_PORTS=80,443:ssl \
    -e MODE=swarm \
    -e STATS_USER=none \
    -e STATS_PASS=none \
    -e TIMEOUT_CONNECT=10 \
    -e TIMEOUT_HTTP_REQUEST=10 \
    -e TERMINATE_ON_RELOAD=true \
    %s/docker-flow-proxy:beta`,
		s.dockerHubUser)
	out, err := s.createService(cmd)
	if err != nil {
		log.Println("COMMAND:", cmd)
		log.Println("OUT:", string(out))
		log.Fatal(err)
	}

	s.createService(`docker service create --name go-demo-db \
    --network go-demo \
    mongo`)

	s.createGoDemoService()

	s.waitForContainers(1, "proxy")

	suite.Run(t, s)

	s.removeServices("go-demo-api", "go-demo-db", "proxy", "proxy-env", "redis")
}

// Tests

func (s IntegrationSwarmTestSuite) Test_Reconfigure() {
	s.reconfigureGoDemo("")

	resp, err := s.sendHelloRequest()

	s.NoError(err)
	if resp != nil {
		s.Equal(200, resp.StatusCode, s.getProxyConf(""))
	}
}

func (s IntegrationSwarmTestSuite) Test_Domain() {
	s.reconfigureGoDemo("&serviceDomain=my-domain.com")

	client := new(http.Client)
	url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)
	req, err := http.NewRequest("GET", url, nil)
	s.NoError(err)
	req.Host = "my-domain.com"
	resp, err := client.Do(req)

	s.NoError(err)
	if resp != nil {
		s.Equal(200, resp.StatusCode, s.getProxyConf(""))
	}
}

func (s IntegrationSwarmTestSuite) Test_Config() {
	s.reconfigureGoDemo("")

	actual := s.getProxyConf("")

	// Cannot validate that the config is correct but only that some text is returned
	s.Contains(actual, "pidfile /var/run/haproxy.pid")

	actual = s.getProxyConf("json")

	// Cannot validate that the config is correct but only that some json is returned
	s.Contains(actual, `{"go-demo-api":`)
}

func (s IntegrationSwarmTestSuite) Test_Metrics() {
	defer func() {
		exec.Command("/bin/sh", "-c", "docker service scale proxy=1").Output()
		s.waitForContainers(1, "proxy")
	}()
	addr := fmt.Sprintf(
		"http://%s:8080/v1/docker-flow-proxy/metrics",
		s.hostIP,
	)
	out, err := exec.Command("/bin/sh", "-c", "docker service scale proxy=3").CombinedOutput()
	if err != nil {
		s.Fail("%s\n%s", err.Error(), string(out))
	} else {
		s.waitForContainers(3, "proxy")
	}

	resp, err := http.Get(addr)
	s.NoError(err)

	body, _ := ioutil.ReadAll(resp.Body)

	// Cannot validate that the metrics are correct but only that some text is returned
	s.Contains(string(body[:]), "services,FRONTEND")

	resp, err = http.Get(addr + "?distribute=true")
	s.NoError(err)

	body, _ = ioutil.ReadAll(resp.Body)

	// Cannot validate that the metrics are correct but only that some text is returned
	s.Contains(string(body[:]), "services,FRONTEND")
}

func (s IntegrationSwarmTestSuite) Test_Compression() {
	defer func() {
		exec.Command("/bin/sh", "-c", `docker service update --env-rm "COMPRESSION_ALGO" proxy`).Output()
		s.waitForContainers(1, "proxy")
	}()
	_, err := exec.Command(
		"/bin/sh",
		"-c",
		`docker service update --env-add "COMPRESSION_ALGO=gzip" --env-add "COMPRESSION_TYPE=text/css text/html text/javascript application/javascript text/plain text/xml application/json" proxy`,
	).Output()
	s.NoError(err)
	s.waitForContainers(1, "proxy")
	s.reconfigureGoDemo("")

	client := new(http.Client)
	url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("Accept-Encoding", "gzip")
	resp, err := client.Do(req)

	s.NoError(err)
	if resp != nil {
		s.Equal(200, resp.StatusCode, s.getProxyConf(""))
		s.Contains(resp.Header["Content-Encoding"], "gzip", s.getProxyConf(""))
	}
}

// The attempt to remove zombie processes failed
//func (s IntegrationSwarmTestSuite) Test_ZombieProcesses() {
//	for i:=0; i < 30; i++ {
//		s.reconfigureGoDemo("")
//	}
//	out, err := exec.Command(
//		"/bin/sh",
//		"-c",
//		"docker container ls -q -f \"label=com.docker.swarm.service.name=proxy\" | tail -n 1",
//	).CombinedOutput()
//	s.NoError(err)
//	out, err = exec.Command(
//		"/bin/sh",
//		"-c",
//		"docker container exec -t " + strings.Trim(string(out), "\n") + " ps aux | grep haproxy",
//	).CombinedOutput()
//	time.Sleep(10 * time.Second)
//
//	s.NoError(err)
//	// There should be only one processes plus extra line at the end of the output
//	s.Len(strings.Split(string(out), "\n"), 2)
//}

func (s IntegrationSwarmTestSuite) Test_HeaderAcls() {
	client := new(http.Client)
	url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)

	// Responds with 200 since the header matches

	s.reconfigureGoDemo("&serviceHeader=X-Version:3")
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Add("X-Version", "3")
	resp, err := client.Do(req)

	s.NoError(err)
	if resp != nil {
		s.Equal(200, resp.StatusCode, s.getProxyConf(""))
	}

	// Responds with 200 since both headers match

	s.reconfigureGoDemo("&serviceHeader=X-Version:3,name:Viktor")
	req, err = http.NewRequest("GET", url, nil)
	req.Header.Add("X-Version", "3")
	req.Header.Add("name", "Viktor")
	resp, err = client.Do(req)

	s.NoError(err)
	if resp != nil {
		s.Equal(200, resp.StatusCode, s.getProxyConf(""))
	}

	// Does NOT respond with 200 since both headers do NOT match

	s.reconfigureGoDemo("&serviceHeader=X-Version:3,name:Viktor")
	req, err = http.NewRequest("GET", url, nil)
	req.Header.Add("X-Version", "3")
	resp, err = client.Do(req)

	s.NoError(err)
	if resp != nil {
		s.NotEqual(200, resp.StatusCode, s.getProxyConf(""))
	}

	// Does NOT respond with 200 since headers do not match

	resp, err = client.Get(url)

	s.NoError(err)
	s.NotEqual(200, resp.StatusCode, s.getProxyConf(""))
}

func (s IntegrationSwarmTestSuite) Test_AddHeaders() {
	s.reconfigureGoDemo("&addResHeader=my-res-header%20my-res-value")

	resp, err := http.Get(fmt.Sprintf("http://%s/demo/hello", s.hostIP))

	s.NoError(err)
	if resp != nil {
		s.Equal(200, resp.StatusCode, s.getProxyConf(""))
		s.Contains(resp.Header["My-Res-Header"], "my-res-value", s.getProxyConf(""))
	}
}

func (s IntegrationSwarmTestSuite) Test_UserAgent() {
	defer func() { s.reconfigureGoDemo("") }()
	s.reconfigureGoDemo("&userAgent=amiga,amstrad")
	url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)
	client := new(http.Client)

	// With the amstrad agent

	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Add("User-Agent", "amstrad")
	resp, err := client.Do(req)

	s.NoError(err)
	if resp != nil {
		s.Equal(200, resp.StatusCode, s.getProxyConf(""))
	}

	// Without the matching agent

	resp, err = http.Get(url)

	s.NoError(err)
	if resp != nil {
		s.NotEqual(200, resp.StatusCode, s.getProxyConf(""))
	}

	// With the amiga agent

	req, _ = http.NewRequest("GET", url, nil)
	req.Header.Add("User-Agent", "amiga")
	resp, err = client.Do(req)

	s.NoError(err)
	if resp != nil {
		s.Equal(200, resp.StatusCode, s.getProxyConf(""))
	}
}

func (s IntegrationSwarmTestSuite) Test_UserAgent_LastIndexCatchesAllNonMatchedRequests() {
	defer func() { s.reconfigureGoDemo("") }()
	service1 := "&servicePath.1=/demo&port.1=1111&userAgent.1=amiga"
	service2 := "&servicePath.2=/demo&port.2=2222&userAgent.2=amstrad"
	service3 := "&servicePath.3=/demo&port.3=8080"
	params := "serviceName=go-demo-api" + service1 + service2 + service3
	s.reconfigureService(params)
	url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)

	// Not testing ports 1111 and 2222 since go-demo-api is not listening on those ports

	// Without the matching agent

	resp, err := http.Get(url)

	s.NoError(err)
	if resp != nil {
		s.Equal(200, resp.StatusCode, s.getProxyConf(""))
	}
}

func (s IntegrationSwarmTestSuite) Test_VerifyClientSsl_DeniesRequest() {
	defer func() { s.reconfigureGoDemo("") }()
	s.reconfigureGoDemo("&verifyClientSsl=true")
	url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)

	// Returns 403 Forbidden

	resp, err := http.Get(url)

	s.NoError(err)
	if resp != nil {
		s.Equal(403, resp.StatusCode, s.getProxyConf(""))
	}
}

func (s IntegrationSwarmTestSuite) Test_Stats() {
	url := fmt.Sprintf("http://%s/admin?stats", s.hostIP)

	resp, err := http.Get(url)

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf(""))
}

func (s IntegrationSwarmTestSuite) Test_Remove() {
	s.reconfigureGoDemo("")

	url := fmt.Sprintf(
		"http://%s:8080/v1/docker-flow-proxy/remove?serviceName=go-demo-api",
		s.hostIP,
	)
	http.Get(url)

	resp, err := s.sendHelloRequest()

	s.NoError(err)
	s.Equal(503, resp.StatusCode, s.getProxyConf(""))
}

func (s IntegrationSwarmTestSuite) Test_Scale() {
	defer func() {
		exec.Command("/bin/sh", "-c", "docker service scale proxy=1").Output()
		s.waitForContainers(1, "proxy")
	}()
	out, err := exec.Command("/bin/sh", "-c", "docker service scale proxy=3").CombinedOutput()
	if err != nil {
		s.Fail("%s\n%s", err.Error(), string(out))
	} else {
		s.waitForContainers(3, "proxy")

		s.reconfigureGoDemo("&distribute=true")

		ok := 0
		for i := 0; i < 10; i++ {
			resp, err := s.sendHelloRequest()
			if resp.StatusCode == 200 {
				ok++
			}

			s.NoError(err)
		}
		// For some unexplainable reason one of the go-demo-api requests will fail.
		s.True(ok >= 7, "Expected at least 7 requests with the response code 200 but got %d", ok)
	}

}

func (s IntegrationSwarmTestSuite) Test_RewritePaths() {

	// With reqPathReplace

	url := fmt.Sprintf(
		`http://%s:8080/v1/docker-flow-proxy/reconfigure?serviceName=go-demo-api&servicePath=/something&port=8080&reqPathSearch=xxx|/something/&reqPathReplace=/demo/`,
		s.hostIP,
	)
	resp, err := http.Get(url)
	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf(""))

	url = fmt.Sprintf("http://%s/something/hello", s.hostIP)
	resp, err = http.Get(url)

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf(""))

	// With empty reqPathReplace

	url = fmt.Sprintf(
		"http://%s:8080/v1/docker-flow-proxy/reconfigure?serviceName=go-demo-api&servicePath=/something&port=8080&reqPathSearch=/something&reqPathReplace=",
		s.hostIP,
	)
	resp, err = http.Get(url)
	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf(""))

	url = fmt.Sprintf("http://%s/something/demo/hello", s.hostIP)
	resp, err = http.Get(url)

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf(""))

	// Without reqPathReplace

	url = fmt.Sprintf(
		"http://%s:8080/v1/docker-flow-proxy/reconfigure?serviceName=go-demo-api&servicePath=/something&port=8080&reqPathSearch=/something",
		s.hostIP,
	)
	_, err = http.Get(url)
	s.NoError(err)

	url = fmt.Sprintf("http://%s/something/demo/hello", s.hostIP)
	resp, err = http.Get(url)
	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf(""))

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf(""))
}

func (s IntegrationSwarmTestSuite) Test_GlobalAuthentication() {
	defer func() {
		exec.Command("/bin/sh", "-c", `docker service update --env-rm "USERS" proxy`).Output()
		s.waitForContainers(1, "proxy")
	}()
	_, err := exec.Command("/bin/sh", "-c", `docker service update --env-add "USERS=my-user:my-pass" proxy`).Output()
	s.NoError(err)
	s.waitForContainers(1, "proxy")

	s.reconfigureGoDemo("")

	resp, err := s.sendHelloRequest()

	s.NoError(err)
	statusCode := 0
	if err == nil {
		statusCode = resp.StatusCode
	}
	s.Equal(401, statusCode, s.getProxyConf(""))

	url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)
	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("my-user", "my-pass")
	client := &http.Client{}
	resp, err = client.Do(req)

	s.NoError(err)
	if err == nil {
		statusCode = resp.StatusCode
	}
	s.Equal(200, statusCode, s.getProxyConf(""))
}

func (s IntegrationSwarmTestSuite) Test_GlobalAuthenticationWithEncryption() {
	defer func() {
		exec.Command("/bin/sh", "-c", `docker service update --env-rm USERS --env-rm USERS_PASS_ENCRYPTED proxy`).Output()
		s.waitForContainers(1, "proxy")
	}()
	_, err := exec.Command("/bin/sh", "-c", `docker service update --env-add "USERS_PASS_ENCRYPTED=true" --env-add "USERS=my-user:\$6\$AcrjVWOkQq1vWp\$t55F7Psm3Ujvp8lpqdAwrc5RxWORYBeDV6ji9KoO029ojooj4Pi.JVGwxdicB0Fuu.NSDyGaZt7skHIo3Nayq/" proxy`).Output()
	s.NoError(err)
	s.waitForContainers(1, "proxy")

	s.reconfigureGoDemo("")

	resp, err := s.sendHelloRequest()

	if err != nil {
		s.Fail(err.Error())
	} else {
		s.Equal(401, resp.StatusCode, s.getProxyConf(""))
		url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)
		req, err := http.NewRequest("GET", url, nil)
		req.SetBasicAuth("my-user", "my-pass")
		client := &http.Client{}
		resp, err = client.Do(req)

		s.NoError(err)
		s.Equal(200, resp.StatusCode, s.getProxyConf(""))
	}
}

func (s IntegrationSwarmTestSuite) Test_ServiceAuthentication() {
	defer func() {
		s.reconfigureGoDemo("")
	}()

	// Add authorization

	s.reconfigureGoDemo("&users=admin:password")

	// Proxy returns 401 when user/pass is NOT provided

	resp, err := s.sendHelloRequest()

	if err != nil {
		s.Fail(err.Error())
	} else {
		s.Equal(401, resp.StatusCode, s.getProxyConf(""))
	}

	// Proxy returns 200 when user/pass is provided

	url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)
	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("admin", "password")
	client := &http.Client{}
	resp, err = client.Do(req)

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf(""))

	// Add ignoreAuthorization

	params := fmt.Sprintf("serviceName=go-demo-api&servicePath.1=/demo&port.1=8080&users=admin:password&ignoreAuthorization.1=true")
	s.reconfigureService(params)

	// Proxy returns 200 when user/pass is NOT provided

	resp, err = s.sendHelloRequest()

	if err != nil {
		s.Fail(err.Error())
	} else {
		s.Equal(200, resp.StatusCode, s.getProxyConf(""))
	}
}

// TODO: Figure out what is missing inside a container
//func (s IntegrationSwarmTestSuite) Test_XTcp() {
//	defer func() {
//		s.removeServices("redis")
//		s.waitForContainers(0, "redis")
//	}()
//	cmdString := `docker service create --name redis \
//	--network proxy \
//	redis:3.2`
//	exec.Command("/bin/sh", "-c", cmdString).Output()
//	s.waitForContainers(1, "redis")
//	s.reconfigureRedis()
//
//	cmdString = fmt.Sprintf("ADDR=%s PORT=6379 /src/integration_tests/redis_check.sh", s.hostIP)
//	var stdout bytes.Buffer
//	var stderr bytes.Buffer
//	cmd := exec.Command("/bin/sh", "-c", cmdString)
//	cmd.Stdout = &stdout
//	cmd.Stderr = &stderr
//	err := cmd.Run()
//
//	s.NoError(
//		err,
//		"CONFIG\n%s\n\nOUT:\n%s\n\nERR:\n%s",
//		s.getProxyConf(""),
//		stdout.String(),
//		stderr.String(),
//	)
//}

// Cannot use `docker ps` on multi-node cluster
// TODO: Refactor
//func (s IntegrationSwarmTestSuite) Test_Reload() {
//	// Reconfigure
//	s.reconfigureGoDemo("")
//	resp, err := s.sendHelloRequest()
//	s.NoError(err)
//	s.Equal(200, resp.StatusCode, s.getProxyConf(""))
//
//	// Corrupt the config
//	out, _ := exec.Command("/bin/sh", "-c", "docker ps -q -f label=com.docker.swarm.service.name=proxy").Output()
//	id := strings.TrimRight(string(out), "\n")
//	cmd := fmt.Sprintf("docker cp /tmp/haproxy.cfg %s:/cfg/haproxy.cfg", id)
//	if f, err := os.Create("/tmp/haproxy.cfg"); err != nil {
//		s.Fail(err.Error())
//	} else {
//		f.Write([]byte("This config is corrupt"))
//	}
//	exec.Command("/bin/sh", "-c", cmd).Output()
//
//	// Reload with reconfigure
//	s.reloadService("?recreate=true")
//	config := s.getProxyConf("")
//	s.NotEqual("This config is corrupt", config)
//}

func (s IntegrationSwarmTestSuite) Test_ReconfigureFromEnvVars() {
	defer func() {
		s.removeServices("proxy-env")
		time.Sleep(1 * time.Second)
	}()
	cmd := fmt.Sprintf(
		`docker service create --name proxy-env \
    -p 8081:80 \
    --network proxy \
    -e MODE=swarm \
    -e DFP_SERVICE_1_SERVICE_NAME=go-demo-api \
    -e DFP_SERVICE_1_SERVICE_PATH=/demo \
    -e DFP_SERVICE_1_PORT=8080 \
    %s/docker-flow-proxy:beta`,
		s.dockerHubUser)
	_, err := s.createService(cmd)
	s.NoError(err)
	s.waitForContainers(1, "proxy-env")

	url := fmt.Sprintf("http://%s:8081/demo/hello", s.hostIP)
	resp, err := http.Get(url)

	s.NoError(err)
	if resp != nil {
		s.Equal(200, resp.StatusCode)
	} else {
		s.Fail("No response")
	}
}

func (s IntegrationSwarmTestSuite) Test_ReconfigureWithDefaultBackend() {
	params := "serviceName=go-demo-api&servicePath=/xxx&port=8080&isDefaultBackend=true"
	s.reconfigureService(params)

	resp, err := s.sendHelloRequest()

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf(""))
}

func (s IntegrationSwarmTestSuite) Test_Methods() {

	// Forbidden

	s.reconfigureGoDemo("&allowedMethods=DELETE")

	resp, err := s.sendHelloRequest()

	s.NoError(err)
	s.Equal(403, resp.StatusCode, s.getProxyConf(""))

	// Allowed

	s.reconfigureGoDemo("")

	resp, err = s.sendHelloRequest()

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf(""))

	// Forbidden

	s.reconfigureGoDemo("&deniedMethods=GET")

	resp, err = s.sendHelloRequest()

	s.NoError(err)
	s.Equal(403, resp.StatusCode, s.getProxyConf(""))
}

func (s IntegrationSwarmTestSuite) Test_DenyHttp() {
	s.reconfigureGoDemo("&denyHttp=true")

	resp, err := s.sendHelloRequest()

	if err != nil {
		s.Fail(err.Error())
	} else {
		s.Equal(403, resp.StatusCode, s.getProxyConf(""))
	}
}

// Util

func (s *IntegrationSwarmTestSuite) areContainersRunning(expected int, name string) bool {
	out, _ := exec.Command("/bin/sh", "-c", "docker service ps -f desired-state=Running "+name+" | grep -v Preparing").Output()
	lines := strings.Split(strings.Trim(string(out), "\n"), "\n")
	return len(lines) == (expected + 1)
}

func (s *IntegrationSwarmTestSuite) createService(command string) ([]byte, error) {
	return exec.Command("/bin/sh", "-c", command).CombinedOutput()
}

func (s *IntegrationSwarmTestSuite) removeServices(service ...string) {
	for _, s := range service {
		cmd := fmt.Sprintf("docker service rm %s", s)
		exec.Command("/bin/sh", "-c", cmd).Output()
	}
}

func (s *IntegrationSwarmTestSuite) waitForContainers(expected int, name string) {
	time.Sleep(2 * time.Second)
	i := 1
	for {
		if s.areContainersRunning(expected, name) {
			break
		}
		if i > 30 {
			fmt.Printf("Failed to run the service %s\n", name)
			out, _ := exec.Command("/bin/sh", "-c", "docker service ps -f desired-state=Running "+name).Output()
			println(string(out))
			out, _ = exec.Command("/bin/sh", "-c", "docker service ls").Output()
			println(string(out))
			break
		}
		fmt.Printf("Waiting for %d tasks of service %s...\n", expected, name)
		i = i + 1
		time.Sleep(1 * time.Second)
	}
	time.Sleep(5 * time.Second)
}

func (s *IntegrationSwarmTestSuite) createGoDemoService() {
	cmd := `docker service create --name go-demo-api \
    -e DB=go-demo-db \
    --network go-demo \
    --network proxy \
    --label com.df.notify=true \
    --label com.df.distribute=true \
    --label com.df.servicePath=/demo \
    --label com.df.port=8080 \
    vfarcic/go-demo:no-health`
	s.createService(cmd)
}

func (s *IntegrationSwarmTestSuite) sendHelloRequest() (*http.Response, error) {
	url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)
	return http.Get(url)
}

func (s *IntegrationSwarmTestSuite) reconfigureGoDemo(extraParams string) {
	params := fmt.Sprintf("serviceName=go-demo-api&servicePath=/demo&port=8080%s", extraParams)
	s.reconfigureService(params)
}

func (s *IntegrationSwarmTestSuite) reconfigureRedis() {
	s.reconfigureService("serviceName=redis&port=6379&srcPort=6379&reqMode=tcp")
}

func (s *IntegrationSwarmTestSuite) reconfigureService(params string) {
	url := fmt.Sprintf(
		"http://%s:8080/v1/docker-flow-proxy/reconfigure?%s",
		s.hostIP,
		params,
	)
	resp, err := http.Get(url)
	if err != nil {
		println(s.getProxyConf(""))
		s.Fail(err.Error())
	} else {
		msg := fmt.Sprintf(
			`Failed to reconfigure the proxy by sending a request to URL %s

CONFIGURATION:
%s`,
			url,
			s.getProxyConf(""))
		s.Equal(200, resp.StatusCode, msg)
	}
	time.Sleep(5 * time.Second)
}

func (s *IntegrationSwarmTestSuite) reloadService(params string) {
	url := fmt.Sprintf(
		"http://%s:8080/v1/docker-flow-proxy/reload%s",
		s.hostIP,
		params,
	)
	resp, err := http.Get(url)
	if err != nil {
		s.Fail(err.Error())
	} else {
		msg := fmt.Sprintf(
			`Failed to reload the proxy by sending a request to URL %s

CONFIGURATION:
%s`,
			url,
			s.getProxyConf(""))
		s.Equal(200, resp.StatusCode, msg)
	}
	time.Sleep(1 * time.Second)
}

func (s *IntegrationSwarmTestSuite) getProxyConf(respType string) string {
	configAddr := fmt.Sprintf(
		"http://%s:8080/v1/docker-flow-proxy/config",
		s.hostIP,
	)
	if strings.EqualFold(respType, "json") {
		configAddr += "?type=json"
	}
	resp, err := http.Get(configAddr)
	if err != nil {
		println(err.Error())
		return ""
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body)
}
