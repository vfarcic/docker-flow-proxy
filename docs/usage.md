# Usage

*Docker Flow Proxy* can be reconfigured by sending HTTP requests or through Docker Service labels when combined with [Docker Flow Swarm Listener](https://github.com/vfarcic/docker-flow-swarm-listener).

## Reconfigure

> Reconfigures the proxy

The proxy can be reconfigured to use request mode *http* or *tcp*. The default value of the request mode is *http* and can be changed with the parameter `reqMode`.

The following query parameters can be used to send as a *reconfigure* request to *Docker Flow Proxy*. They apply to any request mode and should be added to the base address **<PROXY_IP>:<PROXY_PORT>/v1/docker-flow-proxy/reconfigure**. The apply to any `reqMode`.

|Query        |Description                                                                     |Required|Default|Example      |
|-------------|--------------------------------------------------------------------------------|--------|-------|-------------|
|httpsPort    |The internal HTTPS port of a service that should be reconfigured. The port is used only in the *swarm* mode. If not specified, the `port` parameter will be used instead.|No| ||443|
|port         |The internal port of a service that should be reconfigured. The port is used only in the *swarm* mode. The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `port.1`, `port.2`, and so on).|Only in *swarm* mode| |8080|
|reqMode      |The request mode. The proxy should be able to work with any mode supported by HAProxy. However, actively supported and tested modes are *http* and *tcp*. Please open an GitHub issue if the mode you're using does not work as expected.|Yes |http   |tcp          |
|reqPathReplace|A regular expression to apply the modification. If specified, `reqPathSearch` needs to be set as well.|No| |/demo/|
|reqPathSearch|A regular expression to search the content to be replaced. If specified, `reqPathReplace` needs to be set as well.|No| |/something/|
|serviceName  |The name of the service. It must match the name of the Swarm service or the one stored in Consul.|Yes| |go-demo |
|timeoutServer|The server timeout in seconds.                                                  |No      |       |60           |
|timeoutTunnel|The tunnel timeout in seconds.                                                  |No      |       |1800         |

The following query parameters can be used when `reqMode` is set to `http` or is empty.

