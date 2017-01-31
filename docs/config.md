# Configuring Docker Flow Proxy

*Docker Flow Proxy* can be configured through Docker environment variables and/or by creating a new image based on `vfarcic/docker-flow-proxy`.

## Environment Variables

!!! tip
	The *Docker Flow Proxy* container can be configured through environment variables

The following environment variables can be used to configure the *Docker Flow Proxy*.

|Variable           |Description                                               |Required|Default|Example|
|-------------------|----------------------------------------------------------|--------|-------|-------|
|BIND_PORTS         |Ports to bind in addition to `80` and `443`. Multiple values can be separated with comma|No| |8085, 8086|
|CONNECTION_MODE    |HAProxy supports 5 connection modes. *keep alive*: all requests and responses are processed. *tunnel*: only the first request and response are processed, everything else is forwarded with no analysis. *passive close*: tunnel with "Connection: close" added in both directions. *server close*: the server-facing connection is closed after the response. *forced close*: the connection is actively closed after end of response. In general it is preferred to use *http-server-close* with application servers, and some static servers might benefit from *http-keep-alive*.|No|http-server-close|http-keep-alive|
|CONSUL_ADDRESS     |The address of a Consul instance used for storing proxy information and discovering running nodes.  Multiple addresses can be separated with comma (e.g. 192.168.0.10:8500,192.168.0.11:8500).|Only in the *default* mode| |192.168.0.10:8500|
|EXTRA_FRONTEND     |Value will be added to the default `frontend` configuration.|No    | |http-request set-header X-Forwarded-Proto https if { ssl_fc }|
|LISTENER_ADDRESS   |The address of the [Docker Flow: Swarm Listener](https://github.com/vfarcic/docker-flow-swarm-listener) used for automatic proxy configuration.|Only in the *swarm* mode| |swarm-listener|
|MODE               |Two modes are supported. The *default* mode should be used for general purpose. It requires a Consul instance and service data to be stored in it (e.g. through Registrator). The *swarm* mode is designed to work with new features introduced in Docker 1.12 and assumes that containers are deployed as Docker services (new Swarm).|No      |default|swarm|
|PROXY_INSTANCE_NAME|The name of the proxy instance. Useful if multiple proxies are running inside a cluster|No|docker-flow|docker-flow|
|SERVICE_NAME       |The name of the service. It must be the same as the value of the `--name` argument used to create the proxy service. Used only in the *swarm* mode.|No|proxy|my-proxy|
|SKIP_ADDRESS_VALIDATION|Whether to skip validating service address before reconfiguring the proxy.|No|false|true|
|STATS_USER         |Username for the statistics page                          |No      |admin  |my-user|
|STATS_PASS         |Password for the statistics page                          |No      |admin  |my-pass|
|TIMEOUT_CLIENT     |The client timeout in seconds                             |No      |20     |5      |
|TIMEOUT_CONNECT    |The connect timeout in seconds                            |No      |5      |3      |
|TIMEOUT_QUEUE      |The queue timeout in seconds                              |No      |30     |10     |
|TIMEOUT_SERVER     |The server timeout in seconds                             |No      |20     |5      |
|TIMEOUT_TUNNEL     |The tunnel timeout in seconds                             |No      |3600   |1800   |
|TIMEOUT_HTTP_REQUEST|The HTTP request timeout in seconds                      |No      |5      |3      |
|TIMEOUT_HTTP_KEEP_ALIVE|The HTTP keep alive timeout in seconds                |No      |15     |10     |
|USERS              |A comma-separated list of credentials(<user>:<pass>) for HTTP basic auth, which applies to all the backend routes.|No| |user1:pass1, user2:pass2|

## Custom Config

The base HAProxy configuration can be found in [haproxy.tmpl](haproxy.tmpl). It can be customized by creating a new image. An example *Dockerfile* is as follows.

```
FROM vfarcic/docker-flow-proxy
COPY haproxy.tmpl /cfg/tmpl/haproxy.tmpl
```

## Custom Errors

Default error messages are stored in the `/errorfiles` directory inside the *Docker Flow Proxy* image. They can be customized by creating a new image with custom error files or mounting a volume. Currently supported errors are `400`, `403`, `405`, `408`, `429`, `500`, `502`, `503`, and `504`.
