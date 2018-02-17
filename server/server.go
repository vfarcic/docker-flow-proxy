package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

	"../actions"
	"../proxy"
	"github.com/kelseyhightower/envconfig"
)

// Server handles requests
type Server interface {
	GetServicesFromEnvVars() *[]proxy.Service
	GetServiceFromUrl(req *http.Request) *proxy.Service
	PingHandler(w http.ResponseWriter, req *http.Request)
	ReconfigureHandler(w http.ResponseWriter, req *http.Request)
	ReloadHandler(w http.ResponseWriter, req *http.Request)
	RemoveHandler(w http.ResponseWriter, req *http.Request)
	Test1Handler(w http.ResponseWriter, req *http.Request)
	Test2Handler(w http.ResponseWriter, req *http.Request)
}

const (
	distributed = "Distributed to all instances"
)

type serve struct {
	listenerAddress string
	port            string
	serviceName     string
	configsPath     string
	templatesPath   string
	cert            Certer
}

// NewServer returns instance of the Server with populated data
var NewServer = func(listenerAddr, port, serviceName, configsPath, templatesPath string, cert Certer) Server {
	return &serve{
		listenerAddress: listenerAddr,
		port:            port,
		serviceName:     serviceName,
		configsPath:     configsPath,
		templatesPath:   templatesPath,
		cert:            cert,
	}
}

//Response message returns to HTTP clients
type Response struct {
	Status      string
	Message     string
	ServiceName string
	proxy.Service
}

func (m *serve) GetServiceFromUrl(req *http.Request) *proxy.Service {
	provider := HttpRequestParameterProvider{Request: req}
	return proxy.GetServiceFromProvider(&provider)
}

