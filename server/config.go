package server

import (
	"../proxy"
	"encoding/json"
	"net/http"
	"strings"
)

// Configer defines the interface that must be implemented by any struct that deals with proxy configuration.
type Configer interface {
	Get(w http.ResponseWriter, req *http.Request)
}

type config struct{}

// NewConfig returns a new instance of the Configer interface
func NewConfig() Configer {
	return &config{}
}

// Get writes proxy configuration to the ResponseWriter.
// If query parameter `type` is set to `json`, the response will contain the struct with all the services.
// Any other `type` returns the configuration in `text` format.
func (m *config) Get(w http.ResponseWriter, req *http.Request) {
	status := http.StatusOK
	typeParam := req.URL.Query().Get("type")
	contentType := "text/html"
	body := []byte{}
	if strings.EqualFold(typeParam, "json") {
		contentType = "application/json"
		services := proxy.Instance.GetServices()
		body, _ = json.Marshal(services)
	} else {
		out, err := proxy.Instance.ReadConfig()
		if err != nil {
			status = http.StatusInternalServerError
		}
		body = []byte(out)
	}
	httpWriterSetContentType(w, contentType)
	w.WriteHeader(status)
	w.Write(body)
}
