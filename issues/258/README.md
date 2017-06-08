```bash
docker network create -d overlay proxy

docker stack deploy -c proxy.yml proxy

docker stack deploy -c stack.yml sa

curl "http://localhost:8080/v1/docker-flow-proxy/config"

open "http://localhost"

open "http://localhost:8081/counter"

open "http://localhost/counter"
```


```bash
docker network create --driver overlay proxy

docker stack deploy -c proxy.yml proxy

docker stack deploy -c go-demo.yml go-demo

curl "http://localhost/demo/hello"

curl "http://localhost:8080/v1/docker-flow-proxy/config"

docker service logs proxy_proxy
```