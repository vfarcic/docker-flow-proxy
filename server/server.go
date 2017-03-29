package server

import (
	"../actions"
	"../proxy"
	"encoding/json"
	"fmt"
	"github.com/kelseyhightower/envconfig"
	"net/http"
	"os"
	"strconv"
	"strings"
)

type Server interface {
	GetServiceFromUrl(req *http.Request) *proxy.Service
	TestHandler(w http.ResponseWriter, req *http.Request)
	ReconfigureHandler(w http.ResponseWriter, req *http.Request)
	ReloadHandler(w http.ResponseWriter, req *http.Request)
	RemoveHandler(w http.ResponseWriter, req *http.Request)
	GetServicesFromEnvVars() *[]proxy.Service
}

const (
	DISTRIBUTED = "Distributed to all instances"
)

type Serve struct {
	ListenerAddress string
	Mode            string
	Port            string
	ServiceName     string
	ConfigsPath     string
	TemplatesPath   string
	ConsulAddresses []string
	Cert            Certer
}

var NewServer = func(listenerAddr, mode, port, serviceName, configsPath, templatesPath string, consulAddresses []string, cert Certer) Server {
	return &Serve{
		ListenerAddress: listenerAddr,
		Mode:            mode,
		Port:            port,
		ServiceName:     serviceName,
		ConfigsPath:     configsPath,
		TemplatesPath:   templatesPath,
		ConsulAddresses: consulAddresses,
		Cert:            cert,
	}
}

type Response struct {
	Mode        string
	Status      string
	Message     string
	ServiceName string
	proxy.Service
}

type ServiceParameterProvider interface {
	Fill(service *proxy.Service)
	GetString(name string) string
}

type HttpRequestParameterProvider struct {
	*http.Request
}

func (p *HttpRequestParameterProvider) Fill(service *proxy.Service) {
	p.ParseForm()
	decoder.Decode(service, p.Form)
}

func (p *HttpRequestParameterProvider) GetString(name string) string {
	return p.URL.Query().Get(name)
}

func (m *Serve) GetServiceFromUrl(req *http.Request) *proxy.Service {
	provider := HttpRequestParameterProvider{Request: req}
	return proxy.GetServiceFromProvider(&provider)
}

func (m *Serve) TestHandler(w http.ResponseWriter, req *http.Request) {
	js, _ := json.Marshal(Response{Status: "OK"})
	httpWriterSetContentType(w, "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(js)
}

func (m *Serve) ReconfigureHandler(w http.ResponseWriter, req *http.Request) {
	sr := m.GetServiceFromUrl(req)
	response := Response{
		Mode:        m.Mode,
		Status:      "OK",
		ServiceName: sr.ServiceName,
		Service:     *sr,
	}
	ok, msg := m.isValidReconf(sr)
	if ok {
		if m.isSwarm(m.Mode) && !m.hasPort(sr.ServiceDest) {
			m.writeBadRequest(w, &response, `When MODE is set to "service" or "swarm", the port query is mandatory`)
		} else if sr.Distribute {
			if status, err := SendDistributeRequests(req, m.Port, m.ServiceName); err != nil || status >= 300 {
				m.writeInternalServerError(w, &response, err.Error())
			} else {
				response.Message = DISTRIBUTED
				w.WriteHeader(http.StatusOK)
			}
		} else {
			if len(sr.ServiceCert) > 0 {
				// Replace \n with proper carriage return as new lines are not supported in labels
				sr.ServiceCert = strings.Replace(sr.ServiceCert, "\\n", "\n", -1)
				if len(sr.ServiceDomain) > 0 {
					m.Cert.PutCert(sr.ServiceDomain[0], []byte(sr.ServiceCert))
				} else {
					m.Cert.PutCert(sr.ServiceName, []byte(sr.ServiceCert))
				}
			}
			action := actions.NewReconfigure(m.getBaseReconfigure(), *sr, m.Mode)
			if err := action.Execute(true); err != nil {
				m.writeInternalServerError(w, &response, err.Error())
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}
	} else {
		m.writeBadRequest(w, &response, msg)
	}
	httpWriterSetContentType(w, "application/json")
	js, _ := json.Marshal(response)
	w.Write(js)
}

func (m *Serve) getBaseReconfigure() actions.BaseReconfigure {
	//MW: What about skipAddressValidation???
	return actions.BaseReconfigure{
		ConsulAddresses: m.ConsulAddresses,
		ConfigsPath:     m.ConfigsPath,
		InstanceName:    os.Getenv("PROXY_INSTANCE_NAME"),
		TemplatesPath:   m.TemplatesPath,
	}
}

func (m *Serve) ReloadHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	params := new(ReloadParams)
	decoder.Decode(params, req.Form)
	listenerAddr := ""
	response := Response{
		Status: "OK",
	}
	if params.FromListener {
		listenerAddr = m.ListenerAddress
	}
	//MW: I've reconstructed original behavior. BUT.
	//shouldn't reload call ReloadServicesFromRegistry not just
	//reload in else, if so ReloadClusterConfig & ReloadServicesFromRegistry
	//could be enclosed in one method
	if len(listenerAddr) > 0 {
		fetch := actions.NewFetch(m.getBaseReconfigure(), m.Mode)
		if err := fetch.ReloadClusterConfig(listenerAddr); err != nil {
			logPrintf("Error: ReloadClusterConfig failed: %s", err.Error())
			m.writeInternalServerError(w, &Response{}, err.Error())

		} else {
			w.WriteHeader(http.StatusOK)
		}
	} else {
		reload := actions.NewReload()
		if err := reload.Execute(params.Recreate); err != nil {
			logPrintf("Error: ReloadExecute failed: %s", err.Error())
			m.writeInternalServerError(w, &Response{}, err.Error())
		} else {
			w.WriteHeader(http.StatusOK)
		}
	}
	httpWriterSetContentType(w, "application/json")
	js, _ := json.Marshal(response)
	w.Write(js)
}

func (m *Serve) RemoveHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	params := new(RemoveParams)
	decoder.Decode(params, req.Form)
	header := http.StatusOK
	response := Response{
		Status:      "OK",
		ServiceName: params.ServiceName,
	}
	if params.Distribute {
		response.Distribute = params.Distribute
		response.Message = DISTRIBUTED
	}
	if len(params.ServiceName) == 0 {
		response.Status = "NOK"
		response.Message = "The serviceName query is mandatory"
		header = http.StatusBadRequest
	} else if params.Distribute {
		if status, err := SendDistributeRequests(req, m.Port, m.ServiceName); err != nil || status >= 300 {
			response.Status = "NOK"
			response.Message = err.Error()
			header = http.StatusInternalServerError
		}
	} else {
		logPrintf("Processing remove request %s", req.URL.Path)
		action := actions.NewRemove(
			params.ServiceName,
			params.AclName,
			m.ConfigsPath,
			m.TemplatesPath,
			m.ConsulAddresses,
			m.ServiceName,
			m.Mode,
		)
		action.Execute([]string{})
	}
	w.WriteHeader(header)
	httpWriterSetContentType(w, "application/json")
	js, _ := json.Marshal(response)
	w.Write(js)
}

