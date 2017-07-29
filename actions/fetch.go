package actions

import (
	"../proxy"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Fetchable defines interface that fetches information from other sources
type Fetchable interface {
	// Sends request to swarm-listener to request reconfiguration of all proxy instances in Swarm.
	ReloadClusterConfig(listenerAddr string) error
	// Reconfigures this instance of proxy based on configuration taken from swarm-listener.
	// This is synchronous.
	// If listenerAddr is nil, unreachable or any other problem error is returned.
	ReloadConfig(baseData BaseReconfigure, listenerAddr string) error
}
type fetch struct {
	BaseReconfigure
}

// NewFetch returns instance of the Fetchable object
var NewFetch = func(baseData BaseReconfigure) Fetchable {
	return &fetch{
		BaseReconfigure: baseData,
	}
}

// ReloadConfig recreates proxy configuration with data fetches from Swarm Listener
func (m *fetch) ReloadConfig(baseData BaseReconfigure, listenerAddr string) error {
	if len(listenerAddr) == 0 {
		return fmt.Errorf("Swarm Listener address is missing %s", listenerAddr)
	}
	fullAddress := fmt.Sprintf("%s/v1/docker-flow-swarm-listener/get-services", listenerAddr)
	resp, err := httpGet(fullAddress)
	if err != nil {
		return err
	} else if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Swarm Listener responded with the status code %d", resp.StatusCode)
	}
	logPrintf("Got configuration from %s.", listenerAddr)
	defer resp.Body.Close()
	services := []map[string]string{}
	if err = json.NewDecoder(resp.Body).Decode(&services); err != nil {
		return err
	}
	needsReload := false
	for _, s := range services {
		proxyService := proxy.GetServiceFromMap(&s)
		if statusCode, _ := proxy.IsValidReconf(proxyService); statusCode == http.StatusOK {
			reconfigure := NewReconfigure(baseData, *proxyService)
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

// ReloadClusterConfig sends a request to Swarm Listener that will, in turn, send reconfigure requests for each service
func (m *fetch) ReloadClusterConfig(listenerAddr string) error {
	if len(listenerAddr) > 0 {
		if !strings.Contains(listenerAddr, ":") {
			listenerAddr += ":8080"
		}
		if !strings.HasPrefix(listenerAddr, "http://") {
			listenerAddr = "http://" + listenerAddr
		}
		listenerAddr := fmt.Sprintf("%s/v1/docker-flow-swarm-listener/notify-services", listenerAddr)
		if resp, err := httpGet(listenerAddr); err != nil {
			return err
		} else if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("Swarm Listener responded with the status code %d", resp.StatusCode)
		}
		logPrintf("A request was sent to the Swarm listener running on %s. The proxy will be reconfigured soon.", listenerAddr)
	}
	return nil
}

func (m *fetch) getReconfigure(service *proxy.Service) Reconfigurable {
	return NewReconfigure(m.BaseReconfigure, *service)
}

func (m *fetch) getReload() Reloader {
	return NewReload()
}
