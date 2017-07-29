```bash
docker network create -d overlay proxy

docker stack deploy -c stack.yml proxy

watch "docker stack ps proxy"

curl "http://localhost:8080/v1/docker-flow-proxy/config"

curl "http://localhost/demo/hello"

docker service scale proxy_go-demo-api=2

curl "http://localhost/demo/hello"

curl "http://localhost:8080/v1/docker-flow-proxy/config"

curl "http://localhost:8080/v1/docker-flow-proxy/reload?fromListener=true"

docker service logs proxy_proxy

curl "http://localhost:8080/v1/docker-flow-proxy/config"

docker service scale proxy_go-demo-api=5

curl "http://localhost:8080/v1/docker-flow-proxy/config"

curl "http://localhost:8080/v1/docker-flow-proxy/reload?fromListener=true"

curl "http://localhost:8080/v1/docker-flow-proxy/config"
```