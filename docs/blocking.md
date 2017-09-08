# Blocking Requests

This article provides examples that can be used as a starting point when configuring the proxy to block requests based on their method type or protocol.

## Requirements

The examples that follow assume that you are using Docker v1.13+, Docker Compose v1.10+, and Docker Machine v0.9+.

!!! info
	If you are a Windows user, please run all the examples from *Git Bash* (installed through *Docker Toolbox*). Also, make sure that your Git client is configured to check out the code *AS-IS*. Otherwise, Windows might change carriage returns to the Windows format.

Please note that *Docker Flow Proxy* is not limited to *Docker Machine*. We're using it as an easy way to create a cluster.

## Setting Up A Swarm Cluster

To setup an example Swarm cluster using Docker Machine, please run the commands that follow.

!!! tip
	Feel free to skip this section if you already have a working Swarm cluster.

```bash
curl -o swarm-cluster.sh \
    https://raw.githubusercontent.com/\
vfarcic/docker-flow-proxy/master/scripts/swarm-cluster.sh

chmod +x swarm-cluster.sh

./swarm-cluster.sh

eval $(docker-machine env node-1)
```

## Deploying Services

Now we're ready to deploy the services that form the proxy stack and the demo services.

```bash
docker network create --driver overlay proxy

curl -o docker-compose-stack.yml \
    https://raw.githubusercontent.com/\
vfarcic/docker-flow-proxy/master/docker-compose-stack.yml

docker stack deploy -c docker-compose-stack.yml proxy

curl -o docker-compose-go-demo.yml \
    https://raw.githubusercontent.com/\
vfarcic/go-demo/master/docker-compose-stack.yml

docker stack deploy -c docker-compose-go-demo.yml go-demo
```

Please consult [Using Docker Stack To Run Docker Flow Proxy In Swarm Mode](/swarm-mode-stack/) for a more detailed set of examples of deployment with Docker stack.

We should wait until all the services are running before proceeding towards the examples that will block requests.

```bash
docker service ls
```

Now we are ready to explore how to block access requests.

## Blocking Requests Based on Request Type

In some cases, we want to deny certain types of methods to requests sent through the proxy. An example use case would be a service that can accept `GET` and `DELETE` request through the proxy and deny requests on any other method. That way, our front-end could send `GET` and `DELETE` requests through the proxy while any other request type would be allowed only through internal networking.

We can block requests by specifying which types are allowed.

Please execute the command that follows.

```
docker service update \
    --label-add "com.df.allowedMethods=GET,DELETE" \
    go-demo_main
```

We specified the `com.df.allowedMethods` label that tells the proxy that only `GET` and `DELETE` methods are allowed. A request with any other method will be denied.

Let's confirm that the feature indeed works as expected.

```bash
curl -i "http://$(docker-machine ip node-1)/demo/hello"
```

We sent an `GET` request (default type) and the output is as follows.

```
HTTP/1.1 200 OK
Date: Sat, 22 Jul 2017 13:35:55 GMT
Content-Length: 14
Content-Type: text/plain; charset=utf-8

hello, world!
```

Since `GET` is on the list of allowed request methods, we got OK (status code `200`) indicating that the proxy allowed it to pass to the destination service.

Let's confirm that the behavior is the same with a `DELETE` request.

```bash
curl -i -XDELETE \
    "http://$(docker-machine ip node-1)/demo/hello"
```

Just as with the `GET` request, the response is `200`. The proxy allowed it as well.

According to the current configuration, any other request method should be denied. Let's test it with, for example, a `PUT` request.

```bash
curl -i -XPUT \
    "http://$(docker-machine ip node-1)/demo/hello"
```

The output is as follows (limited to relevant parts).

```
HTTP/1.0 403 Forbidden
Cache-Control: no-cache
Connection: close
Content-Type: text/html
...
```

This time the proxy responded with `403` status code. The response comes from the proxy. The request method is not on the list of those that are allowed, and proxy chose not to forward it to the destination service. Instead, it returned `403 Forbidden`.

Similarly, we can choose which methods to deny.

```bash
docker service update \
    --label-rm "com.df.allowedMethods" \
    --label-add "com.df.deniedMethods=DELETE" \
    go-demo_main
```

We removed the `com.df.allowedMethods` label and created `com.df.deniedMethods` with the value `DELETE`.

If we send a `GET` request, the response should be `200` since it is not on the list of those that are denied.

```bash
curl -i \
    "http://$(docker-machine ip node-1)/demo/hello"
```

On the other hand, if we choose to send a `DELETE` request, the response should be denied.

```bash
curl -i -XDELETE \
    "http://$(docker-machine ip node-1)/demo/hello"
```

We got the response `403` thus confirming that no one can send a `DELETE` request to our service.

Let's remove the `deniedMethods` label and explore how we can block HTTP request.

```bash
docker service update \
    --label-rm "com.df.deniedMethods" \
    go-demo_main
```

## Blocking HTTP Requests

We can block any HTTP request thus allowing only HTTPS. The parameter that allows that is `denyHttp`.

```bash
docker service update \
    --label-add "com.df.denyHttp=true" \
    go-demo_main
```

Let's test whether HTTP requests are indeed denied.

```bash
curl -i "http://$(docker-machine ip node-1)/demo/hello"
```

The output is as follows (limited to relevant parts).

```bash
HTTP/1.0 403 Forbidden
Cache-Control: no-cache
Connection: close
Content-Type: text/html
...
```

From now on, only HTTPS requests are allowed. Typically, we'd prove that by sending an HTTPS request. However, we do not have a valid SSL certificate, so I'll leave that as an exercise. You might want to try [hamburml/docker-flow-letsencrypt](https://github.com/hamburml/docker-flow-letsencrypt). It is well integrated with *Docker Flow Proxy* and will provide you with the certificates from [Let's Encrypt](https://letsencrypt.org/).

## Summary

Please remove the machines we created and free your resources for some other tasks.

```bash
docker-machine rm node-1 node-2 node-3
```