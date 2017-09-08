# Configuring SSL Certificates

This article provides examples that can be used as a starting point when configuring SSL certificates.

!!! tip
    Docker Secrets are a preferable way of managing SSL certificates. If you think secrets are a good fit for your use case, feel free to skip other methods and just straight into [Adding Certificates As Docker Secrets](#adding-certificates-as-docker-secrets).

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

eval $(docker-machine env node-1)
```

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

Before proceeding towards the SSL examples we should wait until all the services are running.

```bash
docker service ls
```

## Creating a New Image With Certificates or Mounting a Volume

Even though we published the proxy port `443` and it is configured to forward traffic to our services, `SSL` communication still does not work. We can confirm that by sending an HTTPS request to our demo service.

```bash
curl -i "https://$(docker-machine ip node-1)/demo/hello"
```

The output is as follows.

```
curl: (35) Unknown SSL protocol error in connection to 192.168.99.100:-9847
```

The error means that there is no certificate that should be used with HTTPS traffic.

There are two ways we can add certificates to the proxy. One is to create your own Docker image based on `docker-flow-proxy`. `Dockerfile` could be as follows.

```
FROM vfarcic/docker-flow-proxy
COPY my-cert.pem /certs/my-cert.pem
```

When the image is built, it will be based on `vfarcic/docker-flow-proxy` and include `my-cert.pem` file. The `my-cert.pem` would be your certificate. Docker Flow proxy will load all certificates located in the `/certs` directory.

If your certificate is static (almost never changes) and you are willing to create your own `docker-flow-proxy` image, this might be a good option. An alternative would be to mount a network volume with certificates.

## Adding Certificates Through HTTP Requests

!!! info
    Certificates can be added to the proxy dynamically through an HTTP request.

We'll start by creating a certificate we'll use throughout the examples.

For production, you should create your certificate through one of the trusted services. For demo purposes, we'll create a self-signed certificate with `openssl`. Before proceeding, please make sure that `openssl` is installed on your host OS.

We'll store a certificate in `tmp` directory, so let us create it.

```bash
mkdir -p certs
```

The certificate will be tied to the `*.xip.io` domain, which is handy for demonstration purposes. It'll let us use the same certificate even if our server IP addresses might change while testing locally. With [xip.io](http://xip.io/) don't need to re-create the self-signed certificate when, for example, our Docker machine changes IP.

!!! info
	I use the [xip.io](http://xip.io/) service as it allows me to use a hostname rather than directly accessing the servers via an IP address. It saves me from editing me computers' host file.

To create a certificate, first we need a key.

```bash
openssl genrsa -out certs/xip.io.key 1024
```

With the newly created key, we can proceed and create a certificate signing request (CSR). The command is as follows.

```bash
openssl req -new \
    -key certs/xip.io.key \
    -out certs/xip.io.csr
```

You will be asked quite a few question. It is important that when "Common Name (e.g. server FQDN or YOUR name)" comes, you answer it with `*.xip.io`. Feel free to answer the rest of the question as you like.

Finally, with the key and the CSR, we can create the certificate. The command is as follows.

```bash
openssl x509 -req -days 365 \
    -in certs/xip.io.csr \
    -signkey certs/xip.io.key \
    -out certs/xip.io.crt
```

As a result, we have the `crt`, `csr`, and `key` files in the `tmp` directory.

Next, after the certificates are created, we need to create a `pem` file. A pem file is essentially just the certificate, the key and optionally certificate authorities concatenated into one file. In our example, we'll simply concatenate the certificate and key files together (in that order) to create a `xip.io.pem` file.

```bash
cat certs/xip.io.crt certs/xip.io.key \
    | tee certs/xip.io.pem
```

To demonstrate how `xip.io` works, we can, for example, send the request that follows.

```bash
curl -i "http://$(docker-machine ip node-1).xip.io/demo/hello"
```

Our laptop thinks that we are dealing with the domain `xip.io`. When your computer looks up the xip.io domain, the xip.io DNS server extracts the IP address from the domain and sends it back in the response. The [xip.io](http://xip.io/) service is useful for testing purposes.

Now we have the PEM file and a domain we'll use to test it. The only thing missing is to add it to the proxy.

We'll send the proxy service the PEM file we just created. Before we do that, we should publish the port `8080`. It is proxy's internal port we can use to send it commands. If you already have the port `8080` published, there is no need to run the command that follows.

```bash
docker service update \
    --publish-add 8080:8080 proxy_proxy
```

Please wait a few moments until the proxy is updated. You can check the status by executing the `docker service ps proxy` command.

Now we can tell the proxy to use the certificate we created. The command is as follows.

```bash
curl -i -XPUT \
    --data-binary @certs/xip.io.pem \
    "$(docker-machine ip node-1):8080/v1/docker-flow-proxy/cert?certName=xip.io.pem&distribute=true"
```

The `PUT` request to the proxy passed the certificate in its body (`--data-binary @tmp/xip.io.pem`). The request URL has the certification name (`certName`) as one of the parameters. The second parameter (`distribute`) sends the proxy the signal to distribute the certificate to all the replicas. The result of the request is that the certificate has been added to all the replicas of the proxy. We can confirm that by inspecting the proxy config.

```bash
curl "$(docker-machine ip node-1).xip.io:8080/v1/docker-flow-proxy/config"
```

The relevant part of the output is as follows.

```
frontend services
    bind *:80
    bind *:443 ssl crt-list /cfg/crt-list.txt
    mode http

    acl url_go-demo path_beg /demo
    use_backend go-demo-be if url_go-demo

backend go-demo-be
    mode http
    server go-demo go-demo:8080
```

The certificate `xip.io.pem` has been added as an entry in /cfg/crt-list.txt to the `*:443` binding. The proxy is ready to serve HTTPS requests.

Let's confirm that HTTPS works.

```bash
curl -i -k "https://$(docker-machine ip node-1).xip.io/demo/hello"
```

Please note that you are not limited to a single certificate. You can send multiple `PUT` requests with different certificates and they will all be added to the proxy.

Now you can secure your proxy communication with SSL certificates. Unless you already have a certificate, purchase it or get it for free from [Let's Encrypt](https://letsencrypt.org/). The only thing left is for you to send a request to the proxy to include the certificate and try it out with your domain.

Now that we have a way to add certificates to the proxy, we might explore a more secure way to accomplish the same result.

## Adding Certificates As Docker Secrets

Keeping certificates inside Docker images or mounted volumes is fairly insecure. To tighten the security, we can add a certificate as a Docker secret.

```bash
docker secret create cert-xip.io.pem certs/xip.io.pem
```

Once a secret is safely stored inside Swarm managers, the only thing missing is to add it to the proxy.

Normally, we would include certificates when creating the service. However, since the `proxy` service is already running, we'll update it instead.

```bash
docker service update --secret-add cert-xip.io.pem proxy_proxy
```

Docker stored the secret certificate inside all the containers that form the `proxy` service.

Let's confirm that the certificate indeed works.

```bash
curl -i -k "https://$(docker-machine ip node-1).xip.io/demo/hello"
```

We got the `200` response confirming that the certificate is stored in the proxy as a Docker secret and that the configuration was updated accordingly.

!!! info
    Since many other types of information can be stored as secrets, *Docker Flow Proxy* assumes that secrets that should be used as certificates are prefixed with `cert-` or `cert_`. Secrets with any other naming convention will not be loaded as certificates.

## Summary

We explored a few ways to store certificates inside the proxy. We can build a new image that already includes the certificates or we can mount a network volume. Certificates can be added after the service was created through the [PUT certificate](/usage/#put-certificate) request. Finally, we explored how we can leverage Docker secrets that provide a more secure way to transmit certificates to the proxy.


!!! info
    The recommended way to manage proxy certificates is through Docker secrets.
