package proxy

import (
	"fmt"
	"os"
	"strings"
)

// TODO: Change to private function when actions.GetTemplates is moved to the proxy package
// TODO: Create a single string for the template
// TODO: Unify HTTP and HTTPS into a single string
// TODO: Move to a file
func GetBackTemplate(sr *Service, mode string) string {
	sr.ProxyMode = strings.ToLower(mode)
	for i := range sr.ServiceDest {
		if strings.EqualFold(sr.ServiceDest[i].ReqMode, "sni") {
			sr.ServiceDest[i].ReqModeFormatted = "tcp"
		} else {
			sr.ServiceDest[i].ReqModeFormatted = sr.ServiceDest[i].ReqMode
		}
	}
	if len(GetSecretOrEnvVar("USERS", "")) > 0 {
		sr.UseGlobalUsers = true
	}
	if strings.EqualFold(os.Getenv("DEBUG"), "true") {
		sr.Debug = true
	}

	// HTTP
	tmpl := `{{range .ServiceDest}}
backend {{$.ServiceName}}-be{{.Port}}
    mode {{.ReqModeFormatted}}
        {{- if ne $.ConnectionMode ""}}
    option {{$.ConnectionMode}}
        {{- end}}
        {{- if $.Debug}}
    log global
        {{- end}}`
	tmpl += getHeaders(sr)
	tmpl += `{{- if ne $.TimeoutServer ""}}
    timeout server {{$.TimeoutServer}}s
        {{- end}}
        {{- if ne $.TimeoutTunnel ""}}
    timeout tunnel {{$.TimeoutTunnel}}s
        {{- end}}
        {{- if ne $.ReqPathSearch ""}}
    http-request set-path %[path,regsub({{$.ReqPathSearch}},{{$.ReqPathReplace}})]
        {{- end}}
        {{- if or (eq $.ProxyMode "service") (eq $.ProxyMode "swarm")}}
            {{- if eq .VerifyClientSsl true}}
    acl valid_client_cert_{{$.ServiceName}}{{.Port}} ssl_c_used ssl_c_verify 0
    http-request deny unless valid_client_cert_{{$.ServiceName}}{{.Port}}
            {{- end}}
            {{- if $.HttpsOnly}}
    redirect scheme https if !{ ssl_fc }
            {{- end}}
    server {{$.ServiceName}} {{$.Host}}:{{.Port}}{{if eq $.CheckResolvers true}} check resolvers docker{{end}}{{if eq $.SslVerifyNone true}} ssl verify none{{end}}
        {{- /* TODO: It's Consul and it's deprecated. Remove it. */}}
        {{- else}}
    {{"{{"}}range $i, $e := service "{{$.FullServiceName}}" "any"{{"}}"}}
    server {{"{{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}}"}}
    {{"{{end}}"}}
        {{- end}}
        {{- if not .IgnoreAuthorization}}
            {{- if and ($.Users) (not .IgnoreAuthorization)}}
    acl {{$.ServiceName}}UsersAcl http_auth({{$.ServiceName}}Users)
    http-request auth realm {{$.ServiceName}}Realm if !{{$.ServiceName}}UsersAcl
            {{- end}}
            {{- if $.UseGlobalUsers}}
    acl defaultUsersAcl http_auth(defaultUsers)
    http-request auth realm defaultRealm if !defaultUsersAcl
            {{- end}}
            {{- if or ($.Users) ($.UseGlobalUsers)}}
    http-request del-header Authorization
            {{- end}}
        {{- end}}
    {{- end}}
    {{- if ne $.BackendExtra ""}}
    {{ $.BackendExtra }}
    {{- end}}
    {{- if gt .HttpsPort 0}}{{range .ServiceDest}}
backend https-{{$.ServiceName}}-be{{.Port}}
    mode {{.ReqModeFormatted}}
        {{- if ne $.ConnectionMode ""}}
    option {{$.ConnectionMode}}
        {{- end}}
        {{- if $.Debug}}
    log global
        {{- end}}`
	tmpl += getHeaders(sr)
	tmpl += `{{- if ne $.TimeoutServer ""}}
    timeout server {{$.TimeoutServer}}s
        {{- end}}
        {{- if ne $.TimeoutTunnel ""}}
    timeout tunnel {{$.TimeoutTunnel}}s
        {{- end}}
        {{- if ne $.ReqPathSearch ""}}
    http-request set-path %[path,regsub({{$.ReqPathSearch}},{{$.ReqPathReplace}})]
        {{- end}}
        {{- if or (eq $.ProxyMode "service") (eq $.ProxyMode "swarm")}}
            {{- if eq .VerifyClientSsl true}}
    acl valid_client_cert_{{$.ServiceName}}{{.Port}} ssl_c_used ssl_c_verify 0
    http-request deny unless valid_client_cert_{{$.ServiceName}}{{.Port}}
            {{- end}}
    server {{$.ServiceName}} {{$.Host}}:{{$.HttpsPort}}{{if eq $.CheckResolvers true}} check resolvers docker{{end}}{{if eq $.SslVerifyNone true}} ssl verify none{{end}}
        {{- /* TODO: It's Consul and it's deprecated. Remove it. */}}
        {{- else}}
    {{"{{"}}range $i, $e := service "{{$.FullServiceName}}" "any"{{"}}"}}
    server {{"{{$e.Node}}_{{$i}}_{{$e.Port}} {{$e.Address}}:{{$e.Port}}"}}
    {{"{{end}}"}}
        {{- end}}
        {{- if not .IgnoreAuthorization}}
            {{- if $.Users}}
    acl {{$.ServiceName}}UsersAcl http_auth({{$.ServiceName}}Users)
    http-request auth realm {{$.ServiceName}}Realm if !{{$.ServiceName}}UsersAcl
            {{- end}}
            {{- if $.UseGlobalUsers}}
    acl defaultUsersAcl http_auth(defaultUsers)
    http-request auth realm defaultRealm if !defaultUsersAcl
            {{- end}}
            {{- if or ($.Users) ($.UseGlobalUsers)}}
    http-request del-header Authorization
            {{- end}}
        {{- end}}
    {{- end}}
    {{- if ne $.BackendExtra ""}}
    {{ $.BackendExtra }}
    {{- end}}
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
