```bash
docker service create \
    --name dev-test \
    --network proxy \
    --label com.df.notify=true \
    --label com.df.distribute=true \
    --label com.df.serviceDomain=dev-test.example.com \
    --label com.df.port.1=80 \
    --label com.df.servicePath.1=/ \
    --label com.df.port.2=443 \
    --label com.df.srcPort.2=443 \
    --label com.df.reqMode.2=tcp \
    shebinbabu05/nginx-ssl

```