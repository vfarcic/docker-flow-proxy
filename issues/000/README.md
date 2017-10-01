```
docker network create -d overlay proxy

docker stack deploy -c stack.yml proxy

open "http://localhost"

curl "http://localhost"

curl -H 'Host: zarbis.tk' "http://localhost"

curl -i -H 'Host: zarbis.tk' "https://localhost"
```