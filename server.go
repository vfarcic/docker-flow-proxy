package main

import (
	"./actions"
	"./proxy"
	"./server"
	"fmt"
	"github.com/gorilla/mux"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// TODO: Move to server package

type Server interface {
	Execute(args []string) error
}

type Serve struct {
	IP string `short:"i" long:"ip" default:"0.0.0.0" env:"IP" description:"IP the server listens to."`
	// TODO: Remove
	// The default mode is designed to work with any setup and requires Consul and Registrator.
	// The swarm mode aims to leverage the benefits that come with Docker Swarm and new networking introduced in the 1.12 release.
	// The later mode (swarm) does not have any dependency but Docker Engine.
	// The swarm mode is recommended for all who use Docker Swarm features introduced in v1.12.
	Mode            string `short:"m" long:"mode" env:"MODE" description:"If set to 'swarm', proxy will operate assuming that Docker service from v1.12+ is used."`
	ListenerAddress string `short:"l" long:"listener-address" env:"LISTENER_ADDRESS" description:"The address of the Docker Flow: Swarm Listener. The address matches the name of the Swarm service (e.g. swarm-listener)"`
	Port            string `short:"p" long:"port" default:"8080" env:"PORT" description:"Port the server listens to."`
	ServiceName     string `short:"n" long:"service-name" default:"proxy" env:"SERVICE_NAME" description:"The name of the proxy service. It is used only when running in 'swarm' mode and must match the '--name' parameter used to launch the service."`
	// TODO: Remove
	actions.BaseReconfigure
}

var serverImpl = Serve{}
var cert server.Certer = server.NewCert("/certs")
var retryInterval time.Duration = 5000

func (m *Serve) Execute(args []string) error {
	if proxy.Instance == nil {
		proxy.Instance = proxy.NewHaProxy(m.TemplatesPath, m.ConfigsPath)
	}
	logPrintf("Starting HAProxy")
	m.setConsulAddresses()
	NewRun().Execute([]string{})
	address := fmt.Sprintf("%s:%s", m.IP, m.Port)
	cert.Init()
	var server2 = server.NewServer(
		m.ListenerAddress,
		m.Mode,
		m.Port,
		m.ServiceName,
		m.ConfigsPath,
		m.TemplatesPath,
		m.ConsulAddresses,
		cert,
	)
	if err := m.reconfigure(server2); err != nil {
		return err
	}
	logPrintf(`Starting "Docker Flow: Proxy"`)
	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/v1/docker-flow-proxy/cert", m.CertPutHandler).Methods("PUT")
	r.HandleFunc("/v1/docker-flow-proxy/certs", m.CertsHandler)
	r.HandleFunc("/v1/docker-flow-proxy/config", m.ConfigHandler)
	r.HandleFunc("/v1/docker-flow-proxy/reconfigure", server2.ReconfigureHandler)
	r.HandleFunc("/v1/docker-flow-proxy/remove", server2.RemoveHandler)
	r.HandleFunc("/v1/docker-flow-proxy/reload", server2.ReloadHandler)
	r.HandleFunc("/v1/test", server2.TestHandler)
	r.HandleFunc("/v2/test", server2.TestHandler)
	if err := httpListenAndServe(address, r); err != nil {
		return err
	}
	return nil
}

func (m *Serve) reconfigure(server server.Server) error {
	lAddr := ""
	if len(m.ListenerAddress) > 0 {
		lAddr = fmt.Sprintf("http://%s:8080", m.ListenerAddress)
	}
	fetch := actions.NewFetch(m.BaseReconfigure, m.Mode)
	if err := fetch.ReloadServicesFromRegistry(
		m.ConsulAddresses,
		m.InstanceName,
		m.Mode,
	); err != nil {
		return err
	}
	if len(lAddr) > 0 {
		go func() {
			interval := time.Millisecond * retryInterval
			for range time.Tick(interval) {
				if err := fetch.ReloadConfig(m.BaseReconfigure, m.Mode, lAddr); err != nil {
					logPrintf(
						"Error: Fetching config from swarm listener failed: %s. Will retry in %d seconds.",
						err.Error(),
						interval/time.Second,
					)
				} else {
					break
				}
			}

		}()
	}

	services := server.GetServicesFromEnvVars()

	for _, service := range *services {
		recon := actions.NewReconfigure(m.BaseReconfigure, service, m.Mode)
		//todo: there could be only one reload after this whole loop
		recon.Execute(true)
	}
	return nil
}

// TODO: Move to server package
func (m *Serve) CertPutHandler(w http.ResponseWriter, req *http.Request) {
	cert.Put(w, req)
}

// TODO: Move to server package
func (m *Serve) CertsHandler(w http.ResponseWriter, req *http.Request) {
	cert.GetAll(w, req)
}

// TODO: Move to server package
func (m *Serve) ConfigHandler(w http.ResponseWriter, req *http.Request) {
	m.config(w, req)
}

func (m *Serve) isSwarm(mode string) bool {
	return strings.EqualFold("service", m.Mode) || strings.EqualFold("swarm", m.Mode)
}

func (m *Serve) hasPort(sd []proxy.ServiceDest) bool {
	return len(sd) > 0 && len(sd[0].Port) > 0
}

func (m *Serve) getBoolParam(req *http.Request, param string) bool {
	value := false
	if len(req.URL.Query().Get(param)) > 0 {
		value, _ = strconv.ParseBool(req.URL.Query().Get(param))
	}
	return value
}

func (m *Serve) writeBadRequest(w http.ResponseWriter, resp *server.Response, msg string) {
	resp.Status = "NOK"
	resp.Message = msg
	w.WriteHeader(http.StatusBadRequest)
}

func (m *Serve) writeInternalServerError(w http.ResponseWriter, resp *server.Response, msg string) {
	resp.Status = "NOK"
	resp.Message = msg
	w.WriteHeader(http.StatusInternalServerError)
}

func (m *Serve) config(w http.ResponseWriter, req *http.Request) {
	httpWriterSetContentType(w, "text/html")
	out, err := proxy.Instance.ReadConfig()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	w.Write([]byte(out))
}

func (m *Serve) setConsulAddresses() {
	m.ConsulAddresses = []string{}
	if len(os.Getenv("CONSUL_ADDRESS")) > 0 {
		for _, address := range strings.Split(os.Getenv("CONSUL_ADDRESS"), ",") {
			if !strings.HasPrefix(address, "http") {
				address = fmt.Sprintf("http://%s", address)
			}
			m.ConsulAddresses = append(m.ConsulAddresses, address)
		}
	}
}
