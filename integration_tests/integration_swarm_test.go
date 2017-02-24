package integration_test

import (
	"bytes"
	"fmt"
	"github.com/stretchr/testify/suite"
	"io/ioutil"
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

	exec.Command("/bin/sh", "-c", `docker rm $(docker ps -qa)`).Output()

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
    -e MODE=swarm \
    %s/docker-flow-proxy:beta`,
		s.dockerHubUser)
	s.createService(cmd)

	s.createService(`docker service create --name go-demo-db \
    --network go-demo \
    mongo`)

	s.createGoDemoService()

	s.waitForContainers(1, "proxy")

	suite.Run(t, s)

	s.removeServices("go-demo", "go-demo-db", "proxy")
}

// Tests

func (s IntegrationSwarmTestSuite) Test_Reconfigure() {
	s.reconfigureGoDemo("")

	resp, err := s.sendHelloRequest()

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf())
}

func (s IntegrationSwarmTestSuite) Test_Remove() {
	s.reconfigureGoDemo("")

	url := fmt.Sprintf(
		"http://%s:8080/v1/docker-flow-proxy/remove?serviceName=go-demo",
		s.hostIP,
	)
	http.Get(url)

	resp, err := s.sendHelloRequest()

	s.NoError(err)
	s.Equal(503, resp.StatusCode, s.getProxyConf())
}

func (s IntegrationSwarmTestSuite) Test_Scale() {
	defer func() {
		exec.Command("/bin/sh", "-c", "docker service scale proxy=1").Output()
		s.waitForContainers(1, "proxy")
	}()
	exec.Command("/bin/sh", "-c", "docker service scale proxy=3").Output()
	s.waitForContainers(3, "proxy")

	s.reconfigureGoDemo("&distribute=true")

	for i := 0; i < 10; i++ {
		resp, err := s.sendHelloRequest()

		s.NoError(err)
		s.Equal(200, resp.StatusCode)
	}

}

func (s IntegrationSwarmTestSuite) Test_RewritePaths() {
	url := fmt.Sprintf(
		"http://%s:8080/v1/docker-flow-proxy/reconfigure?serviceName=go-demo&servicePath=/something&port=8080&reqPathSearch=/something/&reqPathReplace=/demo/",
		s.hostIP,
	)
	http.Get(url)

	url = fmt.Sprintf("http://%s/something/hello", s.hostIP)
	resp, err := http.Get(url)

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf())
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
	s.Equal(401, resp.StatusCode, s.getProxyConf())

	url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)
	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("my-user", "my-pass")
	client := &http.Client{}
	resp, err = client.Do(req)

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf())
}

func (s IntegrationSwarmTestSuite) Test_GlobalAuthenticationWithEncryption() {
	defer func() {
		exec.Command("/bin/sh", "-c", `docker service update --env-rm "USERS" proxy`).Output()
		s.waitForContainers(1, "proxy")
	}()
	_, err := exec.Command("/bin/sh", "-c", `docker service update --env-add "USERS_PASS_ENCRYPTED=true" --env-add "USERS=my-user:\$6\$AcrjVWOkQq1vWp\$t55F7Psm3Ujvp8lpqdAwrc5RxWORYBeDV6ji9KoO029ojooj4Pi.JVGwxdicB0Fuu.NSDyGaZt7skHIo3Nayq/" proxy`).Output()
	s.NoError(err)
	s.waitForContainers(1, "proxy")

	s.reconfigureGoDemo("")

	resp, err := s.sendHelloRequest()

	s.NoError(err)
	s.Equal(401, resp.StatusCode, s.getProxyConf())

	url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)
	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("my-user", "my-pass")
	client := &http.Client{}
	resp, err = client.Do(req)

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf())
}

func (s IntegrationSwarmTestSuite) Test_ServiceAuthentication() {
	defer func() {
		s.reconfigureGoDemo("")
	}()

	s.reconfigureGoDemo("&users=admin:password")

	resp, err := s.sendHelloRequest()

	if err != nil {
		s.Fail(err.Error())
	} else {
		s.Equal(401, resp.StatusCode, s.getProxyConf())
	}

	url := fmt.Sprintf("http://%s/demo/hello", s.hostIP)
	req, err := http.NewRequest("GET", url, nil)
	req.SetBasicAuth("admin", "password")
	client := &http.Client{}
	resp, err = client.Do(req)

	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf())
}

func (s IntegrationSwarmTestSuite) Test_Tcp() {
	defer func() {
		s.removeServices("redis")
		s.waitForContainers(0, "redis")
	}()
	cmdString := `docker service create --name redis \
	--network proxy \
	redis:3.2`
	exec.Command("/bin/sh", "-c", cmdString).Output()
	s.waitForContainers(1, "redis")
	s.reconfigureRedis()

	cmdString = fmt.Sprintf("ADDR=%s PORT=6379 /usr/src/myapp/integration_tests/redis_check.sh", s.hostIP)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd := exec.Command("/bin/sh", "-c", cmdString)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	s.NoError(
		err,
		"CONFIG\n%s\n\nOUT:\n%s\n\nERR:\n%s",
		s.getProxyConf(),
		stdout.String(),
		stderr.String(),
	)
}

func (s IntegrationSwarmTestSuite) Test_Reload() {

	// Reconfigure

	s.reconfigureGoDemo("")
	resp, err := s.sendHelloRequest()
	s.NoError(err)
	s.Equal(200, resp.StatusCode, s.getProxyConf())

	// Corrupt the config

	out, _ := exec.Command("/bin/sh", "-c", "docker ps -q -f label=com.docker.swarm.service.name=proxy").Output()
	id := strings.TrimRight(string(out), "\n")
	cmd := fmt.Sprintf("docker cp /tmp/haproxy.cfg %s:/cfg/haproxy.cfg", id)
	if f, err := os.Create("/tmp/haproxy.cfg"); err != nil {
		s.Fail(err.Error())
	} else {
		f.Write([]byte("This config is corrupt"))
	}
	out, _ = exec.Command("/bin/sh", "-c", cmd).Output()

	// Reload with reconfigure

	s.reloadService("?recreate=true")
	config := s.getProxyConf()
	s.NotEqual("This config is corrupt", config)
}

//func (s IntegrationSwarmTestSuite) Test_HttpsOnly() {
//	s.reconfigureGoDemo("&httpsOnly=true")
//
//	resp, err := s.sendHelloRequest()
//
//	if err != nil {
//		s.Fail("Failed to block HTTPS requests", "ERROR: %s\n\nConfig: %s", err.Error(), s.getProxyConf())
//	} else {
//		s.Equal(200, resp.StatusCode, s.getProxyConf())
//	}
//}

// Util

func (s *IntegrationSwarmTestSuite) areContainersRunning(expected int, name string) bool {
	out, _ := exec.Command("/bin/sh", "-c", "docker ps -q -f label=com.docker.swarm.service.name=" + name ).Output()
	lines := strings.Split(string(out), "\n")
	return len(lines) == (expected +1) //+1 because there is new line at the end of ps output
}

func (s *IntegrationSwarmTestSuite) createService(command string) {
	exec.Command("/bin/sh", "-c", command).Output()
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
		if i > 20 {
			fmt.Printf("Waiting for %d tasks of service %s...\n", expected, name)
		}
		i = i + 1
		time.Sleep(1 * time.Second)
	}
	time.Sleep(2 * time.Second)
}

func (s *IntegrationSwarmTestSuite) createGoDemoService() {
	cmd := `docker service create --name go-demo \
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
	params := fmt.Sprintf("serviceName=go-demo&servicePath=/demo&port=8080%s", extraParams)
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
		s.Fail(err.Error())
	} else {
		msg := fmt.Sprintf(
			`Failed to reconfigure the proxy by sending a request to URL %s

CONFIGURATION:
%s`,
			url,
			s.getProxyConf())
		s.Equal(200, resp.StatusCode, msg)
	}
	time.Sleep(1 * time.Second)
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
			s.getProxyConf())
		s.Equal(200, resp.StatusCode, msg)
	}
	time.Sleep(1 * time.Second)
}

func (s *IntegrationSwarmTestSuite) getProxyConf() string {
	configAddr := fmt.Sprintf(
		"http://%s:8080/v1/docker-flow-proxy/config",
		s.hostIP,
	)
	resp, err := http.Get(configAddr)
	if err != nil {
		println(err.Error())
		return ""
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	return string(body)
}
