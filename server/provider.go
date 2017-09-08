package server

import (
	"../proxy"
	"net/http"
)

// HttpRequestParameterProvider defines structure used to convert HTTP parameters into proxy.Service
type HttpRequestParameterProvider struct {
	*http.Request
}

// Fill converts HTTP parameters into proxy.Service struct
func (p *HttpRequestParameterProvider) Fill(service *proxy.Service) {
	p.ParseForm()
	decoder.Decode(service, p.Form)
}

// GetString returns parameter value
func (p *HttpRequestParameterProvider) GetString(name string) string {
	return p.URL.Query().Get(name)
}
