package proxy

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
)

func getFrontTemplate(s Service) string {
	// TODO: Change domain_{{$.AclName}} to a unique value
	tmplString := `
{{- range $sd := .ServiceDest}}
    {{- if eq .ReqMode "http"}}
        {{- if ne .Port ""}}
    acl url_{{$.AclName}}{{.Port}}_{{.Index}}{{range .ServicePath}} {{if eq $.PathType ""}}path_beg{{end}}{{if ne $.PathType ""}}{{$.PathType}}{{end}} {{.}}{{end}}{{.SrcPortAcl}}
        {{- end}}
        {{- $length := len .UserAgent.Value}}{{if gt $length 0}}
    acl user_agent_{{$.AclName}}_{{.UserAgent.AclName}}_{{.Index}} hdr_sub(User-Agent) -i{{range .UserAgent.Value}} {{.}}{{end}}
        {{- end}}
    {{- end}}
    {{- if .ServiceDomain}}
    acl domain_{{$.AclName}}{{.Port}}_{{.Index}} {{$.ServiceDomainAlgo}} -i{{range .ServiceDomain}} {{.}}{{end}}
    {{- end}}
    {{- if .ServiceHeader}}{{$skIndex := 0}}
        {{- range $key, $value := .ServiceHeader}}
    acl hdr_{{$.AclName}}{{$sd.Port}}_{{incIndex}} hdr({{$key}}) {{$value}}
        {{- end}}
    {{- end}}
{{- end}}
{{- if gt $.HttpsPort 0 }}
    acl http_{{.ServiceName}} src_port 80
    acl https_{{.ServiceName}} src_port 443
{{- end}}
{{- if $.RedirectWhenHttpProto}}
    {{- range .ServiceDest}}
        {{- if eq .ReqMode "http"}}
            {{- if ne .Port ""}}
    acl is_{{$.AclName}}_http hdr(X-Forwarded-Proto) http
    redirect scheme https if is_{{$.AclName}}_http url_{{$.AclName}}{{.Port}}_{{.Index}}{{if .ServiceDomain}} domain_{{$.AclName}}{{.Port}}_{{.Index}}{{end}}{{.SrcPortAclName}}
            {{- end}}
        {{- end}}
    {{- end}}
{{- end}}
{{- range $sd := .ServiceDest}}
    {{- if eq .ReqMode "http"}}{{- if ne .Port ""}}
    use_backend {{$.ServiceName}}-be{{.Port}}_{{.Index}} if url_{{$.AclName}}{{.Port}}_{{.Index}}{{if .ServiceDomain}} domain_{{$.AclName}}{{.Port}}_{{.Index}}{{end}}{{if .ServiceHeader}}{{resetIndex}}{{range $key, $value := .ServiceHeader}} hdr_{{$.AclName}}{{$sd.Port}}_{{incIndex}}{{end}}{{end}}{{.SrcPortAclName}}
	    {{- if gt $.HttpsPort 0 }} http_{{$.ServiceName}}
    use_backend https-{{$.ServiceName}}-be{{.Port}}_{{.Index}} if url_{{$.AclName}}{{.Port}}_{{.Index}}{{if .ServiceDomain}} domain_{{$.AclName}}{{.Port}}_{{.Index}}{{end}} https_{{$.ServiceName}}
        {{- end}}
    {{- $length := len .UserAgent.Value}}{{if gt $length 0}} user_agent_{{$.AclName}}_{{.UserAgent.AclName}}_{{.Index}}{{end}}
        {{- if $.IsDefaultBackend}}
    default_backend {{$.ServiceName}}-be{{.Port}}_{{$sd.Index}}
        {{- end}}
    {{- end}}{{- end}}
{{- end}}`
	return templateToString(tmplString, s)
}

func getFrontTemplateTcp(servicesByPort map[int]Services) string {
	tmpl := ""
	for _, services := range servicesByPort {
		sort.Sort(services)
		tmplString := `

{{$sd := (index . 0).ServiceDest}}{{$srcPort := (index $sd 0).SrcPort}}frontend tcpFE_{{$srcPort}}
    bind *:{{$srcPort}}
    mode tcp
	{{- if (index . 0).Debug}}{{$debugFormat := (index . 0).DebugFormat}}
    option tcplog
    log global
	    {{- if ne $debugFormat ""}}
    log-format {{$debugFormat}}
	    {{- end}}
	{{- end}}
    {{- range $s := .}}
        {{- range $sd := .ServiceDest}}
            {{- if $sd.ServiceDomain}}
    acl domain_{{$s.AclName}}{{.Port}}_{{$sd.Index}} {{$s.ServiceDomainAlgo}} -i{{range $sd.ServiceDomain}} {{.}}{{end}}
    use_backend {{$s.ServiceName}}-be{{$sd.Port}}_{{$sd.Index}} if domain_{{$s.AclName}}{{.Port}}_{{$sd.Index}}
            {{- end}}
            {{- if not $sd.ServiceDomain}}
    default_backend {{$s.ServiceName}}-be{{$sd.Port}}_{{$sd.Index}}
            {{- end}}
        {{- end}}
    {{- end}}`
		tmpl += templateToString(tmplString, services)
	}
	return tmpl
}

