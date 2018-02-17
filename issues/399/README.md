```bash
docker network create --driver overlay proxy

docker network create --driver overlay hidden

docker service create --name swarm-listener \
    --network proxy \
    --mount "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock" \
    -e DF_NOTIFY_CREATE_SERVICE_URL=http://proxy:8080/v1/docker-flow-proxy/reconfigure \
    -e DF_NOTIFY_REMOVE_SERVICE_URL=http://proxy:8080/v1/docker-flow-proxy/remove \
    --constraint 'node.role==manager' \
    vfarcic/docker-flow-swarm-listener

docker service create --name proxy \
    -p 80:80 \
    -p 443:443 \
    --network proxy \
    -e LISTENER_ADDRESS=swarm-listener \
    vfarcic/docker-flow-proxy

echo "admin" | docker secret create root_db_password -

echo "admin" | docker secret create wp_db_password -

docker service create \
    --name mariadb \
    --network hidden \
    --secret source=root_db_password,target=root_db_password \
    --secret source=wp_db_password,target=wp_db_password \
    -e MYSQL_ROOT_PASSWORD_FILE=/run/secrets/root_db_password \
    -e MYSQL_PASSWORD_FILE=/run/secrets/wp_db_password \
    -e MYSQL_USER=wp \
    -e MYSQL_DATABASE=wp \
    mariadb:10.2

docker service create \
    --name dev-cman \
    --network proxy \
    --network hidden \
    --secret source=wp_db_password,target=wp_db_password,mode=0400 \
    --label com.df.notify=true \
	--label com.df.servicePath=/ \
	--label com.df.serviceDomain=localhost \
	--label com.df.port=80 \
    -e WORDPRESS_DB_USER=wp \
    -e WORDPRESS_DB_PASSWORD_FILE=/run/secrets/wp_db_password \
    -e WORDPRESS_DB_HOST=mariadb \
    -e WORDPRESS_DB_NAME=wp \
    wordpress:4.7

docker service update --publish-add 8080:80 dev-cman

curl -i -L "http://localhost:8080"

curl -i -L "http://localhost"

open "http://localhost:8080"

open "http://localhost"
```