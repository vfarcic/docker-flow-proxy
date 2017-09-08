```bash
docker network create -d overlay proxy

docker stack deploy -c stack.yml proxy

# The stack opens port `8080` of the proxy only so that we can easily see the configuration by sending a request to `/v1/docker-flow-proxy/config`.

# Environment variable `REPEAT_RELOAD=true` is set on the `proxy` service. It'll reconfigure itself periodically. By default, the period is 5000 milliseconds and it can be changed with the environment variable `RELOAD_INTERVAL`.

# `go-demo` service has the label `com.df.sessionType=sticky-server`. That tells proxy that particular service should use sticky sessions set by the proxy (server). Other types of sticky sessions will be added later if there is demand.

docker stack ps proxy

# Wait until all the services are up and running.

curl "http://localhost:8080/v1/docker-flow-proxy/config"

# `go-demo` should have three IPs instead service name in the backend. It bypasses Overlay network and does it's own LB and it sets a cookie with the same name as the name of the service.

curl -i "http://localhost/demo/hello"

# There should be a response code 200 and text `hello, world!`

docker service scale proxy_go-demo-api=2

# Wait for 20 seconds. It takes up to 5 seconds to reload the config plus 10 seconds for new replicas to pass their health checks. It should work after only a few seconds but 20 is a sure bet.

curl "http://localhost:8080/v1/docker-flow-proxy/config"

# `go-demo` should have two IPs. Since it was scaled to two, the proxy detected that there are two replicas (instead of three) and reconfigured itself.

docker service scale proxy_go-demo-api=5

# Wait...

curl "http://localhost:8080/v1/docker-flow-proxy/config"

# `go-demo` should have five IPs. Since it was scaled to two, the proxy detected that there are two replicas (instead of three) and reconfigured itself.
```


```bash
docker network create -d overlay proxy

docker stack deploy -c stack2.yml proxy

docker stack ps proxy

docker service scale proxy_docs=2
```