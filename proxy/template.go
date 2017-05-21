package proxy

import (
	"fmt"
	"strings"
	"os"
)

// TODO: Change to private when actions.GetTemplates is moved to the proxy package
func GetBackTemplate(sr *Service, mode string) string {
	back := getBackTemplateProtocol("http", mode, sr)
	if sr.HttpsPort > 0 {
		back += fmt.Sprintf(
			`
%s`,
			getBackTemplateProtocol("https", mode, sr))
	}
	return back
}

func getBackTemplateProtocol(protocol, mode string, sr *Service) string {
	prefix := ""
	if strings.EqualFold(protocol, "https") {
		prefix = "https-"
	}
	for i := range sr.ServiceDest {
		if strings.EqualFold(sr.ServiceDest[i].ReqMode, "sni") {
			sr.ServiceDest[i].ReqModeFormatted = "tcp"
		} else {
			sr.ServiceDest[i].ReqModeFormatted = sr.ServiceDest[i].ReqMode
		}
	}
	tmpl := fmt.Sprintf(`{{range .ServiceDest}}
backend %s{{$.ServiceName}}-be{{.Port}}
    mode {{.ReqModeFormatted}}`,
		prefix,
	)
	if len(sr.ConnectionMode) > 0 {
		tmpl += `
    option {{$.ConnectionMode}}`
	}
	if strings.EqualFold(os.Getenv("DEBUG"), "true") {
		tmpl += `
    log global`
	}
	tmpl += getHeaders(sr)
	tmpl += `{{- if ne $.TimeoutServer ""}}
    timeout server {{$.TimeoutServer}}s
    {{- end}}
    {{- if ne $.TimeoutTunnel ""}}
    timeout tunnel {{$.TimeoutTunnel}}s
    {{- end}}
	{{- if (and (ne $.ReqPathSearch "") (ne $.ReqPathReplace ""))}}
    http-request set-path %[path,regsub({{$.ReqPathSearch}},{{$.ReqPathReplace}})]
    {{- end}}`
	// TODO: Deprecated (dec. 2016).
	if len(sr.ReqRepSearch) > 0 && len(sr.ReqRepReplace) > 0 {
		tmpl += `
    reqrep {{$.ReqRepSearch}}     {{$.ReqRepReplace}}`
	}
	tmpl += getServerTemplate(protocol, mode)
	tmpl += getUsersTemplate(sr.Users)
	tmpl += `
    {{- end}}
    {{- if ne $.BackendExtra ""}}
    {{ $.BackendExtra }}
    {{- end}}`
	return tmpl
}

func getHeaders(sr *Service) string {
	tmpl := ""
	if sr.XForwardedProto {
		tmpl += `
    http-request add-header X-Forwarded-Proto https if { ssl_fc }`
	}
	for _, header := range sr.AddReqHeader {
		tmpl += fmt.Sprintf(`
    http-request add-header %s`,
			header,
		)
	}
	for _, header := range sr.SetReqHeader {
		tmpl += fmt.Sprintf(`
    http-request set-header %s`,
			header,
		)
	}
	for _, header := range sr.AddResHeader {
		tmpl += fmt.Sprintf(`
    http-response add-header %s`,
			header,
		)
	}
	for _, header := range sr.SetResHeader {
		tmpl += fmt.Sprintf(`
    http-response set-header %s`,
			header,
		)
	}
	for _, header := range sr.DelReqHeader {
		tmpl += fmt.Sprintf(`
    http-request del-header %s`,
			header,
		)
	}
	for _, header := range sr.DelResHeader {
		tmpl += fmt.Sprintf(`
    http-response del-header %s`,
			header,
		)
	}
	return tmpl
}

func getServerTemplate(protocol, mode string) string {
	if strings.EqualFold(mode, "service") || strings.EqualFold(mode, "swarm") {
		tmpl := `
    {{- if eq .VerifyClientSsl true}}
    acl valid_client_cert_{{$.ServiceName}}{{.Port}} ssl_c_used ssl_c_verify 0
    http-request deny unless valid_client_cert_{{$.ServiceName}}{{.Port}}
    {{- end}}`
		if strings.EqualFold(protocol, "https") {
			return tmpl + `
    server {{$.ServiceName}} {{$.Host}}:{{$.HttpsPort}}{{if eq $.CheckResolvers true}} check resolvers docker{{end}}{{if eq $.SslVerifyNone true}} ssl verify none{{end}}`
		}
		return tmpl + `
    server {{$.ServiceName}} {{$.Host}}:{{.Port}}{{if eq $.CheckResolvers true}} check resolvers docker{{end}}{{if eq $.SslVerifyNone true}} ssl verify none{{end}}`
	}
	// It's Consul
	return `
    {{"{{"}}range $i, $e := service "{{$.FullServiceName}}" "any"{{"}}"}}
    server {{"{{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}}"}}
    {{"{{end}}"}}`
}

func getUsersTemplate(users []User) string {
	if len(users) > 0 {
		return `
    acl {{$.ServiceName}}UsersAcl http_auth({{$.ServiceName}}Users)
    http-request auth realm {{$.ServiceName}}Realm if !{{$.ServiceName}}UsersAcl
    http-request del-header Authorization`
	} else if len(GetSecretOrEnvVar("USERS", "")) > 0 {
		return `
    acl defaultUsersAcl http_auth(defaultUsers)
    http-request auth realm defaultRealm if !defaultUsersAcl
    http-request del-header Authorization`
	}
	return ""
}