|Query        |Description                                                                     |Required|Default|Example      |
|-------------|--------------------------------------------------------------------------------|--------|-------|-------------|
|aclName      |ACLs are ordered alphabetically by their names. If not specified, serviceName is used instead.|No| |05-go-demo-acl|
|consulTemplateBePath|The path to the Consul Template representing a snippet of the backend configuration. If set, proxy template will be loaded from the specified file.| ||/consul_templates/tmpl/go-demo-be.tmpl|
|consulTemplateFePath|The path to the Consul Template representing a snippet of the frontend configuration. If set, proxy template will be loaded from the specified file.| ||/consul_templates/tmpl/go-demo-fe.tmpl|
|distribute   |Whether to distribute a request to all the instances of the proxy. Used only in the *swarm* mode.|No|false|true|
|httpsOnly    |If set to true, HTTP requests to the service will be redirected to HTTPS.        |No      |false  |true         |
|outboundHostname|The hostname where the service is running, for instance on a separate swarm. If specified, the proxy will dispatch requests to that domain.|No| |ecme.com|
|pathType     |The ACL derivative. Defaults to *path_beg*. See [HAProxy path](https://cbonte.github.io/haproxy-dconv/configuration-1.5.html#7.3.6-path) for more info.|No| |path_beg|
|serviceCert  |Content of the PEM-encoded certificate to be used by the proxy when serving traffic over SSL.|No| ||
|serviceDomain|The domain of the service. If set, the proxy will allow access only to requests coming to that domain. Multiple domains should be separated with comma (`,`).|No| |ecme.com|
|servicePath  |The URL path of the service. Multiple values should be separated with comma (`,`). The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `servicePath.1`, `servicePath.2`, and so on).|Yes| |/api/v1/books|
|skipCheck    |Whether to skip adding proxy checks. This option is used only in the *default* mode.|No      |false  |true         |
|srcPort      |The source (entry) port of a service. Useful only when specifying multiple destinations of a single service. The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `srcPort.1`, `srcPort.2`, and so on).|No| |80|
|templateBePath|The path to the template representing a snippet of the backend configuration. If specified, the backend template will be loaded from the specified file. If specified, `templateFePath` must be set as well. See the [Templates](#templates) section for more info.| ||/templates/go-demo-be.tmpl|
|templateFePath|The path to the template representing a snippet of the frontend configuration. If specified, the frontend template will be loaded from the specified file. If specified, `templateBePath` must be set as well. See the [Templates](#templates) section for more info.| ||/templates/go-demo-fe.tmpl|
|users        |A comma-separated list of credentials(<user>:<pass>) for HTTP basic auth, which applies only to the service that will be reconfigured.|No| |usr1:pwd1,usr2:pwd2|

The following query parameters can be used when `reqMode` is set to `tcp`.

|Query        |Description                                                                     |Required|Default|Example      |
|-------------|--------------------------------------------------------------------------------|--------|-------|-------------|
|srcPort      |The source (entry) port of a service. The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `srcPort.1`, `srcPort.2`, and so on).|Yes| |6378|
|port         |The internal port of a service that should be reconfigured. The parameter can be prefixed with an index thus allowing definition of multiple destinations for a single service (e.g. `port.1`, `port.2`, and so on).|Yes| |6379|

Multiple destinations for a single service can be specified by adding index as a suffix to `servicePath` and `port` parameters. In that case, `srcPort` is required. Defining multiple destinations is useful in cases when a service exposes multiple ports with different paths and functions.

Please consult the [Using TCP Request Mode](swarm-mode-auto.md#using-tcp-request-mode) section for an example of working with `tcp` request mode.

An example request is as follows.

```
<PROXY_IP>:<PROXY_PORT>/v1/docker-flow-proxy/reconfigure?serviceName=foo&servicePath.1=/&port.1=8080&srcPort.1=80&servicePath.2=/&port.2=8081&srcPort.2=443
```

The command would create a service `foo` that exposes ports `8080` and `8081`. All requests coming to proxy port `80` with the path that starts with `/` will be forwarded to the service `foo` port `8080`. Equally, all requests coming to proxy port `443` (*HTTPS*) with the path that starts with `/` will be forwarded to the service `foo` port `8081`.

Indexes are incremental and start with `1`.

## Remove

> Removes a service from the proxy

The following query arguments can be used to send a *remove* request to *Docker Flow Proxy*. They should be added to the base address **[PROXY_IP]:[PROXY_PORT]/v1/docker-flow-proxy/remove**.

|Query      |Description                                                                 |Required|Default|Example|
|-----------|----------------------------------------------------------------------------|--------|-------|-------|
|aclName    |Mandatory if ACL name was specified in reconfigure request                  |No      |       |05-go-demo-acl|
|serviceName|The name of the service. It must match the name stored in Consul            |Yes     |       |go-demo|
|distribute |Whether to distribute a request to all the instances of the proxy. Used only in the *swarm* mode.|No|false|true|

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

## Reload

> Reloads proxy configuration

The address is **[PROXY_IP]:[PROXY_PORT]/v1/docker-flow-proxy/reload**

## Config

> Outputs HAProxy configuration

The address is **[PROXY_IP]:[PROXY_PORT]/v1/docker-flow-proxy/config**

## Templates

Proxy configuration is a combination of configuration files generated from templates. Base template is `haproxy.tmpl`. Each service appends frontend and backend templates on top of the base template. Once all the templates are combined, they are converted into the `haproxy.cfg` configuration file.

The templates can be extended by creating a new Docker image based on `vfarcic/docker-flow-proxy` and adding the templates through `templateFePath` and `templateBePath` [reconfigure parameters](#reconfigure).

Templates are based on [Go HTML Templates](https://golang.org/pkg/html/template/).

Please see the [proxy/types.go](https://github.com/vfarcic/docker-flow-proxy/blob/master/proxy/types.go) for info about the structure used with templates.

