# Running Docker Flow Proxy In Swarm Mode With Automatic Reconfiguration

*Docker Flow Proxy* running in the *Swarm Mode* is designed to leverage the features introduced in *Docker v1.12+*.

The examples that follow assume that you have Docker Machine version v0.8+ that includes Docker Engine v1.12+. The easiest way to get them is through [Docker Toolbox](https://www.docker.com/products/docker-toolbox).

!!! info
	If you are a Windows user, please run all the examples from *Git Bash* (installed through *Docker Toolbox*). Also, make sure that your Git client is configured to check out the code *AS-IS*. Otherwise, Windows might change carriage returns to the Windows format.

Please note that *Docker Flow Proxy* is not limited to *Docker Machine*. We're using it as an easy way to create a cluster.

## Setup

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

We'll skip a detailed explanation of the Swarm cluster that is incorporated into Docker Engine 1.12. If you're new to it, please read [Docker Swarm Introduction](https://technologyconversations.com/2016/07/29/docker-swarm-introduction-tour-around-docker-1-12-series/). The rest of this article will assume that you have, at least, basic Docker 1.12+ knowledge.

Now we're ready to deploy a service.

## Automatically Reconfiguring the Proxy

We'll start by creating two networks.

```bash
docker network create --driver overlay proxy

docker network create --driver overlay go-demo
```

The first network (*proxy*) will be dedicated to the proxy container and services that should be exposed through it. The second (*go-demo*) is the network used for communications between containers that constitute the *go-demo* service.

Next, we'll create the [swarm-listener](https://github.com/vfarcic/docker-flow-swarm-listener) service. It is companion to the `Docker Flow: Proxy`. Its purpose is to monitor Swarm services and send requests to the proxy whenever a service is created or destroyed.

Let's create the `swarm-listener` service.

!!! info
	**A note to Windows users**
	
	For mounts to work, you will have to enter one of the machines before executing the `docker service create` command to work. To enter the Docker Machine, execute the `docker-machine ssh node-1` command. Please exit the machine once you finish executing the command that follows.

```bash
docker service create --name swarm-listener \
    --network proxy \
    --mount "type=bind,source=/var/run/docker.sock,target=/var/run/docker.sock" \
    -e DF_NOTIFY_CREATE_SERVICE_URL=http://proxy:8080/v1/docker-flow-proxy/reconfigure \
    -e DF_NOTIFY_REMOVE_SERVICE_URL=http://proxy:8080/v1/docker-flow-proxy/remove \
    --constraint 'node.role==manager' \
    vfarcic/docker-flow-swarm-listener
```

The service is attached to the proxy network, mounts the Docker socket, and declares the environment variables `DF_NOTIFY_CREATE_SERVICE_URL` and `DF_NOTIFY_REMOVE_SERVICE_URL`. We'll see the purpose of the variables soon. The service is constrained to the `manager` nodes.

The next step is to create the proxy service.

```bash
docker service create --name proxy \
    -p 80:80 \
    -p 443:443 \
    --network proxy \
    -e MODE=swarm \
    -e LISTENER_ADDRESS=swarm-listener \
    vfarcic/docker-flow-proxy
```

We opened the ports *80* and *443*. External requests will be routed through them towards destination services. The proxy is attached to the *proxy* network and has the mode set to *swarm*. The proxy must belong to the same network as the listener. They will exchange information whenever a service is created or removed.

!!! info
	If you name the service something other than *proxy*, you have to pass the environment variable `SERVICE_NAME` on creation. The value of `SERVICE_NAME` has to be the same as the name of the service.

Let's deploy the demo service. It consists of two containers; *mongo* is the database and *vfarcic/go-demo* is the actual service that uses it. They will communicate with each other through the *go-demo* network. Since we want to expose only *vfarcic/go-demo* to the "outside" world and keep the database "private", only the *vfarcic/go-demo* container will attach itself to the *proxy* network.

```bash
docker service create --name go-demo-db \
    --network go-demo \
    mongo
```

Let's run up the second service.

```bash
docker service create --name go-demo \
    -e DB=go-demo-db \
    --network go-demo \
    --network proxy \
    --label com.df.notify=true \
    --label com.df.distribute=true \
    --label com.df.servicePath=/demo \
    --label com.df.port=8080 \
    vfarcic/go-demo
```

The details of the *go-demo* service are irrelevant for this exercise. What matters is that it was deployed somewhere inside the cluster and that it does not have any port exposed outside of the networks *go-demo* and *proxy*.

Please note the labels. They are a crucial part of the service definition. The `com.df.notify=true` tells the `Swarm Listener` whether to send a notifications whenever a service is created or removed. The rest of the labels match the query arguments we would use if we'd reconfigure the proxy manually. The only difference is that the labels are prefixed with `com.df`. For the list of the query arguments, please see the [Reconfigure](usage.md#reconfigure) section.

Now we should wait until all the services are running. You can see their status by executing the command that follows.

```bash
docker service ls
```

Once all the replicas are set to `1/1`, we can see the effect of the `com.df` labels by sending a request to the `go-demo` service through the proxy.

```bash
curl -i $(docker-machine ip node-1)/demo/hello
```

The output is as follows.

```
HTTP/1.1 200 OK
Date: Thu, 13 Oct 2016 18:26:18 GMT
Content-Length: 14
Content-Type: text/plain; charset=utf-8

hello, world!
```

We sent a request to the proxy (the only service listening to the port 80) and got back the response from the `go-demo` service. The proxy was configured automatically as soon as the `go-demo` service was created.

The way the process works is as follows.

[Docker Flow: Swarm Listener](https://github.com/vfarcic/docker-flow-swarm-listener) is running inside one of the Swarm manager nodes and queries Docker API in search for newly created services. Once it finds a new service, it looks for its labels. If the service contains the `com.df.notify` (it can hold any value), the rest of the labels with keys starting with `com.df.` are retrieved. All those labels are used to form request parameters. Those parameters are appended to the address specified as the `DF_NOTIFY_CREATE_SERVICE_URL` environment variable defined in the `swarm-listener` service. Finally, a request is sent. In this particular case, the request was made to reconfigure the proxy with the service `go-demo` (the name of the service), using `/demo` as the path, and running on the port `8080`. The `distribute` label is not necessary in this example since we're running only a single instance of the proxy. However, in production we should run at least two proxy instances (for fault tolerance) and the `distribute` argument means that reconfiguration should be applied to all.

Please see the [Reconfigure](usage.md#reconfigure) section for the list of all the arguments that can be used with the proxy.

Since *Docker Flow: Proxy* uses new networking features added to Docker 1.12, it redirects all requests to the Swarm SDN (in this case called `proxy`). As a result, Docker takes care of load balancing, so there is no need to reconfigure it every time a new instance is deployed. We can confirm that by creating a few additional replicas.

```bash
docker service update --replicas 5 go-demo

curl -i $(docker-machine ip node-1)/demo/hello
```

Feel free to repeat this request a few more times. Once done, check the logs of any of the replicas and you'll notice that it received approximately one-fifth of the requests. No matter how many instances are running and with which frequency they change, Swarm networking will make sure that requests are load balanced across all currently running instances.

*Docker Flow: Proxy* reconfiguration is not limited to a single *service path*. Multiple values can be divided by comma (*,*). For example, our service might expose multiple versions of the API. In such a case, an example `servicePath` label attached to the `go-demo`service could be as follows.

```bash
...
  --label com.df.servicePath=/demo/hello,/demo/person \
...
```

Optionally, *serviceDomain* can be used as well. If specified, the proxy will allow access only to requests coming from that domain. The example label that follows would set *serviceDomain* to *my-domain.com*. After the proxy is reconfigured, only requests for that domain will be redirected to the destination service.

```bash
...
  --label com.df.serviceDomain=my-domain.com \
...
```

Multiple domains should be separated with comma (`,`).

```bash
...
  --label com.df.serviceDomain=my-domain.com,my-other-domain.com \
...
```

Domains can be prefixed with a wildcard.

```bash
...
  --label "com.df.serviceDomain=*domain.com" \
...
```

The above example would match any domain ending with `domain.com` (e.g. `my-domain.com`, `my-other-domain.com`, etc).

## Removing a Service From the Proxy

Since `Swarm Listener` is monitoring docker services, if a service is removed, related entries in the proxy configuration will be removed as well.

```bash
docker service rm go-demo
```

If you check the `Swarm Listener` logs, you'll see an entry similar to the one that follows.

```
Sending service removed notification to http://proxy:8080/v1/docker-flow-proxy/remove?serviceName=go-demo
```

A moment later, a new entry would appear in the proxy logs.

```
Processing request /v1/docker-flow-proxy/remove?serviceName=go-demo
Processing remove request /v1/docker-flow-proxy/remove
Removing go-demo configuration
Removing the go-demo configuration files
Reloading the proxy
```

From this moment on, the service *go-demo* is not available through the proxy.

`Swarm Listener` detected that the service was removed, send a notification to the proxy which, in turn, changed its configuration and reloaded underlying HAProxy.

Now that you've seen how to automatically add and remove services from the proxy, let's take a look at scaling options.

## Scaling the Proxy

Swarm is continuously monitoring containers health. If one of them fails, it will be redeployed to one of the nodes. If a whole node fails, Swarm will recreate all the containers that were running on that node. The ability to monitor containers health and make sure that they are (almost) always running is not enough. There is a brief period between the moment an instance fails until Swarm detects that and instantiates a new one. If we want to get close to zero-downtime systems, we must scale our services to at least two instances running on different nodes. That way, while we're waiting for one instance to recuperate from a failure, the others can take over its load. Even that is not enough. We need to make sure that the state of the failed instance is recuperated.

Let's see how *Docker Flow: Proxy* behaves when scaled.

Before we scale the proxy, we'll recreate the `go-demo` service that we removed a few moments ago.

```bash
docker service create --name go-demo \
  -e DB=go-demo-db \
  --network go-demo \
  --network proxy \
  --label com.df.notify=true \
  --label com.df.distribute=true \
  --label com.df.servicePath=/demo \
  --label com.df.port=8080 \
  --replicas 3 \
  vfarcic/go-demo
```

We should wait until the `go-demo` service is up and running. We can check the status by executing `service ps` command.

```bash
docker service ps go-demo
```

At the moment we are still running a single instance of the proxy. Before we scale it, let's confirm that the listener sent a request to reconfigure it.

```bash
curl -i $(docker-machine ip node-1)/demo/hello
```

The output should be status `200` indicating that the proxy works.

Let's scale the proxy to three instances.

```bash
docker service scale proxy=3
```

The proxy was scaled to three instances.

Normally, creating a new instance means that it starts without a state. As a result, the new instances would not have the `go-demo` service configured. Having different states among instances would produce quite a few undesirable effects. This is where the environment variable `LISTENER_ADDRESS` comes into play.

If you go back to the command we used to create the `proxy` service, you'll notice the argument that follows.

```bash
    -e LISTENER_ADDRESS=swarm-listener \
```

This tells the proxy the address of the [Docker Flow: Swarm Listener](https://github.com/vfarcic/docker-flow-swarm-listener) service. Whenever a new instance of the proxy is created, it will send a request to the listener to resend notifications for all the services. As a result, each proxy instance will soon have the same state as the other.

If, for example, an instance of the proxy fails, Swarm will reschedule it and, soon afterwards, a new instance will be created. In that case, the process would be the same as when we scaled the proxy and, as the end result, the rescheduled instance will also have the same state as any other.

To test whether all the instances are indeed having the same configuration, we can send a couple of requests to the *go-demo* service.

Please run the command that follows a couple of times.

```bash
curl -i $(docker-machine ip node-1)/demo/hello
```

Since Docker's networking (`routing mesh`) is performing load balancing, each of those requests is sent to a different proxy instance. Each was forwarded to the `go-demo` service endpoint, Docker networking did load balancing and resent it to one of the `go-demo` instances. As a result, all requests returned status *200 OK* proving that the combination of the proxy and the listener indeed works. All three instances of the proxy were reconfigured.

## Proxy Statistics

It is useful to see the statistics from the proxy. Among other things, they can reveal information that we could use to make decisions whether to scale our services.

As a security precaution, stats are disabled by default. We need to provide authentication as a way to enable statistics.

There are two ways to secure the proxy statistics. One is through environment variables `STATS_USER` and `STATS_PASS`. Typically, you would set those environment variables when creating the services. Since, in this case, the proxy is already running, we'll update it.

```bash
docker service update \
    --env-add STATS_USER=my-user \
    --env-add STATS_PASS=my-pass \
    proxy
```

!!! info
	If you are a Windows user, the `open` command might not be available. In that case, please execute `docker-machine ip node-1` to find out the IP of one of the VMs and open the admin page manually in your favorite browser.

```bash
open "http://$(docker-machine ip node-1)/admin?stats"
```

You will be asked for a username and password. Please use the default values *my-user/my-pass*. You will be presented with a screen with quite a few stats. Since we are running only one service (`go-demo`), those stats might not be of great interest. However, when many services are exposed through the proxy, HAProxy statistics provide indispensable information we should leverage when operating the cluster.

!!! info
	If you are running multiple replicas of the proxy, the statistics page will show information from one of the replicas only (randomly chosen by the ingress network). It is recommended to store stats from all the replicas in one of the monitoring systems like [Prometheus](https://prometheus.io/).

Even now, anyone could retrieve our username and password with a simple `service inspect` command. Fortunately, *Docker Flow Proxy* supports Docker secrets introduced in version 1.13.

Secrets can be used as a replacement for any of the environment variables. They should be prefixed with `dfp_` and written in lower case. As an example, `STATS_USER` environment variable would be specified as a secret `dfp_stats_user`.

Let's convert our credentials into Docker secrets.

```bash
echo "secret-user" \
    | docker secret create dfp_stats_user -

echo "secret-pass" \
    | docker secret create dfp_stats_pass -
```

Now we can attach those secrets to the `proxy` service.

```bash
docker service update \
    --secret-add dfp_stats_user \
    --secret-add dfp_stats_pass \
    proxy
```

From now on, the username and password are `secret-user` and `secret-pass`. Unlike environment variable, they are hidden from everyone and are visible only from inside the containers that form the service.

## Rewriting Paths

In some cases, you might want to rewrite the path of the incoming request before forwarding it to the destination service. We can do that using request parameters `reqPathSearch` and `reqPathReplace`.

As an example, we'll create the `go-demo` service that will be configured in the proxy to accept requests with the path starting with `/something`. Since the `go-demo` service allows only requests that start with `/demo`, we'll use `reqPathSearch` and `reqPathReplace` to rewrite the path.

The command is as follows.

```bash
docker service create --name go-demo \
    -e DB=go-demo-db \
    --network go-demo \
    --network proxy \
    --label com.df.notify=true \
    --label com.df.distribute=true \
    --label com.df.servicePath=/something \
    --label com.df.port=8080 \
    --label com.df.reqPathSearch='/something/' \
    --label com.df.reqPathReplace='/demo/' \
    --replicas 3 \
    vfarcic/go-demo
```

Please notice that, this time, the `servicePath` is `/something`. The `reqPathSearch` specifies the regular expression that will be used to search for part of the address and the `reqPathReplace` will replace it. In this case, `/something/` will be replaced with `/demo/`. The proxy uses the *regsub* function within the *http-request set-path* directive to apply a regex-based substitution which operates as the well-known *sed* utility with `"s/<regex>/<subst>/"`. For more information, please consult [Configuration: 4.2 http-request](https://cbonte.github.io/haproxy-dconv/1.8/configuration.html#4.2-http-request) and [Configuration: 7.3.1 regsub](https://cbonte.github.io/haproxy-dconv/1.8/configuration.html#7.3.1-regsub).

Please wait a few moments until the `go-demo` service is running. You can see the status of the service by executing `docker service ps go-demo`.

Once the `go-demo` service is up and running, we can confirm that the proxy was indeed configured correctly.

```bash
curl -i $(docker-machine ip node-1)/something/hello
```

The output is as follows.

```
HTTP/1.1 200 OK
Date: Sun, 11 Dec 2016 18:43:21 GMT
Content-Length: 14
Content-Type: text/plain; charset=utf-8

hello, world!
```

We sent a request to `/something/hello`. The proxy accepted the request, rewrote the path to `/demo/hello`, and forwarded it to the `go-demo` service.


Let's remove the `go-demo` service before we proceed with *authentication*.

```bash
docker service rm go-demo
```

## Basic Authentication

*Docker Flow: Proxy* can provide basic authentication that can be applied on two levels. We can configure the proxy to protect all or only a selected service.

### Global Authentication

We'll start by recreating the `go-demo` service we removed earlier.

```bash
docker service create --name go-demo \
  -e DB=go-demo-db \
  --network go-demo \
  --network proxy \
  --label com.df.notify=true \
  --label com.df.distribute=true \
  --label com.df.servicePath=/demo \
  --label com.df.port=8080 \
  --replicas 3 \
  vfarcic/go-demo
```

To configure the proxy to protect all the services, we need to specify the environment variable `USERS`.

As an example, we'll update the `proxy` service by adding the environment variable `USERS`.

```bash
docker service update --env-add "USERS=my-user:my-pass" proxy
```

Please wait a few moments until all the instances of the `proxy` are updated. You can monitor the status with the `docker service ps proxy` command.

Let's see what will happen if we send another request to the `go-demo` service.

```bash
curl -i $(docker-machine ip node-1)/demo/hello
```

The output is as follows.

```
HTTP/1.0 401 Unauthorized
Cache-Control: no-cache
Connection: close
Content-Type: text/html
WWW-Authenticate: Basic realm="defaultRealm"

<html><body><h1>401 Unauthorized</h1>
You need a valid user and password to access this content.
</body></html>
```

The `proxy` responded with the status `401 Unauthorized`. Our services are not accessible without the username and password.

Let's send one more request but, this time, with the username and password.

```bash
curl -i -u my-user:my-pass \
    $(docker-machine ip node-1)/demo/hello
```

The response is as follows.

```
HTTP/1.1 200 OK
Date: Sun, 04 Dec 2016 23:37:18 GMT
Content-Length: 14
Content-Type: text/plain; charset=utf-8

hello, world!
```

Since the request contained the correct username and password, proxy let it through and forwarded it to the `go-demo` service.

Multiple usernames and passwords can be separated with a comma.

```bash
docker service update \
    --env-add "USERS=my-user-1:my-pass-1,my-user-2:my-pass-2" \
    proxy
```

Once the update is finished, we will be able to access the services using the user `my-user-1` or `my-user-2`.

```bash
curl -i -u my-user-2:my-pass-2 \
    $(docker-machine ip node-1)/demo/hello
```

As expected, the proxy responded with the status `200` allowing us to access the service.

Let us remove the global authentication before we proceed further.

```bash
docker service update \
    --env-rm "USERS" \
    proxy
```

### Service Authentication

In many cases, we do not want to protect all services but only a selected few. A service can be protected by adding `users` parameter to the `reconfigure` request. Since we are using the [Docker Flow: Swarm Listener](https://github.com/vfarcic/docker-flow-swarm-listener) service to reconfigure the proxy, we'll add the parameter as one more label.

Let's start by removing the `go-demo` service.

```bash
docker service rm go-demo
```

Please wait a few moments until the service is removed and the `swarm-listener` updates the proxy.

Now we can create the `go-demo` service with the label `com.df.users`.

```bash
docker service create --name go-demo \
    -e DB=go-demo-db \
    --network go-demo \
    --network proxy \
    --label com.df.notify=true \
    --label com.df.distribute=true \
    --label com.df.servicePath=/demo \
    --label com.df.port=8080 \
    --label com.df.users=admin:password \
    vfarcic/go-demo
```

We added the `com.df.users` label with the value `admin:password`. Just as with the global authentication, multiple username/password combinations can be separated with comma (`,`).

After a few moments, the `go-demo` service will be created, and `swarm-lister` will update the proxy.

From now on, the `go-demo` service is accessible only if the username and the password are provided.

```bash
curl -i $(docker-machine ip node-1)/demo/hello

curl -i -u admin:password \
    $(docker-machine ip node-1)/demo/hello
```

The first request should return the status code `401 Unauthorized` while the second went through to the `go-demo` service.

Please note that both *global* and *service* authentication can be combined. In that case, all services would be protected with the users specified through the `proxy` environment variable `USERS` and individual services could overwrite that through the `reconfigure` parameter `users`.

Please note that passwords should not be provided in clear text. The above commands were only an example. You should consider encrypting passwords. They will be persisted in HAProxy configuration and they will be visible while inspecting service details in Docker. To encrypt them you should use `mkpasswd` utility and set parameter 'com.df.usersPassEncrypted=true' for passwords provided in `com.df.users` label or environment variable `USERS_PASS_ENCRYPTED` when using `USERS` variable. 

To demonstrated how encrypted passwords work we'll start by hashing a password.

```bash
mkpasswd -m sha-512 password
```

The output should be similar to the one that follows.

```
$6$F2eJJA.G$BfoxX38MoNS10tywEzQZVDZOAjJn9wyTZJecYg.CymjwE8Rgm7xJn0KG3faT36GZbOtrsu4ba.vhsnHrPCNAa0
```

Please note that `$` signs needs to be escaped. In `mkpasswd` output there will be always three `$` characters.

Let's update out go demo service:

```bash
docker service update \
    --label-add com.df.usersPassEncrypted=true \
    --label-add com.df.users=admin:\$6\$F2eJJA.G\$BfoxX38MoNS10tywEzQZVDZOAjJn9wyTZJecYg.CymjwE8Rgm7xJn0KG3faT36GZbOtrsu4ba.vhsnHrPCNAa0 \
    go-demo
```

You can verify that the authentication is required by executing the command that follows.

```bash
curl -i $(docker-machine ip node-1)/demo/hello
```

The output should indicate a `HTTP/1.0 401 Unauthorized` failure.

Let's repeat the request but, this time, with the proper password.

```bash
curl -i -u admin:password \
    $(docker-machine ip node-1)/demo/hello
```

Since Docker release 1.13, the preferable way to store confidential information is through Docker secrets. *Docker Flow Proxy* supports passwords stored as secrets through the `com.df.usersSecret` label. It should contain a name of a secret mounted in *Docker Flow Proxy*. The name of the secret should be prefixed with `dfp_users_`. For example if `com.df.usersSecret` is set to `monitoring`, proxy expects the secret name to be dfp_users_monitoring.

To show how it works, lets create a secret with the username `observer` and the hashed password. The commands are as follows.

```bash
echo "observer:\$6\$F2eJJA.G\$BfoxX38MoNS10tywEzQZVDZOAjJn9wyTZJecYg.CymjwE8Rgm7xJn0KG3faT36GZbOtrsu4ba.vhsnHrPCNAa0" \
    | docker secret create dfp_users_monitoring -
    
docker service update \
    --secret-add dfp_users_monitoring \
    proxy    
```

The first command stored the username and the hashed password as the secret `dfp_users_monitoring`. Username and password were separated with the colon (`:`).

The second command updated the proxy by adding the secret to it.

Now we need to change configuration of our test service so that the proxy can get the information about the name of the secret that contains the username and the hashed password.

```bash
docker service update \
    --label-rm com.df.users \
    --label-add com.df.usersSecret=monitoring \
    go-demo
```

We should verify that our service is reachable and protected with the user `observer`.

```bash
curl -i -u observer:password \
    $(docker-machine ip node-1)/demo/hello
```

As expected, the status code of the response is `200`, indicating that the request was successfull.

Before we move into the next subject, please remove the service and create it again without authentication.

```bash
docker service rm go-demo
```

Please wait a few moments until the service is removed and the `swarm-listener` updates the proxy.

```bash
docker service create --name go-demo \
    -e DB=go-demo-db \
    --network go-demo \
    --network proxy \
    --label com.df.notify=true \
    --label com.df.distribute=true \
    --label com.df.servicePath=/demo \
    --label com.df.port=8080 \
    vfarcic/go-demo
```

## Using TCP Request Mode

All the examples we run by now were limited to the HTTP protocol. *Docker Flow: Proxy* allows us to use *TCP* request mode as well.

We'll start by publishing a new port in the `proxy` service.

```bash
docker service update \
    --publish-add 6379:6379 \
    proxy
```

Please wait a few moments until Swarm updates all the proxy instances. You can monitor the progress by executing `docker service ps proxy`.

Let us create a service that will allow us to test whether `tcp` protocol works. We'll use *Redis* for this purpose.

```bash
docker service create --name redis \
    --network proxy \
    --label com.df.notify=true \
    --label com.df.distribute=true \
    --label com.df.port=6379 \
    --label com.df.srcPort=6379 \
    --label com.df.reqMode=tcp \
    redis:3.2
```

In addition to the labels we used before, we added `reqMode` with value `tcp`. The `swarm-listener` service will send a `reconfigure` request to the `proxy` which will add a new service `redis`. The new `proxy` configuration will listen to the port `6379` (`srcPort`) and forward requests to `redis` listening the same port `6379` (`port`).

Let's test whether the setup works.

```bash
docker-machine ssh node-1

telnet localhost 6379

PING
```

We entered one of the machines, started a telnet session.

The output is as follows.

```
+PONG
```

Redis responded with `PONG` proving that the `proxy` established `tcp` connection.

After a period of inactivity, `redis` will close the connection, and we can exit the machine.

```bash
exit
```

## Configuring Service SSLs

Please consult examples from [Configuring SSL Certificates](/certs).

Before you start using `Docker Flow Proxy`, you might want to get a better understanding of the flow of a request.

## The Flow Explained

We'll go over the flow of a request to one of the services in the Swarm cluster.

A user or a service sends a request to our DNS (e.g. *acme.com*). The request is usually HTTP on the port `80` or HTTPS on the port `443`.

DNS resolves the domain to one of the servers inside the cluster. We do not need to register all the nodes. A few is enough (more than one in the case of a failure).

The Docker's routing mesh inspects which containers are running on a given port and re-sends the request to one of the instances of the proxy. It uses round robin load balancing so that all instances share the load (more or less) equally.

The proxy inspects the request path (e.g. `/demo/hello`) and sends it the end-point with the same name as the destination service (e.g. `go-demo`). Please note that for this to work, both the proxy and the destination service need to belong to the same network (e.g. `proxy`). The proxy changes the port to the one of the destination service (e.g. `8080`).

The proxy network performs load balancing among all the instances of the destination service, and re-sends the request to one of them.

The whole process sounds complicated (it actually is from the engineering point of view). But, as a user, all this is transparent.

One of the important things to note is that, with a system like this, everything can be fully dynamic. Before the new Swarm introduced in Docker 1.12, we would need to run our proxy instances on predefined nodes and make sure that they are registered as DNS records. With the new routing mesh, it does not matter whether the proxy runs on a node registered in DNS. It's enough to hit any of the servers, and the routing mesh will make sure that it reaches one of the proxy instances.

A similar logic is used for the destination services. The proxy does not need to do load balancing. Docker networking does that for us. The only thing it needs is the name of the service and that both belong to the same network. As a result, there is no need to reconfigure the proxy every time a new release is made or when a service is scaled.

## Cleanup

Please remove Docker Machine VMs we created. You might need those resources for some other tasks.

```bash
docker-machine rm -f node-1 node-2 node-3
```
