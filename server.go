package main

import (
	"./actions"
	"./metrics"
	"./proxy"
	"./server"
	"fmt"
	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

// Server defines interface used for creating DFP Web server
// TODO: Move to server package
type Server interface {
	Execute(args []string) error
}

type serve struct {
	IP              string `short:"i" long:"ip" default:"0.0.0.0" env:"IP" description:"IP the server listens to."`
	ListenerAddress string `short:"l" long:"listener-address" env:"LISTENER_ADDRESS" description:"The address of the Docker Flow: Swarm Listener. The address matches the name of the Swarm service (e.g. swarm-listener)"`
	Port            string `short:"p" long:"port" default:"8080" env:"PORT" description:"Port the server listens to."`
	ServiceName     string `short:"n" long:"service-name" default:"proxy" env:"SERVICE_NAME" description:"The name of the proxy service. It is used only when running in 'swarm' mode and must match the '--name' parameter used to launch the service."`
	// TODO: Remove
	actions.BaseReconfigure
}

var serverImpl = serve{}
var cert server.Certer = server.NewCert("/certs")

// Execute runs the Web server.
// Args are not used and are present only for compatibility reasons. Define them as an empty slice.
func (m *serve) Execute(args []string) error {
	if proxy.Instance == nil {
		proxy.Instance = proxy.NewHaProxy(m.TemplatesPath, m.ConfigsPath)
	}
	logPrintf("Starting HAProxy")
	newRun().Execute([]string{})
	address := fmt.Sprintf("%s:%s", m.IP, m.Port)
	cert.Init()
	var server2 = server.NewServer(
		m.ListenerAddress,
		m.Port,
		m.ServiceName,
		m.ConfigsPath,
		m.TemplatesPath,
		cert,
	)
	config := server.NewConfig()
	sm := server.NewMetrics("")
	if err := m.reconfigure(server2); err != nil {
		return err
	}
	metrics.SetupHandler(server.GetCreds())
	logPrintf(`Starting "Docker Flow: Proxy"`)
	r := mux.NewRouter().StrictSlash(true)
	r.HandleFunc("/v1/docker-flow-proxy/cert", m.certPutHandler).Methods("PUT")
	r.HandleFunc("/v1/docker-flow-proxy/certs", m.certsHandler)
	r.HandleFunc("/v1/docker-flow-proxy/config", config.Get)
	r.HandleFunc("/v1/docker-flow-proxy/metrics", sm.Get)
	r.Handle("/metrics", prometheus.Handler())
	r.HandleFunc("/v1/docker-flow-proxy/ping", server2.PingHandler)
	r.HandleFunc("/v1/docker-flow-proxy/reconfigure", server2.ReconfigureHandler)
	r.HandleFunc("/v1/docker-flow-proxy/reload", server2.ReloadHandler)
	r.HandleFunc("/v1/docker-flow-proxy/remove", server2.RemoveHandler)
	r.HandleFunc("/v1/test", server2.Test1Handler)
	r.HandleFunc("/v2/test", server2.Test2Handler)
	if err := httpListenAndServe(address, r); err != nil {
		return err
	}
	return nil
}

func (m *serve) reconfigure(server server.Server) error {
	lAddr := ""
	if len(m.ListenerAddress) > 0 {
		lAddr = fmt.Sprintf("http://%s:8080", m.ListenerAddress)
	}
	fetch := actions.NewFetch(m.BaseReconfigure)
	if len(lAddr) > 0 {
		go func() {
			retryInterval := os.Getenv("RELOAD_INTERVAL")
			interval, _ := time.ParseDuration(retryInterval + "ms")
			repeatReload := strings.EqualFold(os.Getenv("REPEAT_RELOAD"), "true")
			for range time.Tick(interval) {
				if err := fetch.ReloadConfig(m.BaseReconfigure, lAddr); err != nil {
					logPrintf(
						"Error: Fetching config from swarm listener failed: %s. Will retry in %d seconds.",
						err.Error(),
						interval/time.Second,
					)
				} else if !repeatReload {
					break
				}
			}

		}()
	}

	services := server.GetServicesFromEnvVars()

	for _, service := range *services {
		recon := actions.NewReconfigure(m.BaseReconfigure, service)
		//todo: there could be only one reload after this whole loop
		recon.Execute(true)
	}
	return nil
}

// TODO: Move to server package
func (m *serve) certPutHandler(w http.ResponseWriter, req *http.Request) {
	cert.Put(w, req)
}

// TODO: Move to server package
func (m *serve) certsHandler(w http.ResponseWriter, req *http.Request) {
	cert.GetAll(w, req)
}

func (m *serve) hasPort(sd []proxy.ServiceDest) bool {
	return len(sd) > 0 && len(sd[0].Port) > 0
}

func (m *serve) getBoolParam(req *http.Request, param string) bool {
	value := false
	if len(req.URL.Query().Get(param)) > 0 {
		value, _ = strconv.ParseBool(req.URL.Query().Get(param))
	}
	return value
}

func (m *serve) writeBadRequest(w http.ResponseWriter, resp *server.Response, msg string) {
	resp.Status = "NOK"
	resp.Message = msg
	w.WriteHeader(http.StatusBadRequest)
}

func (m *serve) writeInternalServerError(w http.ResponseWriter, resp *server.Response, msg string) {
	resp.Status = "NOK"
	resp.Message = msg
	w.WriteHeader(http.StatusInternalServerError)
}
