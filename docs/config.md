# Configuring Docker Flow Proxy

*Docker Flow Proxy* can be configured through Docker environment variables and/or by creating a new image based on `vfarcic/docker-flow-proxy`.

## Environment Variables

!!! tip
	The *Docker Flow Proxy* container can be configured through environment variables

The following environment variables can be used to configure the *Docker Flow Proxy*.

|Variable           |Description                                               |
|-------------------|----------------------------------------------------------|
|BIND_PORTS         |Ports to bind in addition to `80` and `443`. Multiple values can be separated with comma. If a port is specified with the `srcPort` reconfigure parameter, it is not required to specify it in this environment variable. Those values will be used as default ports used for services that do not specify `srcPort`. Please note that all binded ports need to be published on the service level (usually defined in a Compose stack file).<br>**Example:** 8085, 8086|
|CA_FILE            |Path to a PEM file from which to load CA certificates that will be used to verify client's certificate. Preferably, the file should be provided as a Docker secret.<br>**Example:** /run/secrets/ca-file|
|CAPTURE_REQUEST_HEADER|Allows capturing specific request headers. This feature is useful if debugging is enabled (e.g. `DEBUG=true`) and the format is customized with `DEBUG_HTTP_FORMAT` or `DEBUG_TCP_FORMAT` to output headers. Header name and lenght in bytes must be separated with colon (e.g. `Host:15`). Multiple headers should be separated with colon (e.g. `Host:15,X-Forwarded-For:20`).<br>**Example:** `Host:15,X-Forwarded-For:20,Referer:15`|
|CFG_TEMPLATE_PATH  |Path to the configuration template. The path can be absolute (starting with `/`) or relative to `/cfg/tmpl`.<br>**Default value:** `/cfg/tmpl/haproxy.tmpl`|
|CHECK_RESOLVERS    |Enable `docker` as a resolver. Provides higher reliability at the cost of backend initialization time. If enabled, it might take a few seconds until a backend is resolved and operational.<br>**Default value:** `false`|
|CERTS              |This parameter is **deprecated** as of February 2017. All the certificates from the `/certs/` directory are now loaded automatically.|
|COMPRESSION_ALGO   |Enable HTTP compression. The currently supported algorithms are:<br>**identity**: this is mostly for debugging.<br>**gzip**: applies gzip compression. This setting is only available when support for zlib or libslz was built in.<br>**deflate** same as *gzip*, but with deflate algorithm and zlib format. Note that this algorithm has ambiguous support on many browsers and no support at all from recent ones. It is strongly recommended not to use it for anything else than experimentation. This setting is only available when support for zlib or libslz was built in.<br>**raw-deflate**: same as *deflate* without the zlib wrapper, and used as an alternative when the browser wants "deflate". All major browsers understand it and despite violating the standards, it is known to work better than *deflate*, at least on MSIE and some versions of Safari. This setting is only available when support for zlib or libslz was built in.<br>Compression will be activated depending on the Accept-Encoding request header. With identity, it does not take care of that header. If backend servers support HTTP compression, these directives will be no-op: haproxy will see the compressed response and will not compress again. If backend servers do not support HTTP compression and there is Accept-Encoding header in request, haproxy will compress the matching response.<br>Compression is disabled when:<br>* the request does not advertise a supported compression algorithm in the "Accept-Encoding" header<br>* the response message is not HTTP/1.1<br>* HTTP status code is not 200<br>* response header "Transfer-Encoding" contains "chunked" (Temporary Workaround)<br>* response contain neither a "Content-Length" header nor a "Transfer-Encoding" whose last value is "chunked"<br>* response contains a "Content-Type" header whose first value starts with "multipart"<br>* the response contains the "no-transform" value in the "Cache-control" header<br>* User-Agent matches "Mozilla/4" unless it is MSIE 6 with XP SP2, or MSIE 7 and later<br>* The response contains a "Content-Encoding" header, indicating that the response is already compressed (see compression offload)<br>**Example:** gzip|
|COMPRESSION_TYPE   |The type of files that will be compressed.<br>**Example:** text/css text/html text/javascript application/javascript text/plain text/xml application/json|
|CONNECTION_MODE    |HAProxy supports 5 connection modes.<br><br>`http-keep-alive`: all requests and responses are processed.<br>`http-tunnel`: only the first request and response are processed, everything else is forwarded with no analysis.<br>`httpclose`: tunnel with "Connection: close" added in both directions.<br>`http-server-close`: the server-facing connection is closed after the response.<br>`forceclose`: the connection is actively closed after end of response.<br><br>In general, it is preferred to use `http-server-close` with application servers, and some static servers might benefit from `http-keep-alive`.<br>**Example:** `http-server-close`<br>**Default value:** `http-keep-alive`|
|DEBUG              |Enables logging of each request sent through the proxy. Please consult [Debug Format](#debug-format) for info about the log entries. This feature should be used with caution. **Do not enable debugging in production unless necessary.**<br>**Example:** true<br>**Default value:** `false`|
|DEBUG_ERRORS_ONLY  |If set to `true`, only requests that resulted in an error, timeout, retry, and redispatch will be logged. If a request is HTTP, responses with a status 5xx will be logged too. This variable will take effect only if `DEBUG` is set to `true`.<br>**Example:** true<br>**Default value:** `false`|
|DEBUG_HTTP_FORMAT  |Logging format that will be used with HTTP requests. Please consult [Custom log format](https://cbonte.github.io/haproxy-dconv/1.7/configuration.html#8.2.4) for more info about the available options.|
|DEBUG_TCP_FORMAT   |Logging format that will be used with TCP requests. Please consult [Custom log format](https://cbonte.github.io/haproxy-dconv/1.7/configuration.html#8.2.4) for more info about the available options.|
|DEFAULT_PORTS      |The default ports used by the proxy. Multiple values can be separated with comma (`,`). If a port should be for SSL connections, append it with `:ssl`. Additional binding options can be added after a port. For example, `80 accept-proxy,443 accept-proxy:ssl` adds `accept-proxy` to the defalt binding options.<br>**Default value:** `80,443:ssl`|
|DO_NOT_RESOLVE_ADDR|Whether not to resolve addresses. If set to `true`, the proxy will NOT fail if the service is not available.<br>**Default value:** `false`|
|EXTRA_FRONTEND     |Value will be added to the default `frontend` configuration. Multiple lines should be separated with comma (*,*).|
|EXTRA_GLOBAL       |Value will be added to the default `global` configuration. Multiple lines should be separated with comma (*,*).|
|LISTENER_ADDRESS   |The address of the [Docker Flow: Swarm Listener](https://github.com/vfarcic/docker-flow-swarm-listener) used for automatic proxy configuration.<br>**Example:** `swarm-listener:8080`|
PROXY_INSTANCE_NAME|The name of the proxy instance. Useful if multiple proxies are running inside a cluster.<br>**Default value:** `docker-flow`|
|SERVICE_NAME       |The name of the service. It must be the same as the value of the `--name` argument used to create the proxy service. Used only in the *swarm* mode.<br>**Example:** `my-proxy`<br>**Default value:** `proxy`|
|RELOAD_INTERVAL    |Defines the frequency (in milliseconds) between automatic config reloads from Swarm Listener.<br>**Default value:** `5000`|
|REPEAT_RELOAD      |If set to `true`, the proxy will periodically reload the config, using `RELOAD_INTERVAL` as pause between iterations.<br>**Example:** `true`<br>**Default value:** `false`|
|SERVICE_DOMAIN_ALGO|The default algorithm applied to domain ACLs. It can be overwritten for a service through the `serviceDomainAlgo` parameter.<br>**Examples:**<br>`hdr(host)`: matches only if domain is the same as `serviceDomain`<br>`hdr_dom()`: matches the specified `serviceDomain` and any subdomain<br>`req.ssl_sni`: matches Server Name TLS extension<br>**Default Value:** `hdr(host)`|
|SKIP_ADDRESS_VALIDATION|Whether to skip validating service address before reconfiguring the proxy.<br>**Example:** false<br>**Default value:** `true`|
|SSL_BIND_CIPHERS   |Sets the default string describing the list of cipher algorithms ("cipher suite") that are negotiated during the SSL/TLS handshake for all "bind" lines which do not explicitly define theirs. The format of the string is defined in "man 1 ciphers" from OpenSSL man pages, and can be for instance a string such as `AES:ALL:!aNULL:!eNULL:+RC4:@STRENGTH`.<br>**Default value:** see [Dockerfile](https://github.com/vfarcic/docker-flow-proxy/blob/master/Dockerfile#L31)|
|SSL_BIND_OPTIONS   |Sets default ssl-options to force on all "bind" lines.<br>**Default value:** `no-sslv3`|
|STATS_USER         |Username for the statistics page. If not set, stats will not be available. If both `STATS_USER` and `STATS_PASS` are set to `none`, statistics will be available without authentication.<br>**Example:** my-user<br>**Default value:** `admin`|
|STATS_USER_ENV     |The name of the environment variable that holds the username for the statistics page.<br>**Example:** MY_USER<br>**Default value:** `STATS_USER`|
|STATS_PASS         |Password for the statistics page. If not set, stats will not be available. If both `STATS_USER` and `STATS_PASS` are set to `none`, statistics will be available without authentication.<br>**Example:** my-pass<br>**Default value:** `admin`|
|STATS_PASS_ENV     |The name of the environment variable that holds the password for the statistics page.<br>**Example:** MY_PASS|STATS_PASS|
|STATS_URI          |URI for the statistics page.<br>**Example:** `/proxyStats`<br>**Default value:** `/admin?proxy`|
|STATS_URI_ENV      |The name of the environment variable that holds the URI for the statistics page.<br>**Example:** `MY_URI`<br>**Default value:** `STATS_URI`|
|TERMINATE_ON_RELOAD|Whether to terminate the proxy process every time a reload request is received. If set to `false`, a new process will spawn and all the existing requests will terminate through the old process. The downside of this approach is that the system might end up with zombie processes. If set to `true`, zombie processes will be removed but the existing requests to the proxy might be cut.<br>**Example:** `true`<br>**Default value:** `false`|
|TIMEOUT_CLIENT     |The client timeout in seconds.<br>**Example:** `5`<br>**Default value:** `20`|
|TIMEOUT_CONNECT    |The connect timeout in seconds.<br>**Example:** `3`<br>**Default value:** `5`|
|TIMEOUT_QUEUE      |The queue timeout in seconds.<br>**Example:** `10`<br>**Default value:** `30`|
|TIMEOUT_SERVER     |The server timeout in seconds.<br>**Example:** `15`<br>**Default value:** `20`|
|TIMEOUT_TUNNEL     |The tunnel timeout in seconds.<br>**Example:** `1800`<br>**Default value:** `3600`|
|TIMEOUT_HTTP_REQUEST|The HTTP request timeout in seconds.<br>**Example:** `3`<br>**Default value:** `5`|
|TIMEOUT_HTTP_KEEP_ALIVE|The HTTP keep alive timeout in seconds.<br>**Example:** `10`<br>**Default value:** `15`|
|USERS              |A comma-separated list of credentials(<user>:<pass>) for HTTP basic auth, which applies to all the backend routes. Presence of `dfp_users` Docker secret (`/run/secrets/dfp_users file`) overrides this setting. When present, credentials are read from it.<br>**Example:** `user1:pass1, user2:pass2`|
|USERS_PASS_ENCRYPTED| Indicates if passwords provided through `USERS` or Docker secret `dfp_users` (`/run/secrets/dfp_users` file) are encrypted. Passwords can be encrypted with the `mkpasswd -m sha-512 my-password` command.<br>**Example:** `true`<br>**Default value:** `false`|

## Debug Format

If debugging is enabled through the environment variable `DEBUG`, *Docker Flow Proxy* will log HTTP and TCP requests in addition to the event requests.

### HTTP Requests Debug Format

An example output of a debug log produced by an HTTP request is as follows.

```
HAPRoxy: 10.255.0.3:52662 [10/Mar/2017:13:18:47.759] services go-demo_main-be8080/go-demo_main 10/0/30/69/109 200 159 - - ---- 1/1/0/0/0 0/0 "GET /demo/random-error HTTP/1.1"
```

The format used for logging HTTP requests when the proxy is running in the debug mode is as follows.

|Field|Format                                                                                                  |Example                   |
|-----|--------------------------------------------------------------------------------------------------------|--------------------------|
|1    |Static text `HAProxy` indicating that the log entry comes directly from the proxy.                      |HAProxy                   |
|2    |Client IP and port. When used through Swarm networking, the IP and the port are of the *Ingress* network.|10.255.0.3:52662         |
|3    |Date and time when the request was accepted.                                                            |\[10/Mar/2017:13:18:47.759\]|
|4    |The name of the frontend.                                                                               |services                  |
|5    |Backend and server name. When used through Swarm networking, the server name is the name of the destination service.|go-demo_main-be8080/go-demo_main|
|6    |Total time in milliseconds spent waiting for a full HTTP request from the client (not counting body) after the first byte was received. It can be "-1" if the connection was aborted before a complete request could be received or a bad request was received. It should always be very small because a request generally fits in one single packet. Large times here generally indicate network issues between the client and haproxy or requests being typed by hand.|10|
|7    |Total time in milliseconds spent waiting in the various queues. It can be "-1" if the connection was aborted before reaching the queue.|0|
|8    |total time in milliseconds spent waiting for the connection to establish to the final server, including retries. It can be "-1" if the request was aborted before a connection could be established.|30|
|9    |Total time in milliseconds spent waiting for the server to send a full HTTP response, not counting data. It can be "-1" if the request was aborted before a complete response could be received. It generally matches the server's processing time for the request, though it may be altered by the amount of data sent by the client to the server. Large times here on "GET" requests generally indicate an overloaded server.|69|
|10   |The time the request remained active in haproxy, which is the total time in milliseconds elapsed between the first byte of the request was received and the last byte of response was sent. It covers all possible processing except the handshake and idle time.|109|
|11   |HTTP status code returned to the client. This status is generally set by the server, but it might also be set by proxy when the server cannot be reached or when its response is blocked by haproxy.|200|
|12   |The total number of bytes transmitted to the client when the log is emitted. This does include HTTP headers.|159                   |
|13   |An optional "name=value" entry indicating that the client had this cookie in the request. The field is a single dash ('-') when the option is not set. Only one cookie may be captured, it is generally used to track session ID exchanges between a client and a server to detect session crossing between clients due to application bugs.|-|
|14   |An optional "name=value" entry indicating that the server has returned a cookie with its response. The field is a single dash ('-') when the option is not set. Only one cookie may be captured, it is generally used to track session ID exchanges between a client and a server to detect session crossing between clients due to application bugs.|-|
|15   |The condition the session was in when the session ended. This indicates the session state, which side caused the end of session to happen, for what reason (timeout, error, ...), just like in TCP logs, and information about persistence operations on cookies in the last two characters. The normal flags should begin with "--", indicating the session was closed by either end with no data remaining in buffers.|----|
|16   |Total number of concurrent connections on the process when the session was logged. It is useful to detect when some per-process system limits have been reached. For instance, if actconn is close to 512 or 1024 when multiple connection errors occur, chances are high that the system limits the process to use a maximum of 1024 file descriptors and that all of them are used.|1|
|17   |The total number of concurrent connections on the frontend when the session was logged. It is useful to estimate the amount of resource required to sustain high loads, and to detect when the frontend's "maxconn" has been reached. Most often when this value increases by huge jumps, it is because there is congestion on the backend servers, but sometimes it can be caused by a denial of service attack.|1|
|18   |The total number of concurrent connections handled by the backend when the session was logged. It includes the total number of concurrent connections active on servers as well as the number of connections pending in queues. It is useful to estimate the amount of additional servers needed to support high loads for a given application. Most often when this value increases by huge jumps, it is because there is congestion on the backend servers, but sometimes it can be caused by a denial of service attack.|0|
|19   |The total number of concurrent connections still active on the server when the session was logged. It can never exceed the server's configured "maxconn" parameter. If this value is very often close or equal to the server's "maxconn", it means that traffic regulation is involved a lot, meaning that either the server's maxconn value is too low, or that there aren't enough servers to process the load with an optimal response time. When only one of the server's "srv_conn" is high, it usually means that this server has some trouble causing the requests to take longer to be processed than on other servers.|0|
|20  |The number of connection retries experienced by this session when trying to connect to the server. It must normally be zero, unless a server is being stopped at the same moment the connection was attempted. Frequent retries generally indicate either a network problem between haproxy and the server, or a misconfigured system backlog on the server preventing new connections from being queued. This field may optionally be prefixed with a '+' sign, indicating that the session has experienced a redispatch after the maximal retry count has been reached on the initial server. In this case, the server name appearing in the log is the one the connection was redispatched to, and not the first one, though both may sometimes be the same in case of hashing for instance. So as a general rule of thumb, when a '+' is present in front of the retry count, this count should not be attributed to the logged server.|0|
|21  |The total number of requests which were processed before this one in the server queue. It is zero when the request has not gone through the server queue. It makes it possible to estimate the approximate server's response time by dividing the time spent in queue by the number of requests in the queue. It is worth noting that if a session experiences a redispatch and passes through two server queues, their positions will be cumulated. A request should not pass through both the server queue and the backend queue unless a redispatch occurs.|0|
|22  |The total number of requests which were processed before this one in the backend's global queue. It is zero when the request has not gone through the global queue. It makes it possible to estimate the average queue length, which easily translates into a number of missing servers when divided by a server's "maxconn" parameter. It is worth noting that if a session experiences a redispatch, it may pass twice in the backend's queue, and then both positions will be cumulated. A request should not pass through both the server queue and the backend queue unless a redispatch occurs.|0|
|23  |The complete HTTP request line, including the method, request and HTTP version string. Non-printable characters are encoded. This field might be truncated if the request is huge and does not fit in the standard syslog buffer (1024 characters).|"GET /demo/random-error HTTP/1.1"|

### TCP Requests Debug Format

An example output of a debug log produced by a TCP request is as follows.

```
HAPRoxy: 10.255.0.3:55569 [10/Mar/2017:16:15:40.806] tcpFE_6379 redis_main-be6379/redis_main 0/0/5007 12 -- 0/0/0/0/0 0/0
```

The format used for logging TCP requests when the proxy is running in the debug mode is as follows.

|Field|Format                                                                                                  |Example                   |
|-----|--------------------------------------------------------------------------------------------------------|--------------------------|
|1    |Static text `HAProxy` indicating that the log entry comes directly from the proxy.                      |HAProxy                   |
|2    |Client IP and port. When used through Swarm networking, the IP and the port are of the *Ingress* network.|10.255.0.3:55569         |
|3    |Date and time when the request was accepted.                                                            |\[10/Mar/2017:13:18:47.759\]|
|4    |The name of the frontend.                                                                               |tcpFE_6379                |
|5    |Backend and server name. When used through Swarm networking, the server name is the name of the destination service.|redis_main-be6379/redis_main|
|6    |Total time in milliseconds spent waiting in the various queues. It can be "-1" if the connection was aborted before reaching the queue.|0|
|7    |total time in milliseconds spent waiting for the connection to establish to the final server, including retries. It can be "-1" if the request was aborted before a connection could be established.|0|
|8    |Total time in milliseconds spent waiting for the server to send a full HTTP response, not counting data. It can be "-1" if the request was aborted before a complete response could be received. It generally matches the server's processing time for the request, though it may be altered by the amount of data sent by the client to the server. Large times here on "GET" requests generally indicate an overloaded server.|5007|
|9    |Total number of bytes transmitted from the server to the client when the log is emitted.                |159                       |
|10   |The condition the session was in when the session ended. This indicates the session state, which side caused the end of session to happen, for what reason (timeout, error, ...), just like in TCP logs, and information about persistence operations on cookies in the last two characters. The normal flags should begin with "--", indicating the session was closed by either end with no data remaining in buffers.|--|
|11   |Total number of concurrent connections on the process when the session was logged. It is useful to detect when some per-process system limits have been reached. For instance, if actconn is close to 512 or 1024 when multiple connection errors occur, chances are high that the system limits the process to use a maximum of 1024 file descriptors and that all of them are used.|0|
|12   |The total number of concurrent connections on the frontend when the session was logged. It is useful to estimate the amount of resource required to sustain high loads, and to detect when the frontend's "maxconn" has been reached. Most often when this value increases by huge jumps, it is because there is congestion on the backend servers, but sometimes it can be caused by a denial of service attack.|0|
|13   |The total number of concurrent connections handled by the backend when the session was logged. It includes the total number of concurrent connections active on servers as well as the number of connections pending in queues. It is useful to estimate the amount of additional servers needed to support high loads for a given application. Most often when this value increases by huge jumps, it is because there is congestion on the backend servers, but sometimes it can be caused by a denial of service attack.|0|
|14   |The total number of concurrent connections still active on the server when the session was logged. It can never exceed the server's configured "maxconn" parameter. If this value is very often close or equal to the server's "maxconn", it means that traffic regulation is involved a lot, meaning that either the server's maxconn value is too low, or that there aren't enough servers to process the load with an optimal response time. When only one of the server's "srv_conn" is high, it usually means that this server has some trouble causing the requests to take longer to be processed than on other servers.|0|
|15  |The number of connection retries experienced by this session when trying to connect to the server. It must normally be zero, unless a server is being stopped at the same moment the connection was attempted. Frequent retries generally indicate either a network problem between haproxy and the server, or a misconfigured system backlog on the server preventing new connections from being queued. This field may optionally be prefixed with a '+' sign, indicating that the session has experienced a redispatch after the maximal retry count has been reached on the initial server. In this case, the server name appearing in the log is the one the connection was redispatched to, and not the first one, though both may sometimes be the same in case of hashing for instance. So as a general rule of thumb, when a '+' is present in front of the retry count, this count should not be attributed to the logged server.|0|
|16  |The total number of requests which were processed before this one in the server queue. It is zero when the request has not gone through the server queue. It makes it possible to estimate the approximate server's response time by dividing the time spent in queue by the number of requests in the queue. It is worth noting that if a session experiences a redispatch and passes through two server queues, their positions will be cumulated. A request should not pass through both the server queue and the backend queue unless a redispatch occurs.|0|
|17  |The total number of requests which were processed before this one in the backend's global queue. It is zero when the request has not gone through the global queue. It makes it possible to estimate the average queue length, which easily translates into a number of missing servers when divided by a server's "maxconn" parameter. It is worth noting that if a session experiences a redispatch, it may pass twice in the backend's queue, and then both positions will be cumulated. A request should not pass through both the server queue and the backend queue unless a redispatch occurs.|0|

## Secrets

Secrets can be used as a replacement for any of the environment variables. They should be prefixed with `dfp_` and written in lower case. As an example, `STATS_USER` environment variable would be specified as a secret `dfp_stats_user`.

Please read the [Proxy Statistics](https://proxy.dockerflow.com/swarm-mode-auto/#proxy-statistics) section for an example of using Docker secrets with the proxy.

## Custom Config

The base HAProxy configuration can be found in [haproxy.tmpl](haproxy.tmpl). It can be customized by creating a new image. An example *Dockerfile* is as follows.

```
FROM vfarcic/docker-flow-proxy
COPY haproxy.tmpl /cfg/tmpl/haproxy.tmpl
```

## Custom Errors

Default error messages are stored in the `/errorfiles` directory inside the *Docker Flow Proxy* image. They can be customized by creating a new image with custom error files or mounting a volume. Currently supported errors are `400`, `403`, `405`, `408`, `429`, `500`, `502`, `503`, and `504`.

## Statistics

Proxy statistics can be seen through **http://[NODE_IP_OR_DNS]/admin?stats**.

Please note that if you are running *Docker Flow Proxy* inside a Swarm cluster and with multiple replicas, Docker Ingress network will open one of the replicas only and every time you refresh the screen you'll be forwarded to a different replica.

If you'd like to exploit those statistics, I suggest you pull data into one of monitoring tools like Prometheus. You'll find the link to raw data inside the statistics UI.