func (m *Serve) GetServicesFromEnvVars() *[]proxy.Service {
	services := []proxy.Service{}
	s, err := m.getServiceFromEnvVars("DFP_SERVICE")
	if err == nil {
		services = append(services, s)
	}
	i := 1
	for {
		s, err := m.getServiceFromEnvVars(fmt.Sprintf("DFP_SERVICE_%d", i))
		if err != nil {
			break
		}
		services = append(services, s)
		i++
	}
	return &services
}

func (m *Serve) getServiceFromEnvVars(prefix string) (proxy.Service, error) {
	var s proxy.Service
	envconfig.Process(prefix, &s)
	if len(s.ServiceName) == 0 {
		return proxy.Service{}, fmt.Errorf("%s_SERVICE_NAME is not set", prefix)
	}
	sd := []proxy.ServiceDest{}
	path := []string{}
	if len(os.Getenv(prefix+"_SERVICE_PATH")) > 0 {
		path = strings.Split(os.Getenv(prefix+"_SERVICE_PATH"), ",")
	}
	port := os.Getenv(prefix + "_PORT")
	srcPort, _ := strconv.Atoi(os.Getenv(prefix + "_SRC_PORT"))
	if len(path) > 0 || len(port) > 0 {
		sd = append(
			sd,
			proxy.ServiceDest{Port: port, SrcPort: srcPort, ServicePath: path},
		)
	}
	for i := 1; i <= 10; i++ {
		port := os.Getenv(fmt.Sprintf("%s_PORT_%d", prefix, i))
		path := os.Getenv(fmt.Sprintf("%s_SERVICE_PATH_%d", prefix, i))
		srcPort, _ := strconv.Atoi(os.Getenv(fmt.Sprintf("%s_SRC_PORT_%d", prefix, i)))
		if len(path) > 0 && len(port) > 0 {
			sd = append(
				sd,
				proxy.ServiceDest{Port: port, SrcPort: srcPort, ServicePath: strings.Split(path, ",")},
			)
		} else {
			break
		}
	}
	s.ServiceDest = sd
	return s, nil
}

func (m *Serve) writeBadRequest(w http.ResponseWriter, resp *Response, msg string) {
	resp.Status = "NOK"
	resp.Message = msg
	w.WriteHeader(http.StatusBadRequest)
}

func (m *Serve) writeInternalServerError(w http.ResponseWriter, resp *Response, msg string) {
	resp.Status = "NOK"
	resp.Message = msg
	w.WriteHeader(http.StatusInternalServerError)
}

func (m *Serve) isSwarm(mode string) bool {
	return strings.EqualFold("service", m.Mode) || strings.EqualFold("swarm", m.Mode)
}

func (m *Serve) hasPort(sd []proxy.ServiceDest) bool {
	return len(sd) > 0 && len(sd[0].Port) > 0
}

func (m *Serve) isValidReconf(service *proxy.Service) (bool, string) {
	if len(service.ServiceName) == 0 {
		return false, "serviceName parameter is mandatory"
	} else if len(service.ServiceDest) == 0 {
		return false, "There must be at least one destination"
	}
	hasPath := len(service.ServiceDest[0].ServicePath) > 0
	hasSrcPort := service.ServiceDest[0].SrcPort > 0
	hasPort := len(service.ServiceDest[0].Port) > 0
	if strings.EqualFold(service.ReqMode, "http") {
		if !hasPath && len(service.ConsulTemplateFePath) == 0 {
			return false, "When using reqMode http, servicePath or (consulTemplateFePath and consulTemplateBePath) are mandatory"
		}
	} else if !hasSrcPort || !hasPort {
		return false, "When NOT using reqMode http (e.g. tcp), srcPort and port parameters are mandatory."
	}
	return true, ""
}