func (m *serve) Test1Handler(w http.ResponseWriter, req *http.Request) {
	js, _ := json.Marshal(Response{Status: "OK", Message: "Test v1"})
	httpWriterSetContentType(w, "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(js)
}

func (m *serve) Test2Handler(w http.ResponseWriter, req *http.Request) {
	js, _ := json.Marshal(Response{Status: "OK", Message: "Test v2"})
	httpWriterSetContentType(w, "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(js)
}

func (m *serve) PingHandler(w http.ResponseWriter, req *http.Request) {
	js, _ := json.Marshal(Response{Status: "OK"})
	httpWriterSetContentType(w, "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(js)
}

func (m *serve) ReconfigureHandler(w http.ResponseWriter, req *http.Request) {
	sr := m.GetServiceFromUrl(req)
	response := Response{
		Status:      "OK",
		ServiceName: sr.ServiceName,
		Service:     *sr,
	}
	statusCode, msg := proxy.IsValidReconf(sr)
	if statusCode == http.StatusOK {
		if !m.hasPort(sr.ServiceDest) {
			logPrintf(`Port query is mandatory`)
			m.writeBadRequest(w, &response, `When MODE is set to "service" or "swarm", the port query is mandatory`)
		} else if sr.Distribute {
			if status, err := sendDistributeRequests(req, m.port, m.serviceName); err != nil || status >= 300 {
				m.writeInternalServerError(w, &response, err.Error())
			} else {
				response.Message = distributed
				w.WriteHeader(http.StatusOK)
			}
		} else {
			if len(sr.ServiceCert) > 0 {
				// Replace \n with proper carriage return as new lines are not supported in labels
				sr.ServiceCert = strings.Replace(sr.ServiceCert, "\\n", "\n", -1)
				certName := sr.ServiceName
				if len(sr.ServiceDest) > 0 && len(sr.ServiceDest[0].ServiceDomain) > 0 {
					certName = sr.ServiceDest[0].ServiceDomain[0]
				}
				m.cert.PutCert(certName, []byte(sr.ServiceCert))
			}
			action := actions.NewReconfigure(m.getBaseReconfigure(), *sr)
			if err := action.Execute(true); err != nil {
				m.writeInternalServerError(w, &response, err.Error())
			} else {
				w.WriteHeader(http.StatusOK)
			}
		}
	} else if statusCode == http.StatusConflict {
		m.writeConflictRequest(w, &response, msg)
	} else {
		m.writeBadRequest(w, &response, msg)
	}
	httpWriterSetContentType(w, "application/json")
	js, _ := json.Marshal(response)
	w.Write(js)
}

func (m *serve) getBaseReconfigure() actions.BaseReconfigure {
	//MW: What about skipAddressValidation???
	return actions.BaseReconfigure{
		ConfigsPath:   m.configsPath,
		InstanceName:  os.Getenv("PROXY_INSTANCE_NAME"),
		TemplatesPath: m.templatesPath,
	}
}

func (m *serve) ReloadHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	params := new(reloadParams)
	decoder.Decode(params, req.Form)
	listenerAddr := ""
	response := Response{
		Status: "OK",
	}
	if params.FromListener {
		listenerAddr = m.listenerAddress
	}
	//MW: I've reconstructed original behavior. BUT.
	//shouldn't reload call ReloadServicesFromRegistry not just
	//reload in else, if so ReloadClusterConfig & ReloadServicesFromRegistry
	//could be enclosed in one method
	if len(listenerAddr) > 0 {
		fetch := actions.NewFetch(m.getBaseReconfigure())
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

func (m *serve) RemoveHandler(w http.ResponseWriter, req *http.Request) {
	req.ParseForm()
	params := new(removeParams)
	decoder.Decode(params, req.Form)
	header := http.StatusOK
	response := Response{
		Status:      "OK",
		ServiceName: params.ServiceName,
	}
	if params.Distribute {
		response.Distribute = params.Distribute
		response.Message = distributed
	}
	if len(params.ServiceName) == 0 {
		response.Status = "NOK"
		response.Message = "The serviceName query is mandatory"
		header = http.StatusBadRequest
	} else if params.Distribute {
		if status, err := sendDistributeRequests(req, m.port, m.serviceName); err != nil || status >= 300 {
			response.Status = "NOK"
			response.Message = err.Error()
			header = http.StatusInternalServerError
		}
	} else {
		logPrintf("Processing remove request %s", req.URL.Path)
		action := actions.NewRemove(
			params.ServiceName,
			params.AclName,
			m.configsPath,
			m.templatesPath,
			m.serviceName,
		)
		action.Execute([]string{})
	}
	w.WriteHeader(header)
	httpWriterSetContentType(w, "application/json")
	js, _ := json.Marshal(response)
	w.Write(js)
}

func (m *serve) GetServicesFromEnvVars() *[]proxy.Service {
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

func (m *serve) getServiceFromEnvVars(prefix string) (proxy.Service, error) {
	var s proxy.Service
	envconfig.Process(prefix, &s)
	s.ServiceDomainAlgo = os.Getenv(prefix + "_SERVICE_DOMAIN_ALGO")
	if len(s.ServiceName) == 0 {
		return proxy.Service{}, fmt.Errorf("%s_SERVICE_NAME is not set", prefix)
	}
	sd := []proxy.ServiceDest{}
	path := getSliceFromString(os.Getenv(prefix + "_SERVICE_PATH"))
	port := os.Getenv(prefix + "_PORT")
	srcPort, _ := strconv.Atoi(os.Getenv(prefix + "_SRC_PORT"))
	reqMode := os.Getenv(prefix + "_REQ_MODE")
	domain := getSliceFromString(os.Getenv(prefix + "_SERVICE_DOMAIN"))
	// TODO: Remove.
	// It is a temporary workaround to maintain compatibility with the deprecated serviceDomainMatchAll parameter (since July 2017).
	if len(s.ServiceDomainAlgo) == 0 && strings.EqualFold(os.Getenv(prefix+"_SERVICE_DOMAIN_MATCH_ALL"), "true") {
		s.ServiceDomainAlgo = "hdr_dom(host)"
	}
	if len(reqMode) == 0 {
		reqMode = getSecretOrEnvVar("DEFAULT_PROTOCOL", "http")
	}
	httpsOnly, _ := strconv.ParseBool(os.Getenv(prefix + "_HTTPS_ONLY"))
	httpsRedirectCode := os.Getenv(prefix + "_HTTPS_REDIRECT_CODE")
	globalOutboundHostname := os.Getenv(prefix + "_OUTBOUND_HOSTNAME")
	reqPathSearchReplace := os.Getenv(prefix + "_REQ_PATH_SEARCH_REPLACE")
	reqPathSearchReplaceFormatted := []string{}
	if len(reqPathSearchReplace) > 0 {
		reqPathSearchReplaceFormatted = strings.Split(reqPathSearchReplace, ":")
	}
	allowedMethods := getSliceFromString(os.Getenv(prefix + "_ALLOWED_METHODS"))
	deniedMethods := getSliceFromString(os.Getenv(prefix + "_DENIED_METHODS"))
	redirectFromDomain := getSliceFromString(os.Getenv(prefix + "_REDIRECT_FROM_DOMAIN"))
	servicePathExclude := getSliceFromString(os.Getenv(prefix + "_SERVICE_PATH_EXCLUDE"))
	verifyClientSsl, _ := strconv.ParseBool(os.Getenv(prefix + "_VERIFY_CLIENT_SSL"))
	denyHTTP, _ := strconv.ParseBool(os.Getenv(prefix + "_DENY_HTTP"))
	ignoreAuthorization, _ := strconv.ParseBool(os.Getenv(prefix + "_IGNORE_AUTHORIZATION"))

	if len(path) > 0 || len(port) > 0 {
		sd = append(
			sd,
			proxy.ServiceDest{
				AllowedMethods:                allowedMethods,
				DeniedMethods:                 deniedMethods,
				DenyHttp:                      denyHTTP,
				HttpsOnly:                     httpsOnly,
				HttpsRedirectCode:             httpsRedirectCode,
				IgnoreAuthorization:           ignoreAuthorization,
				OutboundHostname:              globalOutboundHostname,
				Port:                          port,
				RedirectFromDomain:            redirectFromDomain,
				ReqMode:                       reqMode,
				ReqPathSearchReplace:          reqPathSearchReplace,
				ReqPathSearchReplaceFormatted: reqPathSearchReplaceFormatted,
				ServiceDomain:                 domain,
				ServicePath:                   path,
				ServicePathExclude:            servicePathExclude,
				SrcPort:                       srcPort,
				VerifyClientSsl:               verifyClientSsl,
			},
		)
	}
	for i := 1; i <= 10; i++ {
		domain := getSliceFromString(os.Getenv(fmt.Sprintf("%s_SERVICE_DOMAIN_%d", prefix, i)))
		port := os.Getenv(fmt.Sprintf("%s_PORT_%d", prefix, i))
		path := getSliceFromString(os.Getenv(fmt.Sprintf("%s_SERVICE_PATH_%d", prefix, i)))
		reqMode := os.Getenv(fmt.Sprintf("%s_REQ_MODE_%d", prefix, i))
		reqPathSearchReplace := os.Getenv(fmt.Sprintf("%s_REQ_PATH_SEARCH_REPLACE_%d", prefix, i))
		reqPathSearchReplaceFormatted := []string{}
		if len(reqPathSearchReplace) > 0 {
			reqPathSearchReplaceFormatted = strings.Split(reqPathSearchReplace, ":")
		}
		httpsOnly, _ := strconv.ParseBool(os.Getenv(fmt.Sprintf("%s_HTTPS_ONLY_%d", prefix, i)))
		httpsRedirectCode := os.Getenv(fmt.Sprintf("%s_HTTPS_REDIRECT_CODE_%d", prefix, i))
		if len(reqMode) == 0 {
			reqMode = "http"
		}
		srcPort, _ := strconv.Atoi(os.Getenv(fmt.Sprintf("%s_SRC_PORT_%d", prefix, i)))
		allowedMethods := getSliceFromString(os.Getenv(fmt.Sprintf("%s_ALLOWED_METHODS_%d", prefix, i)))
		deniedMethods := getSliceFromString(os.Getenv(fmt.Sprintf("%s_DENIED_METHODS_%d", prefix, i)))
		redirectFromDomain := getSliceFromString(os.Getenv(fmt.Sprintf("%s_REDIRECT_FROM_DOMAIN_%d", prefix, i)))
		servicePathExclude := getSliceFromString(os.Getenv(fmt.Sprintf("%s_SERVICE_PATH_EXCLUDE_%d", prefix, i)))
		verifyClientSsl, _ := strconv.ParseBool(os.Getenv(fmt.Sprintf("%s_VERIFY_CLIENT_SSL_%d", prefix, i)))
		denyHTTP, _ := strconv.ParseBool(os.Getenv(fmt.Sprintf("%s_DENY_HTTP_%d", prefix, i)))
		ignoreAuthorization, _ := strconv.ParseBool(os.Getenv(fmt.Sprintf("%s_IGNORE_AUTHORIZATION_%d", prefix, i)))
		if len(path) > 0 && len(port) > 0 {
			outboundHostname := os.Getenv(fmt.Sprintf("%s_OUTBOUND_HOSTNAME_%d", prefix, i))
			if len(outboundHostname) == 0 {
				outboundHostname = globalOutboundHostname
			}
			sd = append(
				sd,
				proxy.ServiceDest{
					AllowedMethods:                allowedMethods,
					DeniedMethods:                 deniedMethods,
					DenyHttp:                      denyHTTP,
					HttpsOnly:                     httpsOnly,
					HttpsRedirectCode:             httpsRedirectCode,
					IgnoreAuthorization:           ignoreAuthorization,
					OutboundHostname:              outboundHostname,
					Port:                          port,
					RedirectFromDomain:            redirectFromDomain,
					ReqPathSearchReplace:          reqPathSearchReplace,
					ReqPathSearchReplaceFormatted: reqPathSearchReplaceFormatted,
					ServiceDomain:                 domain,
					SrcPort:                       srcPort,
					ServicePath:                   path,
					ServicePathExclude:            servicePathExclude,
					ReqMode:                       reqMode,
					VerifyClientSsl:               verifyClientSsl,
				},
			)
		} else {
			break
		}
	}
	s.ServiceDest = sd
	return s, nil
}

func (m *serve) writeBadRequest(w http.ResponseWriter, resp *Response, msg string) {
	resp.Status = "NOK"
	resp.Message = msg
	w.WriteHeader(http.StatusBadRequest)
}

func (m *serve) writeConflictRequest(w http.ResponseWriter, resp *Response, msg string) {
	resp.Status = "NOK"
	resp.Message = msg
	w.WriteHeader(http.StatusConflict)
}

func (m *serve) writeInternalServerError(w http.ResponseWriter, resp *Response, msg string) {
	resp.Status = "NOK"
	resp.Message = msg
	w.WriteHeader(http.StatusInternalServerError)
}

func (m *serve) hasPort(sd []proxy.ServiceDest) bool {
	return len(sd) > 0 && len(sd[0].Port) > 0
}

func getSliceFromString(input string) []string {
	separator := os.Getenv("SEPARATOR")
	value := []string{}
	if len(input) > 0 {
		value = strings.Split(input, separator)
	}
	return value
}
