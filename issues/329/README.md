```bash
docker stack deploy -c stack.yml proxy

docker stack ps -f desired-state=running proxy

# All services are up

docker service logs proxy_proxy

curl -i "http://localhost/demo/hello"

# Response: 200 OK

PROXY_TAG=17.09.14-14 docker stack deploy -c stack.yml proxy

docker stack ps -f desired-state=running proxy

# Proxy was updated

docker service logs proxy_proxy

curl "http://localhost/demo/hello"

# Response: 200 OK

SERVICE_TAG=2.0 PROXY_TAG=beta docker stack deploy -c stack.yml proxy

docker stack ps -f desired-state=running proxy

# myservice was updated

docker service logs proxy_proxy

curl "http://localhost/demo/hello"

SERVICE_TAG=3.0 PROXY_TAG=17.09.14-14 docker stack deploy -c stack.yml proxy

SERVICE_TAG=4.0 PROXY_TAG=17.09.14-15 docker stack deploy -c stack.yml proxy

docker stack ps -f desired-state=running proxy

# myservice was updated

docker service logs proxy_proxy

curl "http://localhost/demo/hello"
```