package actions

import (
	"../proxy"
	"../registry"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

type Fetchable interface {
	ReloadServicesFromRegistry(addresses []string, instanceName, mode string) error
	//sends request to swarm-listener to request reconfiguration of all
	//proxy instances in swarm
	ReloadClusterConfig(listenerAddr string) error
	//reconfigures this instance of proxy based on configuration taken from
	//swarm-listener. This is synchronous. if listenerAddr is nil, unreachable
	//or any other problem error is returned.
	ReloadConfig(baseData BaseReconfigure, mode string, listenerAddr string) error
}
type Fetch struct {
	BaseReconfigure
	Mode string `short:"m" long:"mode" env:"MODE" description:"If set to 'swarm', proxy will operate assuming that Docker service from v1.12+ is used."`
}

var NewFetch = func(baseData BaseReconfigure, mode string) Fetchable {
	return &Fetch{
		BaseReconfigure: baseData,
		Mode:            mode,
	}
}

func (m *Fetch) ReloadServicesFromRegistry(addresses []string, instanceName, mode string) error {
	if len(addresses) > 0 {
		return m.reloadFromRegistry(addresses, instanceName, mode)
	}
	return nil
}

func (m *Fetch) ReloadConfig(baseData BaseReconfigure, mode string, listenerAddr string) error {
	if len(listenerAddr) > 0 {
		fullAddress := fmt.Sprintf("%s/v1/docker-flow-swarm-listener/get-services", listenerAddr)
		resp, err := httpGet(fullAddress)
		if err != nil {
			return err
		} else if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Swarm Listener responded with the status code %d", resp.StatusCode)
		} else {
			logPrintf("Got configuration from %s.", listenerAddr)
			defer resp.Body.Close()
			services := []map[string]string{}
			err := json.NewDecoder(resp.Body).Decode(&services)
			if err != nil {
				return err
			}
			needsReload := false
			for _, s := range services {
				proxyService := proxy.GetServiceFromMap(&s)
				if statusCode, _ := proxy.IsValidReconf(proxyService); statusCode == http.StatusOK {
					reconfigure := NewReconfigure(baseData, *proxyService, mode)
					reconfigure.Execute(false)
					needsReload = true
				}
			}
			if needsReload {
				reload := m.getReload()
				reload.Execute(true)
			}
			return nil
		}

	}
	return fmt.Errorf("Swarm Listener address is missing %s", listenerAddr)
}

func (m *Fetch) ReloadClusterConfig(listenerAddr string) error {
	if len(listenerAddr) > 0 {
		fullAddress := fmt.Sprintf("%s/v1/docker-flow-swarm-listener/notify-services", listenerAddr)
		resp, err := httpGet(fullAddress)
		if err != nil {
			return err
		} else if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Swarm Listener responded with the status code %d", resp.StatusCode)
		}
		logPrintf("A request was sent to the Swarm listener running on %s. The proxy will be reconfigured soon.", listenerAddr)
	}
	return nil
}

func (m *Fetch) getReconfigure(service *proxy.Service) Reconfigurable {
	return NewReconfigure(m.BaseReconfigure, *service, m.Mode)
}

func (m *Fetch) getReload() Reloader {
	return NewReload()
}

func (m *Fetch) reloadFromRegistry(addresses []string, instanceName, mode string) error {
	var resp *http.Response
	var err error
	logPrintf("Configuring existing services")
	found := false
	for _, address := range addresses {
		address = strings.ToLower(address)
		if !strings.HasPrefix(address, "http") {
			address = fmt.Sprintf("http://%s", address)
		}
		servicesUrl := fmt.Sprintf("%s/v1/catalog/services", address)
		resp, err = http.Get(servicesUrl)
		if err == nil {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("Could not retrieve the list of services from Consul")
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	c := make(chan proxy.Service)
	count := 0
	var data map[string]interface{}
	json.Unmarshal(body, &data)
	count = len(data)
	for key := range data {
		go m.getService(addresses, key, instanceName, c)
	}
	logPrintf("\tFound %d services", count)
	for i := 0; i < count; i++ {
		s := <-c
		if len(s.ServiceDest) > 0 && len(s.ServiceDest[0].ServicePath) > 0 {
			reconfigure := m.getReconfigure(&s)
			reconfigure.Execute(false)
		}
	}
	reload := m.getReload()
	return reload.Execute(true)
}

func (m *Fetch) getService(addresses []string, serviceName, instanceName string, c chan proxy.Service) {
	sr := proxy.Service{ServiceName: serviceName}

	path, err := registryInstance.GetServiceAttribute(addresses, serviceName, registry.PATH_KEY, instanceName)
	domain, err := registryInstance.GetServiceAttribute(addresses, serviceName, registry.DOMAIN_KEY, instanceName)
	port, _ := m.getServiceAttribute(addresses, serviceName, registry.PORT, instanceName)
	sd := proxy.ServiceDest{
		ServicePath: strings.Split(path, ","),
		Port:        port,
	}
	if err == nil {
		sr.ServiceDest = []proxy.ServiceDest{sd}
		sr.ServiceColor, _ = m.getServiceAttribute(addresses, serviceName, registry.COLOR_KEY, instanceName)
		sr.ServiceDomain = strings.Split(domain, ",")
		sr.ServiceCert, _ = m.getServiceAttribute(addresses, serviceName, registry.CERT_KEY, instanceName)
		sr.OutboundHostname, _ = m.getServiceAttribute(addresses, serviceName, registry.HOSTNAME_KEY, instanceName)
		sr.PathType, _ = m.getServiceAttribute(addresses, serviceName, registry.PATH_TYPE_KEY, instanceName)
		skipCheck, _ := m.getServiceAttribute(addresses, serviceName, registry.SKIP_CHECK_KEY, instanceName)
		sr.SkipCheck, _ = strconv.ParseBool(skipCheck)
		sr.ConsulTemplateFePath, _ = m.getServiceAttribute(addresses, serviceName, registry.CONSUL_TEMPLATE_FE_PATH_KEY, instanceName)
		sr.ConsulTemplateBePath, _ = m.getServiceAttribute(addresses, serviceName, registry.CONSUL_TEMPLATE_BE_PATH_KEY, instanceName)
	}
	c <- sr
}

// TODO: Remove in favour of registry.GetServiceAttribute
func (m *Fetch) getServiceAttribute(addresses []string, serviceName, key, instanceName string) (string, bool) {
	for _, address := range addresses {
		url := fmt.Sprintf("%s/v1/kv/%s/%s/%s?raw", address, instanceName, serviceName, key)
		resp, err := http.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			return string(body), true
		}
	}
	return "", false
}
