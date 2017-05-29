package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"

	"../proxy"
)

var mu = &sync.Mutex{}

type Certer interface {
	Put(w http.ResponseWriter, req *http.Request) (string, error)
	PutCert(certName string, certContent []byte) (string, error)
	GetAll(w http.ResponseWriter, req *http.Request) (CertResponse, error)
	Init() error
}

type Cert struct {
	ServicePort      string
	ProxyServiceName string
	CertsDir         string
	CertContent      string
}

type CertResponse struct {
	Status  string
	Message string
	Certs   []Cert
}

func (m *Cert) GetAll(w http.ResponseWriter, req *http.Request) (CertResponse, error) {
	pCerts := proxy.Instance.GetCerts()
	certs := []Cert{}
	for path, content := range pCerts {
		if !strings.HasPrefix(path, "/run/secrets") {
			parts := strings.Split(path, "/")
			cert := Cert{CertContent: content}
			nameIndex := len(parts) - 1
			for index, part := range parts {
				if index == nameIndex {
					cert.ProxyServiceName = part
				} else if len(part) > 0 {
					cert.CertsDir += "/" + part
				}
			}
			certs = append(certs, cert)
		}
	}
	msg := CertResponse{Status: "OK", Message: "", Certs: certs}
	m.writeOK(w, msg)
	return msg, nil
}

func (m *Cert) PutCert(certName string, certContent []byte) (string, error) {
	return m.writeFile(certName, certContent)
}

func (m *Cert) Put(w http.ResponseWriter, req *http.Request) (string, error) {
	distribute, _ := strconv.ParseBool(req.URL.Query().Get("distribute"))
	if distribute {
		return "", m.sendDistributeRequests(w, req)
	}
	certName, certContent, err := m.getCertFromRequest(w, req)
	if err != nil { // TODO: Test
		m.writeError(w, err)
		return "", err
	}

	path, err := m.PutCert(certName, certContent)
	if err != nil {
		m.writeError(w, err)
		return "", err
	}

	proxy.Instance.CreateConfigFromTemplates()
	proxy.Instance.Reload()

	msg := CertResponse{Status: "OK", Message: ""}
	m.writeOK(w, msg)

	return path, nil
}

func (m *Cert) Init() error {
	dns := fmt.Sprintf("tasks.%s", m.ProxyServiceName)
	client := &http.Client{}
	ips, err := lookupHost(dns)
	if err != nil {
		return err
	}
	certs := []Cert{}
	for _, ip := range ips {
		hostPort := ip
		if !strings.Contains(ip, ":") { // TODO: Test
			hostPort = net.JoinHostPort(ip, m.ServicePort)
		}
		addr := fmt.Sprintf("http://%s/v1/docker-flow-proxy/certs", hostPort)
		req, _ := http.NewRequest("GET", addr, nil)
		if resp, err := client.Do(req); err == nil {
			defer resp.Body.Close()
			body, _ := ioutil.ReadAll(resp.Body)
			data := CertResponse{}
			json.Unmarshal(body, &data)
			if len(data.Certs) > len(certs) {
				certs = data.Certs
			}
		}
	}
	if len(certs) > 0 {
		for _, cert := range certs {
			m.writeFile(cert.ProxyServiceName, []byte(cert.CertContent))
		}
		proxy.Instance.CreateConfigFromTemplates()
		proxy.Instance.Reload()
	}
	return nil
}

func (m *Cert) getCertFromRequest(w http.ResponseWriter, req *http.Request) (certName string, certContent []byte, err error) {
	certName = req.URL.Query().Get("certName")
	if len(certName) == 0 { // TODO: Test
		err := fmt.Errorf("Query parameter certName is mandatory")
		return "", []byte{}, err
	}
	defer func() { req.Body.Close() }()
	certContent, err = ioutil.ReadAll(req.Body)
	if err != nil {
		return "", []byte{}, err
	} else if len(certContent) == 0 {
		err := fmt.Errorf("Body is empty")
		return "", []byte{}, err
	}
	return certName, certContent, nil
}

func (m *Cert) sendDistributeRequests(w http.ResponseWriter, req *http.Request) error {
	_, port, err := net.SplitHostPort(req.URL.Host)
	if err != nil {
		port = "8080"
	}
	status, err := sendDistributeRequests(req, port, m.ProxyServiceName)
	if err != nil {
		return m.writeError(w, err)
	} else if status >= 300 {
		msg := fmt.Sprintf("Distribution request failed with status %d", status)
		return m.writeError(w, fmt.Errorf(msg))
	}
	return nil
}

func (m *Cert) writeFile(certName string, certContent []byte) (path string, err error) {
	mu.Lock()
	defer mu.Unlock()
	f, err := os.Create(fmt.Sprintf("%s/%s", m.CertsDir, certName))
	if err != nil {
		return "", err
	}
	f.Write(certContent)
	path, _ = filepath.Abs(fmt.Sprintf("%s/%s", m.CertsDir, certName))
	return path, nil
}

func (m *Cert) writeOK(w http.ResponseWriter, msg interface{}) {
	httpWriterSetContentType(w, "application/json")
	w.WriteHeader(http.StatusOK)
	js, _ := json.Marshal(msg)
	w.Write(js)
}

func (m *Cert) writeError(w http.ResponseWriter, err error) error {
	w.WriteHeader(http.StatusBadRequest)
	js, _ := json.Marshal(CertResponse{
		Status:  "NOK",
		Message: err.Error(),
	})
	w.Write(js)
	return err
}

func NewCert(certsDir string) *Cert {
	return &Cert{
		CertsDir:         certsDir,
		ProxyServiceName: os.Getenv("SERVICE_NAME"),
		ServicePort:      "8080",
	}
}
