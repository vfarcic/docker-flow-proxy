Docker Flow: Proxy - Swarm Mode (Docker 1.12+)
==============================================

*Docker Flow: Proxy* running in the *swarm* mode is designed to leverage the features introduced in Docker v1.12. If you are looking for a proxy solution that would work with older Docker versions, please explore the [Docker Flow: Proxy - Standard Mode](standard-mode.md) article.

Examples
--------

The examples that follow assume that you have Docker Machine version v0.8+ that includes Docker Engine v1.12+. The easiest way to get them is through [Docker Toolbox](https://www.docker.com/products/docker-toolbox).

> If you are a Windows user, please run all the examples from *Git Bash* (installed through *Docker Toolbox*).

Please note that *Docker Flow: Proxy* is not limited to *Docker Machine*. We're using it as an easy way to create a cluster.

### Setup

To setup an example environment using Docker Machine, please run the commands that follow.

```bash
git clone https://github.com/vfarcic/docker-flow-proxy.git

cd docker-flow-proxy

chmod +x scripts/swarm-cluster.sh

scripts/swarm-cluster.sh
```

Right now we have three machines running (*node-1*, *node-2*, and *node-3*). Each of those machines runs Docker Engine. Together, they form a Swarm cluster. Docker Engine running in the first node (*node-1*) is the leader.

We can see the cluster status by running the following command.

```bash
eval $(docker-machine env node-1)

docker node ls
```

We'll skip a detailed explanation of the Swarm cluster that is incorporated into Docker Engine 1.12. If you're new to it, please read [TODO](TODO). The rest of this article will assume that you have, at least, basic Docker 1.12+ knowledge.

Now we're ready to deploy a service.

### Automatically Reconfiguring the Proxy

We'll start by creating two networks.

```bash
docker network create --driver overlay proxy

docker network create --driver overlay go-demo
```

The first (*proxy*) will be dedicated to the proxy container and services that should be exposed through it. The second (*go-demo*) is the network used for communications between containers that constitute the *go-demo* service.

Let's deploy the demo service. It consists of two containers; *mongo* is the database and *vfarcic/go-demo* is the actual service that uses it. They will communicate with each other through the *go-demo* network. Since we want to expose only *vfarcic/go-demo* to the "outside" world and keep the database "private", only the *vfarcic/go-demo* container will attach itself to the *proxy* network.

```bash
docker service create --name go-demo-db \
  -p 27017 \
  --network go-demo \
  mongo

docker service create --name go-demo \
  -p 8080 \
  -e DB=go-demo-db \
  --network go-demo \
  --network proxy \
  vfarcic/go-demo
```

We can see the status of those containers by executing the `service ls` command.

```bash
docker service ls
```

Please wait until both services are having replicas set to *1/1*.

The details of the *go-demo* service are irrelevant for this exercise. What matters is that it was deployed somewhere inside the cluster and that it does not have any port exposed outside of the networks *go-demo* and *proxy*.

The only thing missing now is to reconfigure the proxy so that our newly deployed service is accessible on a standard HTTP port *80*. That is the problem *Docker Flow: Proxy* is solving.

```bash
docker service create --name proxy \
    -p 80:80 \
    -p 443:443 \
    -p 8080:8080 \
    --network proxy \
    -e MODE=swarm \
    --constraint node.id==$(docker node inspect node-1 --format "{{.ID}}") \
    vfarcic/docker-flow-proxy
```

We opened ports *80* and *443*. External requests will be routed through them towards the destination services. The third port (*8080*) will be used to send requests to the proxy specifying what it should do. The proxy it belongs to the *proxy* network and has the mode set to *swarm*. Finally, we're using the `--constraint` argument as a way to ensure that the proxy is running on a specific server.

As before, please use the `docker service ls` command to check that the container is running (replicas set to 1/1) before proceeding with the rest of the article.

Now that the proxy is running, we can tell it to include the *go-demo* service in its configuration.

```bash
curl "$(docker-machine ip node-1):8080/v1/docker-flow-proxy/reconfigure?serviceName=go-demo&servicePath=/demo&port=8080"
```

That's it. All we had to do is send an HTTP request to `reconfigure` the proxy. The `serviceName` query contains the name of the service we want to integrate with the proxy. The `servicePath` is the unique URL that identifies the service. Finally, the `port` should match the internal port of the service.

The output of the reconfigure request is as follows (formatted for better readability).

```json
{
  "Status": "OK",
  "Message": "",
  "ServiceName": "go-demo",
  "ServiceColor": "",
  "ServicePath": [
    "/demo"
  ],
  "ServiceDomain": "",
  "ConsulTemplateFePath": "",
  "ConsulTemplateBePath": "",
  "PathType": "",
  "SkipCheck": false,
  "Mode": "service",
  "Port": "8080"
}
```

*Docker Flow: Proxy* responded saying that reconfiguration of the service *go-demo* running on the path */demo* was performed successfully.

Let's see whether the service is indeed accessible through the proxy.

```bash
curl -i $(docker-machine ip node-1)/demo/hello
```

The output of the `curl` command is as follows.

```bash
HTTP/1.1 200 OK
Date: Thu, 07 Jul 2016 23:14:47 GMT
Content-Length: 14
Content-Type: text/plain; charset=utf-8

hello, world!
```

The response is *200 OK*, meaning that our service is indeed accessible through the proxy. All we had to do is tell *docker-flow-proxy* the name of the service.

Since *Docker Flow: Proxy* uses new networking features added to Docker 1.12, it redirects all requests to the internally created SDN. As a result, Docker takes care of load balancing, so there is no need to reconfigure the proxy every time a new instance is deployed. We can confirm that by creating a few additional replicas.

```bash
docker service update --replicas 5 go-demo

curl -i $(docker-machine ip node-1)/demo/hello
```

Feel free to repeat this request a few more times. Once done, check the logs of any of the replicas and you'll notice that it received approximately one-fifth of the requests. No matter how many instances are running and with which frequency they change, swarm network will make sure that requests are load balanced across all currently running instances.

*Docker Flow: Proxy* reconfiguration is not limited to a single *service path*. Multiple values can be divided by comma (*,*). For example, our service might expose multiple versions of the API. In such a case, an example reconfiguration request could be as follows.

```bash
curl "$(docker-machine ip node-1):8080/v1/docker-flow-proxy/reconfigure?serviceName=go-demo&servicePath=/demo/hello,/demo/person&port=8080"
```

The result from the `curl` request is the reconfiguration of the *HAProxy* so that the *go-demo* service can be accessed through both the */demo/hello* and the */demo/person* paths.

Optionally, *serviceDomain* can be used as well. If specified, the proxy will allow access only to requests coming from that domain. The example that follows sets *serviceDomain* to *my-domain.com*. After the proxy is reconfigured, only requests for that domain will be redirected to the destination service.

```bash
curl "$(docker-machine ip node-1):8080/v1/docker-flow-proxy/reconfigure?serviceName=go-demo&servicePath=/demo&serviceDomain=my-domain.com&port=8080"
```

For a more detailed example, please read the [TODO](TODO) article.

### Removing a Service From the Proxy

We can as easily remove a service from the *Docker Flow: Proxy*. An example that removes the service *go-demo* is as follows.

```bash
curl "$(docker-machine ip node-1):8080/v1/docker-flow-proxy/remove?serviceName=go-demo"
```

From this moment on, the service *go-demo* is not available through the proxy.

### Usage

Please explore [Usage][../README.md#usage] for more information.