// GetBackTemplate returns template used to create a service backend
// TODO: Change to private function when actions.GetTemplates is moved to the proxy package
// TODO: Create a single string for the template
// TODO: Unify HTTP and HTTPS into a single string
// TODO: Move to a file
func GetBackTemplate(sr *Service) string {
	for i := range sr.ServiceDest {
		if strings.EqualFold(sr.ServiceDest[i].ReqMode, "sni") {
			sr.ServiceDest[i].ReqModeFormatted = "tcp"
		} else {
			sr.ServiceDest[i].ReqModeFormatted = sr.ServiceDest[i].ReqMode
		}
	}
	if len(getSecretOrEnvVar("USERS", "")) > 0 {
		sr.UseGlobalUsers = true
	}
	if strings.EqualFold(os.Getenv("DEBUG"), "true") {
		sr.Debug = true
	}

	// HTTP
	tmpl := `{{- range $sd := .ServiceDest}}
backend {{$.ServiceName}}-be{{.Port}}_{{.Index}}
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
		{{- if eq .VerifyClientSsl true}}
    acl valid_client_cert_{{$.ServiceName}}{{.Port}} ssl_c_used ssl_c_verify 0
    http-request deny unless valid_client_cert_{{$.ServiceName}}{{.Port}}
		{{- end}}
		{{- if .AllowedMethods}}
    acl valid_allowed_method method{{range .AllowedMethods}} {{.}}{{end}}
    http-request deny unless valid_allowed_method
		{{- end}}
		{{- if .DeniedMethods}}
    acl valid_denied_method method{{range .DeniedMethods}} {{.}}{{end}}
    http-request deny if valid_denied_method
		{{- end}}
		{{- if .DenyHttp}}
    http-request deny if !{ ssl_fc }
		{{- end}}
		{{- if .HttpsOnly}}
    redirect scheme https if !{ ssl_fc }
		{{- end}}
		{{- if eq $.SessionType "sticky-server"}}
    balance roundrobin
    cookie {{$.ServiceName}} insert indirect nocache
		{{- end}}
		{{- range $i, $t := $.Tasks}}
    server {{$.ServiceName}}_{{$i}} {{$t}}:{{$sd.Port}} check cookie {{$.ServiceName}}_{{$i}}
		{{- end}}
		{{- if not $.Tasks}}
    server {{$.ServiceName}} {{$.Host}}:{{$sd.Port}}{{if eq $.CheckResolvers true}} check resolvers docker{{end}}{{if eq $.SslVerifyNone true}} ssl verify none{{end}}
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
    {{- if gt .HttpsPort 0}}
        {{- range $sd := .ServiceDest}}
backend https-{{$.ServiceName}}-be{{.Port}}_{{.Index}}
    mode {{.ReqModeFormatted}}
            {{- if ne $.ConnectionMode ""}}
    option {{$.ConnectionMode}}
            {{- end}}
            {{- if $.Debug}}
    log global
            {{- end}}`
	tmpl += getHeaders(sr)
	tmpl += `
	        {{- if ne $.TimeoutServer ""}}
    timeout server {{$.TimeoutServer}}s
            {{- end}}
            {{- if ne $.TimeoutTunnel ""}}
    timeout tunnel {{$.TimeoutTunnel}}s
            {{- end}}
            {{- if ne $.ReqPathSearch ""}}
    http-request set-path %[path,regsub({{$.ReqPathSearch}},{{$.ReqPathReplace}})]
            {{- end}}
		    {{- if eq .VerifyClientSsl true}}
    acl valid_client_cert_{{$.ServiceName}}{{.Port}} ssl_c_used ssl_c_verify 0
    http-request deny unless valid_client_cert_{{$.ServiceName}}{{.Port}}
		    {{- end}}
		    {{- if .AllowedMethods}}
    acl valid_allowed_method method{{range .AllowedMethods}} {{.}}{{end}}
    http-request deny unless valid_allowed_method
		    {{- end}}
		    {{- if .DeniedMethods}}
    acl valid_denied_method method{{range .DeniedMethods}} {{.}}{{end}}
    http-request deny if valid_denied_method
		    {{- end}}
		    {{- if eq $.SessionType "sticky-server"}}
    balance roundrobin
    cookie {{$.ServiceName}} insert indirect nocache
		    {{- end}}
		    {{- range $i, $t := $.Tasks}}
    server {{$.ServiceName}}_{{$i}} {{$t}}:{{$.HttpsPort}} check cookie {{$.ServiceName}}_{{$i}}
		    {{- end}}
		    {{- if not $.Tasks}}
    server {{$.ServiceName}} {{$.Host}}:{{$.HttpsPort}}{{if eq $.CheckResolvers true}} check resolvers docker{{end}}{{if eq $.SslVerifyNone true}} ssl verify none{{end}}
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

func templateToString(templateString string, data interface{}) string {
	i := -1
	funcMap := template.FuncMap{
		"resetIndex": func() string {
			i = -1
			return ""
		},
		"incIndex": func() int {
			i += 1
			return i
		},
	}
	tmpl, _ := template.New("template").Funcs(funcMap).Parse(templateString)
	var b bytes.Buffer
	tmpl.Execute(&b, data)
	return b.String()
}

func putDomainAlgo(s *Service) {
	if len(s.ServiceDomainAlgo) == 0 {
		s.ServiceDomainAlgo = os.Getenv("SERVICE_DOMAIN_ALGO")
	}
	for _, sd := range s.ServiceDest {
		for i, domain := range sd.ServiceDomain {
			if strings.HasPrefix(domain, "*") {
				sd.ServiceDomain[i] = strings.Trim(domain, "*")
				s.ServiceDomainAlgo = "hdr_end(host)"
				return
			}
		}
	}
}
