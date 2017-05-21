package registry

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"
)

type Consul struct{}

//TODO: Cache a valid address

var cmdRunConsulTemplate = func(cmd *exec.Cmd) error {
	return cmd.Run()
}
var WriteConsulTemplateFile = ioutil.WriteFile

type CreateConfigsArgs struct {
	Addresses     []string
	TemplatesPath string
	FeFile        string
	FeTemplate    string
	BeFile        string
	BeTemplate    string
	ServiceName   string
}

func (m Consul) PutService(addresses []string, instanceName string, r Registry) error {
	consulChannel := make(chan error)
	type data struct{ key, value string }
	d := []data{
		{COLOR_KEY, r.ServiceColor},
		{PATH_KEY, strings.Join(r.ServicePath, ",")},
		{DOMAIN_KEY, strings.Join(r.ServiceDomain, ",")},
		{HOSTNAME_KEY, r.OutboundHostname},
		{PATH_TYPE_KEY, r.PathType},
		{CONSUL_TEMPLATE_FE_PATH_KEY, r.ConsulTemplateFePath},
		{CONSUL_TEMPLATE_BE_PATH_KEY, r.ConsulTemplateBePath},
		{PORT, r.Port},
	}
	for _, e := range d {
		go m.SendPutRequest(addresses, r.ServiceName, e.key, e.value, instanceName, consulChannel)
	}
	go m.SendPutRequest(addresses, "service", r.ServiceName, "swarm", instanceName, consulChannel)
	for i := 0; i < len(d)+1; i++ {
		err := <-consulChannel
		if err != nil {
			return fmt.Errorf("Could not send KV data to Consul\n%s", err.Error())
		}
	}
	return nil
}

func (m Consul) SendPutRequest(addresses []string, serviceName, key, value, instanceName string, c chan error) {
	c <- m.sendRequest("PUT", addresses, serviceName, key, value, instanceName)
}

func (m Consul) DeleteService(addresses []string, serviceName, instanceName string) error {
	var err error
	for _, address := range addresses {
		if !strings.HasPrefix(address, "http") {
			address = fmt.Sprintf("http://%s", address)
		}
		url := fmt.Sprintf("%s/v1/kv/%s/%s?recurse", address, instanceName, serviceName)
		client := &http.Client{}
		request, _ := http.NewRequest("DELETE", url, nil)
		_, err = client.Do(request)
		if err == nil {
			return nil
		}
	}
	return err
}

func (m Consul) CreateConfigs(args *CreateConfigsArgs) error {
	if err := m.createConfig(
		args.Addresses,
		args.TemplatesPath,
		args.FeFile,
		args.FeTemplate,
		args.ServiceName,
		"fe",
	); err != nil {
		return err
	}
	if err := m.createConfig(
		args.Addresses,
		args.TemplatesPath,
		args.BeFile,
		args.BeTemplate,
		args.ServiceName,
		"be",
	); err != nil {
		return err
	}
	return nil
}

func (m Consul) GetServiceAttribute(addresses []string, serviceName, key, instanceName string) (string, error) {
	var err error
	for _, address := range addresses {
		url := fmt.Sprintf("%s/v1/kv/%s/%s/%s?raw", address, instanceName, serviceName, key)
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			return string(body), nil
		}
	}
	return "", fmt.Errorf("Could not retrieve the attribute %s\n%s", key, err)
}

func (m Consul) createConfig(addresses []string, templatesPath, file, template, serviceName, confType string) error {
	if len(template) > 0 {
		src := fmt.Sprintf("%s/%s", templatesPath, file)
		WriteConsulTemplateFile(src, []byte(template), 0664)
		dest := fmt.Sprintf("%s/%s-%s", templatesPath, serviceName, confType)
		var err error
		for _, address := range addresses {
			if err = m.runConsulTemplateCmd(src, dest, address); err == nil {
				return nil
			}
		}
		return fmt.Errorf("Could not create Consul configuration %s from the template %s\n%s", dest, src, err.Error())
	}
	return nil
}

func (m Consul) sendRequest(requestType string, addresses []string, serviceName, key, value, instanceName string) error {
	var err error
	for _, address := range addresses {
		if !strings.HasPrefix(address, "http") {
			address = fmt.Sprintf("http://%s", address)
		}
		url := fmt.Sprintf("%s/v1/kv/%s/%s/%s", address, instanceName, serviceName, key)
		client := &http.Client{}
		request, _ := http.NewRequest(requestType, url, strings.NewReader(value))
		_, err = client.Do(request)
		if err == nil {
			return nil
		}
	}
	return err
}

func (m Consul) runConsulTemplateCmd(src, dest, address string) error {
	template := fmt.Sprintf(`%s:%s.cfg`, src, dest)
	cmdArgs := []string{
		"-consul", m.getConsulAddress(address),
		"-template", template,
		"-once",
	}
	cmd := exec.Command("consul-template", cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmdRunConsulTemplate(cmd); err != nil {
		return fmt.Errorf("Command: %s\n%s\n", strings.Join(cmd.Args, " "), err.Error())
	}
	return nil
}

func (m Consul) getConsulAddress(address string) string {
	a := strings.ToLower(address)
	a = strings.TrimLeft(a, "http://")
	a = strings.TrimLeft(a, "https://")
	return a
}
