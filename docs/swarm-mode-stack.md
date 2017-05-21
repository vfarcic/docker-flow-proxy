# Using Docker Stack To Run Docker Flow Proxy In Swarm Mode

This article assumes that you already understand how *Docker Flow Proxy* works. If you don't, please visit the [Running Docker Flow Proxy In Swarm Mode With Automatic Reconfiguration](swarm-mode-auto.md) page for a tutorial.

We'll explore how to create *Docker Flow Proxy* service through *Docker Compose* files and the `docker stack deploy` command.

## Requirements

The examples that follow assume that you are using Docker v1.13+, Docker Compose v1.10+, and Docker Machine v0.9+.

!!! info
	If you are a Windows user, please run all the examples from *Git Bash* (installed through *Docker Toolbox*). Also, make sure that your Git client is configured to check out the code *AS-IS*. Otherwise, Windows might change carriage returns to the Windows format.

Please note that *Docker Flow Proxy* is not limited to *Docker Machine*. We're using it as an easy way to create a cluster.

## Swarm Cluster Setup

To setup an example Swarm cluster using Docker Machine, please run the commands that follow.

!!! tip
	Feel free to skip this section if you already have a working Swarm cluster.

```bash
curl -o swarm-cluster.sh \
    https://raw.githubusercontent.com/\
vfarcic/docker-flow-proxy/master/scripts/swarm-cluster.sh

chmod +x swarm-cluster.sh

./swarm-cluster.sh

docker-machine ssh node-1
```

Now we're ready to deploy the `docker-flow-proxy` service.

## Running Docker Flow Proxy As A Docker Stack

We'll start by creating a network.

```bash
docker network create --driver overlay proxy
```

The *proxy* network will be dedicated to the proxy container and services that will be attached to it.

We'll use [docker-compose-stack.yml](https://github.com/vfarcic/docker-flow-proxy/blob/master/docker-compose-stack.yml) from the [vfarcic/docker-flow-proxy](https://github.com/vfarcic/docker-flow-proxy) repository to create `docker-flow-proxy` and `docker-flow-swarm-listener` services.

The content of the `docker-compose-stack.yml` file is as follows.

```
version: "3"

services:

  proxy:
    image: vfarcic/docker-flow-proxy
    ports:
      - 80:80
      - 443:443
    networks:
      - proxy
    environment:
      - LISTENER_ADDRESS=swarm-listener
      - MODE=swarm
    deploy:
      replicas: 2

  swarm-listener:
    image: vfarcic/docker-flow-swarm-listener
    networks:
      - proxy
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
    environment:
      - DF_NOTIFY_CREATE_SERVICE_URL=http://proxy:8080/v1/docker-flow-proxy/reconfigure
      - DF_NOTIFY_REMOVE_SERVICE_URL=http://proxy:8080/v1/docker-flow-proxy/remove
    deploy:
      placement:
        constraints: [node.role == manager]

networks:
  proxy:
    external: true
```

The format is written in version `3` (mandatory for `docker stack deploy`).

It contains two services; `proxy` and `swarm-listener`. Since you are already familiar with *Docker Flow Proxy* and *Docker Flow Swarm Listener*, the arguments used with the services should be self-explanatory.

The `proxy` network is defined as `external`. Even though `docker stack deploy` will create a `default` network for all the services that form the stack, the `proxy` network should be external so that we can attach services from other stacks to it.

Let's create the stack.

```bash
curl -o docker-compose-stack.yml \
    https://raw.githubusercontent.com/\
vfarcic/docker-flow-proxy/master/docker-compose-stack.yml

docker stack deploy -c docker-compose-stack.yml proxy
```

