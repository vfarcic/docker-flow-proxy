```bash
docker network create -d overlay proxy

TAG=beta docker stack deploy -c stack.yml proxy

docker stack ps proxy

docker container ls

ID=[...]

docker container exec -it $ID sh

ps aux

pkill haproxy
```