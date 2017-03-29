package actions

import (
	"../proxy"
	"fmt"
	"net/http"
	"encoding/json"
)

type Reloader interface {
	Execute(recreate bool) error
	//sends request to swarm-listener to request reconfiguration of all
	//proxy instances in swarm
	ReloadClusterConfig(listenerAddr string) error
	//reconfigures this instance of proxy based on configuration taken from
	//swarm-listener. This is synchronous. if listenerAddr is nil, unreachable
	//or any other problem error is returned.
	ReloadConfig(baseData BaseReconfigure, mode string, listenerAddr string) error
}

type Reload struct{}

func (m *Reload) ReloadConfig(baseData BaseReconfigure, mode string, listenerAddr string) error {
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
			err:=json.NewDecoder(resp.Body).Decode(&services)
			if err != nil {
				return err
			}
			needsReload := false
			for _, s := range services {
				proxyService := proxy.GetServiceFromMap(&s)
				reconfigure := NewReconfigure(baseData, *proxyService, mode)
				reconfigure.Execute(false)
				needsReload = true
			}
			if needsReload {
				m.Execute(false)
			}
			return nil
		}

	}
	return fmt.Errorf("Swarm Listener address is missing %s", listenerAddr)
}

func (m *Reload) ReloadClusterConfig(listenerAddr string) error {
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

func (m *Reload) Execute(recreate bool) error {
	if recreate {
		if err := proxy.Instance.CreateConfigFromTemplates(); err != nil {
			logPrintf(err.Error())
			return err
		}
	}
	if err := proxy.Instance.Reload(); err != nil {
		logPrintf(err.Error())
		return err
	}
	return nil
}

var NewReload = func() Reloader {
	return &Reload{}
}

