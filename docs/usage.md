# Usage

*Docker Flow Proxy* can be controlled by sending HTTP requests or through Docker Service labels when combined with [Docker Flow Swarm Listener](http://swarmlistener.dockerflow.com/).

## Reconfigure

> Reconfigures the proxy

The proxy can be reconfigured to use request mode *http* or *tcp*. The default value of the request mode is *http* and can be changed with the parameter `reqMode`.

### General Query Parameters

The following query parameters can be used to send a *reconfigure* request to *Docker Flow Proxy*. They apply to any request mode and should be added to the base address **[PROXY_IP]:[PROXY_PORT]/v1/docker-flow-proxy/reconfigure**. They apply to any `reqMode`.

|Query          |Description                                                                               |Default|
|---------------|------------------------------------------------------------------------------------------|-------|
|aclName        |ACLs are ordered alphabetically by their names. If not specified, serviceName is used instead.<br>**Example:** `05-go-demo-acl`| |
|addReqHeader   |Additional headers that will be added to the request before forwarding it to the service. Multiple headers should be separated with comma (`,`). Please consult [Add a header to the request](https://www.haproxy.com/doc/aloha/7.0/haproxy/http_rewriting.html#add-a-header-to-the-request) for more info.<br>**Example:** `X-Forwarded-Port %[dst_port],X-Forwarded-Ssl on if { ssl_fc }`| |
|addResHeader   |Additional headers that will be added to the response before forwarding it to the client. Multiple headers should be separated with comma (`,`). Please consult [Add a header to the response](https://www.haproxy.com/doc/aloha/7.0/haproxy/http_rewriting.html#rewriting-http-responses) for more info.<br>**Example:** `X-Via %[env(HOSTNAME)],Server haproxy`| |
|backendExtra   |Additional configuration that will be added to the bottom of the service backend          |       |
|connectionMode |HAProxy supports 5 connection modes.<br><br>`http-keep-alive`: all requests and responses are processed.<br>`http-tunnel`: only the first request and response are processed, everything else is forwarded with no analysis.<br>`httpclose`: tunnel with "Connection: close" added in both directions.<br>`http-server-close`: the server-facing connection is closed after the response.<br>`forceclose`: the connection is actively closed after end of response.<br><br>In general, it is preferred to use `http-server-close` with application servers, and some static servers might benefit from `http-keep-alive`.<br>Connection mode is restricted to HTTP mode only. If specified, connection mode will be applied to the backend section.<br>**Example:** http-keep-alive| |
|delReqHeader   |Additional headers that will be deleted in the request before forwarding it to the service. Multiple headers should be separated with comma (`,`). Please consult [Delete a header in the request](https://www.haproxy.com/doc/aloha/7.0/haproxy/http_rewriting.html#delete-a-header-in-the-request) for more info.<br>**Example:** `X-Forwarded-For,Cookie`| |
|delResHeader   |Additional headers that will be deleted in the response before forwarding it to the client. Multiple headers should be separated with comma (`,`). Please consult [Delete a header in the response](https://www.haproxy.com/doc/aloha/7.0/haproxy/http_rewriting.html#delete-a-header-in-the-response) for more info.<br>**Example:** `X-Varnish,X-Cache`| |
|distribute   |Whether to distribute a request to all the instances of the proxy. Used only in the *swarm* mode.<br>**Example:** `true`|false|
|httpsPort      |The internal HTTPS port of a service that should be reconfigured. The port is used only in the `swarm` mode. If not specified, the `port` parameter will be used instead.<br>**Example:** `443`| |
|ignoreAuthorization|If set to true, the service destination will not require authorization. The parameter must be prefixed with the index of the service destion that should be excluded from authorization.<br>**Example:** true|false|
|isDefaultBackend  |If set to true, the service will be set to the default_backend rule, meaning it will catch all requests not matching any other rules.<br>**Example:** `true`|false|
|port           |The internal port of a service that should be reconfigured. The port is used only in the `swarm` mode. The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `port.1`, `port.2`, and so on). This field is **mandatory** when running in `swarm` or `service` mode.<br>**Example:** `8080`| |
|reqMode        |The request mode. The proxy should be able to work with any mode supported by HAProxy. However, actively supported and tested modes are `http`, `tcp`, and `sni`. The `sni` mode implies TCP with an SNI-based routing. The parameter can be prefixed with an index thus allowing definition of multiple modes for a single service (e.g. `http`, `tcp`, and so on).<br>**Example:** `tcp`|http|
|reqPathReplace |A regular expression to apply the modification.<br>**Example:** `/demo/`| |
|reqPathSearch  |A regular expression to search the content to be replaced. If specified, `reqPathReplace` needs to be set as well.<br>**Example:** `/something/`| |
|serviceName    |The name of the service. It must match the name of the Swarm service. This parameter is **mandatory**. If used through *Docker Flow Swarm Listener*, this parameter is added automatically.<br>**Example:** `go-demo`| |
|setReqHeader   |Additional headers that will be set to the request before forwarding it to the service. If a specified header exists, it will be replaced with the new one. Multiple headers should be separated with comma (`,`). Please consult [Set a header to the request](https://www.haproxy.com/doc/aloha/7.0/haproxy/http_rewriting.html#set-a-header-in-the-request) for more info.<br>**Example:** `X-Forwarded-Port %[dst_port],X-Forwarded-Ssl on if { ssl_fc }`| |
|setResHeader   |Additional headers that will be set to the response before forwarding it to the client. If a specified header exists, it will be replaced with the new one. Multiple headers should be separated with comma (`,`). Please consult [Set a header to the response](https://www.haproxy.com/doc/aloha/7.0/haproxy/http_rewriting.html#set-a-header-in-the-response) for more info.<br>**Example:** `X-Via %[env(HOSTNAME)],Server haproxy`| |
|srcPort        |The source (entry) port of a service. The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `srcPort.1`, `srcPort.2`, and so on). The parameter is mandatory when specifying multiple destinations of a single service.<br>**Example:** `80`| |
|timeoutServer  |The server timeout in seconds.<br>**Example:** `60`                                              |20     |
|timeoutTunnel  |The tunnel timeout in seconds.<br>**Example:** `3600`                                            |3600   |
|xForwardedProto|Whether to add "X-Forwarded-Proto https" header.<br>**Example:** `true`                          |false  |

Multiple destinations for a single service can be specified by adding index as a suffix to `servicePath`, `srcPort`, `port`, `userAgent`, `ignoreAuthorization`, `serviceDomain``allowedMethods`, `deniedMethods`, `denyHttp`, `httpsOnly`, or `ReqMode` parameters. In that case, `srcPort` is required.

### HTTP Mode Query Parameters

The following query parameters can be used only when `reqMode` is set to `http` or is empty.

|Query        |Description                                                                     |
|-------------|--------------------------------------------------------------------------------|
|allowedMethods|The list of allowed methods. If specified, a request with a method that is not on the list will be denied. Multiple methods can be separated with comma (`,`). The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `allowedMethods.1`, `allowedMethods.2`, and so on).<br>**Example:** `GET,DELETE`|
|deniedMethods|The list of denied methods. If specified, a request with a method that is on the list will be denied. Multiple methods can be separated with comma (`,`). The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `deniedMethods.1`, `deniedMethods.2`, and so on).<br>**Example:** `PUT,POST`|
|denyHttp     |Whether to deny HTTP requests thus allowing only HTTPS. The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `denyHttp.1`, `denyHttp.2`, and so on).<br>**Example:** `true`<br>**Default Value:** `false`|
|httpsOnly    |If set to true, HTTP requests to the service will be redirected to HTTPS. The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `httpsOnly.1`, `httpsOnly.2`, and so on).<br>**Example:** `true`<br>**Default Value:** `false`|
|outboundHostname|The hostname where the service is running, for instance on a separate swarm. If specified, the proxy will dispatch requests to that domain.<br>**Example:** `ecme.com`|
|pathType     |The ACL derivative. Defaults to *path_beg*. See [HAProxy path](https://cbonte.github.io/haproxy-dconv/configuration-1.5.html#7.3.6-path) for more info.<br>**Example:** `path_beg`|
|redirectWhenHttpProto|Whether to redirect to https when X-Forwarded-Proto is set and the request is made over an HTTP port.<br>**Example:** `true`<br>**Default Value:** `false`|
|serviceCert  |Content of the PEM-encoded certificate to be used by the proxy when serving traffic over SSL.|
|serviceDomain  |The domain of the service. If set, the proxy will allow access only to requests coming to that domain. The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `serviceDomain.1`, `serviceDomain.2`, and so on). This parameter is **mandatory** if `servicePath` is not specified.<br>**Example:** `ecme.com`|
|serviceDomainAlgo|Algorithm that should be applied to domain ACLs. If not set, the value of the environment variable `SERVICE_DOMAIN_ALGO` will be used instead. If defaults to `hdr(host)`<br>**Examples:**<br>`hdr(host)`: matches only if domain is the same as `serviceDomain`<br>`hdr_dom()`: matches the specified `serviceDomain` and any subdomain<br>`req.ssl_sni`: matches Server Name TLS extension|
|serviceHeader|Headers used to filter requests. If set, the proxy will allow access only to requests that contain specified headers. A header consists of a key and value separated with colon (e.g. `X-Version:3`). Multiple headers can be separated with comma (e.g. `X-Version:3,name:viktor`). The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `serviceHeader.1`, `serviceHeader.2`, and so on). <br>**Example:** `X-Version:3,name:viktor`|
|servicePath  |The URL path of the service. Multiple values should be separated with comma (`,`). The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `servicePath.1`, `servicePath.2`, and so on). This parameter **is mandatory** unless `serviceDomain` is specified.<br>**Example:** `/api/v1/books`|
|sessionType  |Determines the type of sticky sessions. If set to `sticky-server`, session cookie will be set by the proxy. Any other value means that sticky sessions are not used and load balancing is performed by Docker's Overlay network.<br>**Example:** `sticky-server`|
|sslVerifyNone|If set to true, backend server certificates are not verified. This flag should be set for SSL enabled backend services.<br>**Example:** `true`<br>**Default Value:** `false`|
|templateBePath|The path to the template representing a snippet of the backend configuration. If specified, the backend template will be loaded from the specified file. See the [Templates](#templates) section for more info.<br>**Example:** `/tmpl/be.tmpl`|
|templateFePath|The path to the template representing a snippet of the frontend configuration. If specified, the frontend template will be loaded from the specified file. See the [Templates](#templates) section for more info.<br>**Example:** `/tmpl/fe.tmpl`|
|userAgent    |A comma-separated list of user agents. only requests with the same User-Agent will be forwarded to the backend. The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `userAgent.1`, `userAgent.2`, and so on). If the same service is used for multiple agents, it is recommended to use indexes with the last one being without `userAgent`. That way, if no match is found, the last indexed destination will be used as catch-all.<br>**Example:** `googlebot,iphone`|
|users        |A comma-separated list of credentials (<user>:<pass>) for HTTP basic authentication. It applies only to the service that will be reconfigured. If used with `usersSecret`, or when `USERS` environment variable is set, password may be omitted. In that case, it will be taken from `usersSecret` file or the global configuration if `usersSecret` is not present.<br>**Example:** `usr1:pwd1, usr2:pwd2`|
|usersSecret  |Suffix of Docker secret from which credentials will be taken for this service. Files must be a comma-separated list of credentials (<user>:<pass>). This suffix will be prepended with `dfp_users_`. For example, if the value is `mysecrets` the expected name of the Docker secret is `dfp_users_mysecrets`.<br>**Example:** `mysecrets`|
|usersPassEncrypted|Indicates whether passwords provided by `users` or `usersSecret` contain encrypted data. Passwords can be encrypted with the command `mkpasswd -m sha-512 password1`.<br>**Example:** `true`<br>**Default Value:** `false`|
|verifyClientSsl|Whether to verify client SSL and, if it is not valid, deny request and return 403 Forbidden status code. SSL is validated against the `ca-file` specified through the environment variable `CA_FILE`.<br>**Example:** true<br>**Default Value:** `false`|

Multiple destinations for a single service can be specified by adding index as a suffix to `servicePath`, `srcPort`, `port`, `userAgent`, `ignoreAuthorization`, `serviceDomain`, `allowedMethods`, `deniedMethods`, `denyHttp`, `httpsOnly`, or `ReqMode` parameters. In that case, `srcPort` is required.

### TCP Mode HTTP Query Parameters

The `reqMode` set to `tcp` does not have any specific parameters beyond those specified in the [Reconfigure General Parameters](#reconfigure-general-parameters) section.

!!! warning
    If multiple TCP services are defined to use the same `srcPort`, `serviceDomain` must be set for those services.

Please consult the [Using TCP Request Mode](swarm-mode-auto.md#using-tcp-request-mode) section for an example of working with `tcp` request mode.

An example request is as follows.

```
[PROXY_IP]:[PROXY_PORT]/v1/docker-flow-proxy/reconfigure?serviceName=foo&servicePath.1=/&port.1=8080&srcPort.1=80&servicePath.2=/&port.2=8081&srcPort.2=443
```

The command would create a service `foo` that exposes ports `8080` and `8081`. All requests coming to proxy port `80` with the path that starts with `/` will be forwarded to the service `foo` port `8080`. Equally, all requests coming to proxy port `443` (*HTTPS*) with the path that starts with `/` will be forwarded to the service `foo` port `8081`.

Indexes are incremental and start with `1`.

### Environment Variables

When a service is not part of the same Swarm cluster, a failure of a proxy instance means that the information about those services cannot be obtained through Docker API and *Docker Flow Swarm Listener*. In such a case, data loss can be prevented through the usage of environment variables.

!!! tip
    In most cases, there is no need to use environment variables for services running inside the same Swarm cluster as the proxy.

The environment variables that can be used for the initial configuration of services follow the same naming standards as the [HTTP Query Parameters](#general-http-query-parameters).

The environment variables must apply the rules that follow.

* The names of the environment variables must be prefixed with `DFP_SERVICE_`.
* The names of the environment variables must be in upper case and words must be separated by an underscore (`_`). As an example, the HTTP query parameter `aclName` would be defined as the environment variable `DFP_SERVICE_ACL_NAME`.
* Multiple services must be defined by adding an index to the prefix. Indexing starts from `1`. As an example, the `aclName` of the second service would be defined as `DFP_SERVICE_2_ACL_NAME`.
* All other rules applied to `reconfigure` HTTP query parameters apply to environment variables as well.

The map between the HTTP query parameters and environment variables is as follows.

|Query                |Environment variable    |
|---------------------|------------------------|
|aclName              |ACL_NAME                |
|addReqHeader         |ADD_REQ_HEADER          |
|addResHeader         |ADD_RES_HEADER          |
|allowedMethods       |ALLOWED_METHODS         |
|backendExtra         |BACKEND_EXTRA           |
|deniedMethods        |DENIED_METHODS          |
|distribute           |DISTRIBUTE              |
|httpsOnly            |HTTPS_ONLY              |
|httpsPort            |HTTPS_PORT              |
|isDefaultBackend     |IS_DEFAULT_BACKEND      |
|outboundHostname     |OUTBOUND_HOSTNAME       |
|pathType             |PATH_TYPE               |
|port                 |PORT                    |
|redirectWhenHttpProto|REDIRECT_WHEN_HTTP_PROTO|
|reqMode              |REQ_MODE                |
|reqPathReplace       |REQ_PATH_REPLACE        |
|reqPathSearch        |REQ_PATH_SEARCH         |
|serviceCert          |SERVICE_CERT            |
|serviceDomain        |SERVICE_DOMAIN          |
|serviceDomainMatchAll|SERVICE_DOMAIN_MATCH_ALL|
|serviceName          |SERVICE_NAME            |
|servicePath          |SERVICE_PATH            |
|setReqHeader         |SET_REQ_HEADER          |
|setResHeader         |SET_RES_HEADER          |
|srcPort              |SRC_PORT                |
|sslVerifyNone        |SSL_VERIFY_NONE         |
|templateBePath       |TEMPLATE_BE_PATH        |
|templateFePath       |TEMPLATE_FE_PATH        |
|timeoutServer        |TIMEOUT_SERVER          |
|timeoutTunnel        |TIMEOUT_TUNNEL          |
|users                |**Not supported**       |
|usersSecret          |**Not supported**       |
|usersPassEncrypted   |**Not supported**       |
|verifyClientSsl      |VERIFY_CLIENT_SSL       |
|xForwardedProto      |X_FORWARDED_PROTO       |

Please explore the [Configuring Non-Swarm Services](non-swarm.md) tutorial for more info.

## Remove

> Removes a service from the proxy

The following query arguments can be used to send a *remove* request to *Docker Flow Proxy*. They should be added to the base address **[PROXY_IP]:[PROXY_PORT]/v1/docker-flow-proxy/remove**.

|Query      |Description                                                                 |Required|Default|Example|
|-----------|----------------------------------------------------------------------------|--------|-------|-------|
|aclName    |Mandatory if ACL name was specified in reconfigure request                  |No      |       |05-go-demo-acl|
|serviceName|The name of the service. It must match the name of the service              |Yes     |       |go-demo|
|distribute |Whether to distribute a request to all the instances of the proxy. Used only in the *swarm* mode.|No|false|true|

## Certificates

All certificates stored in `/certs` directory are loaded automatically. If you already have a set of certificates you might choose to store them on a network drive and mount it to the service as `/certs`.

*Docker Flow Proxy* supports [Docker Secrets](https://docs.docker.com/engine/swarm/secrets/) for storing certificates. It will automatically load all certificate files added to the service as a secret. Only secrets with names that start with `cert-` or `cert_` will be considered a certificate.

During runtime, additional certificates can be added through [Put Certificate](#put-certificate) request.

Please consult [Configuring SSL Certificates](/certs) for a few examples of working with certificates.

## Put Certificate

> Puts SSL certificate to proxy configuration

The following query arguments can be used to send a *cert* request to *Docker Flow Proxy*. They should be added to the base address **[PROXY_IP]:[PROXY_PORT]/v1/docker-flow-proxy/cert**. Please note that the request method MUST be *PUT* and the certificate must be placed in request body.

When a new replica is deployed, it will synchronize with other replicas and recuperate their certificates.

|Query      |Description                                                                 |Required|Default|Example    |
|-----------|----------------------------------------------------------------------------|--------|-------|-----------|
|certName   |The file name of the certificate                                            |Yes     |       |my-cert.pem|
|distribute |Whether to distribute a request to all the instances of the proxy. Used only in the *swarm* mode.|No|false|true|

An example is as follows.

```bash
curl -i -XPUT \
    --data-binary @my-certificate.pem \
    "[PROXY_IP]:[PROXY_PORT]/v1/docker-flow-proxy/cert?certName=my-certificate.pem&distribute=true"
```

Please note that the internal proxy port `8080` must be published.

The example would send a certificate stored in the `my-certificate.pem` file. The certificate would be distributed to all replicas of the proxy.

!!! tip
    Use this feature only if your certificates are renewed often. To be on the safe side, it is recommended to mount `/certs` directory to a network drive and thus ensure that certs are preserved in case of a failure.

## Reload

> Reloads proxy configuration

The following query arguments can be used to send a *reload* request to *Docker Flow Proxy*. They should be added to the base address **[PROXY_IP]:[PROXY_PORT]/v1/docker-flow-proxy/reload**.

|Query      |Description                                                |Required|Default|Example |
|-----------|-----------------------------------------------------------|--------|-------|--------|
|fromListener|Whether proxy configuration should be recreated from *Docker Flow Swarm Listener*. If set to true, configuration will be recreated independently of the `recreate` parameter. This operation is asynchronous.|No|false|true|
|recreate   |Recreates configuration using the information already available in the proxy. This param is useful in case config gets corrupted.|No|false|true|

An example is as follows.

```bash
curl -i \
    "[PROXY_IP]:[PROXY_PORT]/v1/docker-flow-proxy/reload?recreate=false&fromListener=true"
```

## Ping

> Ping the service

The `ping` endpoint might be useful if implementing `HEALTHCHECK`.

An example Dockerfile is as follows.

```
FROM vfarcic/docker-flow-proxy

HEALTHCHECK --interval=5s --timeout=5s CMD wget -qO- "http://localhost:8080/v1/docker-flow-proxy/ping"
```

## Config

> Outputs HAProxy configuration

The following query arguments can be used to send a *reload* request to *Docker Flow Proxy*. They should be added to the base address **[PROXY_IP]:[PROXY_PORT]/v1/docker-flow-proxy/config**.

|Query      |Description                                                |
|-----------|-----------------------------------------------------------|
|type       |If set to `json`, the list of services is returned in JSON format. Any other value returns HAProxy configuration in text format.<br>**Default:** `text`<br>**Example:** `json`|

## Templates

Proxy configuration is a combination of configuration files generated from templates. Base template is `haproxy.tmpl`. Each service appends frontend and backend templates on top of the base template. Once all the templates are combined, they are converted into the `haproxy.cfg` configuration file.

The templates can be extended by creating a new Docker image based on `vfarcic/docker-flow-proxy` and adding the templates through `templateFePath` and `templateBePath` [reconfigure parameters](#reconfigure).

Templates are based on [Go Templates](https://golang.org/pkg/text/template/).

Please see the [proxy/types.go](https://github.com/vfarcic/docker-flow-proxy/blob/master/proxy/types.go) for info about the structure used with templates.

