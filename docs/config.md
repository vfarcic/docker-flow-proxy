# Configuring Docker Flow Proxy

*Docker Flow Proxy* can be configured through Docker environment variables and/or by creating a new image based on `vfarcic/docker-flow-proxy`.

## Environment Variables

!!! tip
	The *Docker Flow Proxy* container can be configured through environment variables

The following environment variables can be used to configure the *Docker Flow Proxy*.

|Variable           |Description                                               |Required|Default|Example|
|-------------------|----------------------------------------------------------|--------|-------|-------|
|BIND_PORTS         |Ports to bind in addition to `80` and `443`. Multiple values can be separated with comma|No| |8085, 8086|
|CERTS              |This parameter is **deprecated** as of February 2017. All the certificates from the `/cets/` directory are now loaded automatically| | | |
|CONNECTION_MODE    |HAProxy supports 5 connection modes.<br><br>`http-keep-alive`: all requests and responses are processed.<br>`http-tunnel`: only the first request and response are processed, everything else is forwarded with no analysis.<br>`httpclose`: tunnel with "Connection: close" added in both directions.<br>`http-server-close`: the server-facing connection is closed after the response.<br>`forceclose`: the connection is actively closed after end of response.<br><br>In general, it is preferred to use `http-server-close` with application servers, and some static servers might benefit from `http-keep-alive`.|No|http-server-close|http-keep-alive|
|DEBUG              |Enables logging of each request sent through the proxy. Please consult [Debug Format](#debug-format) for info about the log entries. This feature should be used with caution. **Do not enable debugging in production unless necessary.**|No|false|true|
|DEBUG_ERRORS_ONLY  |If set to `true`, only requests that resulted in an error, timeout, retry, and redispatch will be logged. If a request is HTTP, responses with a status 5xx will be logged too. This variable will take effect only if `DEBUG` is set to `true`.|No|false|true|
|DEBUG_HTTP_FORMAT  |Logging format that will be used with HTTP requests. Please consult [Custom log format](https://cbonte.github.io/haproxy-dconv/1.7/configuration.html#8.2.4) for more info about the available options.|No| | |
|DEBUG_TCP_FORMAT   |Logging format that will be used with TCP requests. Please consult [Custom log format](https://cbonte.github.io/haproxy-dconv/1.7/configuration.html#8.2.4) for more info about the available options.|No| | |
|DEFAULT_PORTS      |The default ports used by the proxy. Multiple values can be separated with comma (`,`). If a port should be for SSL connections, append it with `:ssl.|No|80,443:ssl| |
|EXTRA_FRONTEND     |Value will be added to the default `frontend` configuration.|No    | | |
|EXTRA_GLOBAL       |Value will be added to the default `global` configuration.|No      | | |
|LISTENER_ADDRESS   |The address of the [Docker Flow: Swarm Listener](https://github.com/vfarcic/docker-flow-swarm-listener) used for automatic proxy configuration.|Only in the *swarm* mode| |swarm-listener|
|MODE               |Two modes are supported. The *default* mode should be used for general purpose. **This mode is deprecated and will be removed soon**. The *swarm* mode is designed to work with new features introduced in Docker 1.12 and assumes that containers are deployed as Docker services (new Swarm).|No      |default|swarm|
|PROXY_INSTANCE_NAME|The name of the proxy instance. Useful if multiple proxies are running inside a cluster|No|docker-flow|docker-flow|
|SERVICE_NAME       |The name of the service. It must be the same as the value of the `--name` argument used to create the proxy service. Used only in the *swarm* mode.|No|proxy|my-proxy|
|SKIP_ADDRESS_VALIDATION|Whether to skip validating service address before reconfiguring the proxy.|No|false|true|
|STATS_USER         |Username for the statistics page. If not set, stats will not be available.|No      |admin  |my-user|
|STATS_USER_ENV     |The name of the environment variable that holds the username for the statistics page|No|STATS_USER|MY_USER|
|STATS_PASS         |Password for the statistics page. If not set, stats will not be available.|No      |admin  |my-pass|
|STATS_PASS_ENV     |The name of the environment variable that holds the password for the statistics page|No|STATS_PASS|MY_PASS|
|TIMEOUT_CLIENT     |The client timeout in seconds                             |No      |20     |5      |
|TIMEOUT_CONNECT    |The connect timeout in seconds                            |No      |5      |3      |
|TIMEOUT_QUEUE      |The queue timeout in seconds                              |No      |30     |10     |
|TIMEOUT_SERVER     |The server timeout in seconds                             |No      |20     |5      |
|TIMEOUT_TUNNEL     |The tunnel timeout in seconds                             |No      |3600   |1800   |
|TIMEOUT_HTTP_REQUEST|The HTTP request timeout in seconds                      |No      |5      |3      |
|TIMEOUT_HTTP_KEEP_ALIVE|The HTTP keep alive timeout in seconds                |No      |15     |10     |
|USERS              |A comma-separated list of credentials(<user>:<pass>) for HTTP basic auth, which applies to all the backend routes. Presence of `dfp_users` Docker secret (`/run/secrets/dfp_users file`) overrides this setting. When present, credentials are read from it. |No| |user1:pass1, user2:pass2|
|USERS_PASS_ENCRYPTED| Indicates if passwords provided through USERS or Docker secret `dfp_users` (`/run/secrets/dfp_users` file) are encrypted. Passwords can be encrypted with the `mkpasswd -m sha-512 my-password` command |No| false |true|

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
|6    |Total time in milliseconds spent waiting for a full HTTP request from the client (not counting body) after the first byte was received. It can be "-1" if the connection was aborted before a complete request could be received or the a bad request was received. It should always be very small because a request generally fits in one single packet. Large times here generally indicate network issues between the client and haproxy or requests being typed by hand.|10|
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