The first command downloaded the Compose file [docker-compose-stack.yml](https://github.com/vfarcic/docker-flow-proxy/blob/master/docker-compose-stack.yml) from the [vfarcic/docker-flow-proxy](https://github.com/vfarcic/docker-flow-proxy) repository. The second command created the services that form the stack.

The tasks of the stack can be seen through the `stack ps` command.

```bash
docker stack ps proxy
```

The output is as follows (IDs are removed for brevity).

```
NAME                   IMAGE                                     NODE   DESIRED STATE CURRENT STATE         ERROR  PORTS
proxy_proxy.1          vfarcic/docker-flow-proxy:latest          node-2 Running       Running 2 minutes ago
proxy_swarm-listener.1 vfarcic/docker-flow-swarm-listener:latest node-1 Running       Running 2 minutes ago
proxy_proxy.2          vfarcic/docker-flow-proxy:latest          node-3 Running       Running 2 minutes ago
```

We are running two replicas of the `proxy` (for high-availability in the case of a failure) and one of the `swarm-listener`.

## Deploying Services Alongside Docker Flow Proxy

Let's deploy a demo stack. It consists of two containers; *mongo* is the database, and *vfarcic/go-demo* is the service that uses it.

We'll use Docker stack defined in the Compose file [docker-compose-stack.yml](https://github.com/vfarcic/go-demo/blob/master/docker-compose-stack.yml) located in the [vfarcic/go-demo](https://github.com/vfarcic/go-demo/) repository. It is as follows.

```
version: '3'

services:

  main:
    image: vfarcic/go-demo
    environment:
      - DB=db
    networks:
      - proxy
      - default
    deploy:
      replicas: 3
      labels:
        - com.df.notify=true
        - com.df.distribute=true
        - com.df.servicePath=/demo
        - com.df.port=8080

  db:
    image: mongo
    networks:
      - default

networks:
  default:
    external: false
  proxy:
    external: true
```

The stack defines two services (`main` and `db`). They will communicate with each other through the `default` network that will be created automatically by the stack. Since the `main` service is an API, it should be accessible through the proxy, so we're attaching `proxy` network as well. The `main` service defines four service labels. They are the same labels you used in the [Running Docker Flow Proxy In Swarm Mode With Automatic Reconfiguration](swarm-mode-auto.md) tutorial.

!!! tip
    Don't confuse **service** with **container** labels. The syntax is the same with the difference that service labels are inside the `deploy` section. *Docker Flow Swarm Listener* supports only service labels.

```bash
curl -o docker-compose-go-demo.yml \
    https://raw.githubusercontent.com/\
vfarcic/go-demo/master/docker-compose-stack.yml

docker stack deploy -c docker-compose-go-demo.yml go-demo

docker stack ps go-demo
```

We downloaded the stack definition, executed `stack deploy` command that created the services and run the `stack ps` command that lists the tasks that belong to the `go-demo` stack. The output is as follows (IDs are removed for brevity).

```
NAME           IMAGE                  NODE    DESIRED STATE CURRENT STATE          ERROR PORTS
go-demo_main.1 vfarcic/go-demo:latest node-2 Running        Running 7 seconds ago
...
go-demo_db.1   mongo:latest           node-2 Running        Running 21 seconds ago
go-demo_main.2 vfarcic/go-demo:latest node-2 Running        Running 19 seconds ago
...
go-demo_main.3 vfarcic/go-demo:latest node-2 Running        Running 20 seconds ago
...
```

Since Mongo database is much bigger than the `main` service, it takes more time to pull it, resulting in a few failures. The `go-demo` service is designed to fail if it cannot connect to its database. Once the `db` service is running, the `main` service should stop failing, and we'll see three replicas with the current state `Running`.

After a few moments, the `swarm-listener` service will detect the `main` service from the `go-demo` stack and send the `proxy` a request to reconfigure itself. We can see the result by sending an HTTP request to the proxy.

```bash
curl -i "$(docker-machine ip node-1)/demo/hello"
```

The output is as follows.

```
HTTP/1.1 200 OK
Date: Thu, 19 Jan 2017 23:57:05 GMT
Content-Length: 14
Content-Type: text/plain; charset=utf-8

hello, world!
```

The proxy was reconfigured and forwards all requests with the base path `/demo` to the `main` service from the `go-demo` stack.

For more advanced usage of the proxy, please see the examples from [Running Docker Flow Proxy In Swarm Mode With Automatic Reconfiguration](swarm-mode-auto.md) tutorial or consult the [configuration](config.md) and [usage](usage.md) documentation.

## Cleanup

Please remove Docker Machine VMs we created. You might need those resources for some other tasks.

```bash
exit

docker-machine rm -f node-1 node-2 node-3
```
