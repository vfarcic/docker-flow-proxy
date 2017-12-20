# Using SNI routing with Docker Flow Proxy

SNI (Server Name Indication) is an extension to TLS that allows a client to specify which hostname it is attempting to connect to at the start of the TLS handshaking process. The original intent of SNI was to allow a single server to present multiple certificates on the same port and thereby allow multiple websites or services to be served on the same port. The server uses the SNI hostname to determine which certificate to present to the client.

Because the SNI field is available at the beginning of the TLS negotiation phase in plain-text, an entity like HA-Proxy, can peek into the TCP stream and use that information to route to the appropriate backend server without needing to terminate the TLS stream. It can then treat it as any other TCP stream. In this case, the back-end server will establish the TLS connection with the client and HA-Proxy won't be able to sniff the encrypted traffic passing by.

This is not the only mode of operation. HA-Proxy can also be configured to present the appropriate certificate for incoming connections, do the TLS handshaking with the client and then establish a new socket connection with the backend as with normal HTTP routing. In this case, you can use the SNI field to still do routing especially with non-HTTP TLS traffic. 

If you are wondering why we care about what HA-Proxy does, that's because *Docker Flow Proxy* uses HA-Proxy to actually route traffic to backend docker services, so knowing what it's capable of will allow you to debug it if things go wrong. See the [HA-Proxy configuration](http://cbonte.github.io/haproxy-dconv/1.7/configuration.html) manual for complete configuration information.

Before we jump into configuring *Docker Flow Proxy* to route requests to services using the SNI field, let's look at what we need.

## Requirements

This tutorial assumes you already have a docker swarm cluster running _Docker Flow Proxy_. If you don't, please visit the [Running _Docker Flow Proxy_ In Swarm Mode With Automatic Reconfiguration](swarm-mode-auto) page for a tutorial. Once you have _Docker Flow Proxy_ running, you can proceed with the remaining tutorial. We shall only cover the bits where we launch services into swarm and have _Docker Flow Proxy_ route to the service using the SNI headers. Here's what we'll need before we start:

1. Service names (FQDNs) to use.
2. Name resolution
3. TLS enabled services with certificates matching the DNS names

### Service Names
For the sake of this example, let's create 2 services called `api.foobar.net` and another one called `test.foobar.net`. Of course the service endpoints don't need to belong to a single domain. But having trusted certificates for the FQDNs you plan to connect to is important.

### Name Resolution
Since we don't own the `foobar.net` domain, we can't configure authoritative DNS. Instead we can either modify the _hosts_ file on the client machine or configure something like _dnsmasq_ to provide local resolution to allow us to access services on the local network via the service endpoint FQDNs. To modify the hosts file, see [this link](https://support.rackspace.com/how-to/modify-your-hosts-file). Assuming one of your swarm machines is running on 192.168.1.5, add the following line to the hosts file:
```
192.168.1.5	api.foobar.net test.foobar.net
```
To do the same using dnsmasq instead, try [this tutorial](https://blog.heckel.xyz/2013/07/18/how-to-dns-spoofing-with-a-simple-dns-server-using-dnsmasq/).

### TLS Certificates

The last pre-requisite for TLS enabled services is a TLS certificate/key pair. If you are doing this for anything apart from testing, you probably need one or more TLS certificates for the FQDNs matching your service endpoints - from a *certifying authority* like Verisign or Comodo or even Leytsencrypt. In many cases, a single wildcard certificate should suffice for all services that match the wildcard. 

If you are testing on local test machines only, you could setup a local certifying authority to issue test certificates:

```Shell
$ mkdir -p certs
$ docker run -v `pwd`/certs:/certs  -e SSL_SUBJECT=api.foobar.net -e SSL_PREFIX=api -it  faisyl/omgwtfssl
$ docker run -v `pwd`/certs:/certs  -e SSL_SUBJECT=test.foobar.net -e SSL_PREFIX=test -it  faisyl/omgwtfssl
```

Verify that the certificates are created:
```Shell
$ ls certs
api-key.pem  api.csr      api.pem      ca-key.pem   ca.pem       ca.srl       openssl.cnf  test-key.pem test.csr     test.pem
```
We now have 3 sets of certificates. The _CA_ certificate pair (ca.pem, ca-key.pem), the _api_ certificate pair (api.pem, api-key.pem) and the _test_ certificate pair (test.pem, test-key.pem).

While the self signed certificates are sufficient to run this tutorial, if you don't want to see **certificate untrusted** errors, you can simply add the ca.pem as a trusted signing certificate. Depending on your operating system and browser, you might need to add the ca.pem to either the system certificate store or the browser certificate store. See [this link](https://www.bounca.org/tutorials/install_root_certificate.html) on how to do that on different operating systems.


## Running the services

Create the docker stack yaml file. Save this to a file called example.yaml:
```yaml
version: "3.1"
services:

  api:
    image: faisyl/pydemo
    deploy:
      replicas: 1
      labels:
        com.df.notify: 'true'
        com.df.pathType: "req_ssl_sni -i -m reg"
        com.df.servicePath: "^(api\\.)"
        com.df.srcPort: 443
        com.df.reqMode: sni
        com.df.port: 443
    networks:
      - proxy
    environment:
      - SVCNAME=api
    secrets:
      - source: api.crt
        target: server.crt
        mode: 0440
      - source: api.key
        target: server.key
        mode: 0440

  test:
    image: faisyl/pydemo
    deploy:
      replicas: 1
      labels:
        com.df.notify: 'true'
        com.df.pathType: "req_ssl_sni -i -m beg"
        com.df.servicePath: "test."
        com.df.srcPort: 443
        com.df.reqMode: sni
        com.df.port: 443
    networks:
      - proxy
    environment:
      - SVCNAME=test
    secrets:
      - source: test.crt
        target: server.crt
        mode: 0440
      - source: test.key
        target: server.key
        mode: 0440


secrets:
  api.crt:
    file: ./certs/api.pem
  api.key:
    file: certs/api-key.pem
  test.crt:
    file: ./certs/test.pem
  test.key:
    file: certs/test-key.pem

networks:
  proxy:
    external: true
```

To bring up the example stack, run:

```Shell
$ docker stack deploy -c example.yaml example
```

The yaml file defines 2 services called `api` and `test`. The interesting bits to pay attention to are the labels `com.df.reqMode`, `com.df.pathType`, and `com.df.servicePath`. The `com.df.reqMode: sni` implies SNI routing mode. The `com.df.pathType: "req_ssl_sni -i -m beg"` label asks haproxy to do a match with the _beg_inning of the SNI field in the TLS stream. The `com.df.servicePath: "test."` label defines the actual string to check. Roughly translated, this configuration implies: "Use SNI routing. Check for SNI field. If the beginning of the SNI field is 'test.' then route to this service on port 443." 

Similarly, the `api` service uses a regex match instead of a string match at the beginning. 

To verify that the example stack was deployed correctly, run this:
```Shell
$ docker stack ls
NAME                SERVICES
dfp                 2
example             2

$ docker service ls
ID                  NAME                 MODE                REPLICAS            IMAGE                                       PORTS
207xo1qn275w        example_test         replicated          1/1                 faisyl/pydemo:latest
7w2ohiepgcv6        dfp_proxy            global              1/1                 faisyl/docker-flow-proxy:latest             *:443->443/tcp
pcoopm5b91ba        dfp_swarm-listener   replicated          1/1                 vfarcic/docker-flow-swarm-listener:latest
sl75set9jao4        example_api          replicated          1/1                 faisyl/pydemo:latest
```
Here, I have 2 stacks deployed. `dfp` is my docker-flow-proxy stack. `example` is the stack I just deployed. The example stack defines 2 services: `example_test` and `example_api`.

## Testing the services
This is straightforward. Open a browser window to https://test.foobar.net/sleep/3. If you have configured the OS/browser to trust your CA certificate, then you should see a page like this:

```text
[test]Mon Oct 2 17:35:40 2017: 3

[test]Mon Oct 2 17:35:41 2017: 2

[test]Mon Oct 2 17:35:42 2017: 1
```

Similarly if you hit https://api.foobar.net/sleep/5, this is what you see.
```text
[api]Mon Oct 2 17:36:42 2017: 5

[api]Mon Oct 2 17:36:43 2017: 4

[api]Mon Oct 2 17:36:44 2017: 3

[api]Mon Oct 2 17:36:45 2017: 2

[api]Mon Oct 2 17:36:46 2017: 1
```

In either case, HA-Proxy is now using the SNI field added to the TLS handshake by the browser to figure out what service to connect to.

## Final steps
Services can expose more than one backend port and that's supported by the SNI routing code as well. To support front-end ports other than 443, just make sure you modify the _Docker Flow Proxy_ yaml to add the extra ports there. To configure the service, set the srcPort label accordingly. To support more than one front-end/back-end port on the service, use the _Docker Flow Proxy_ indexed configuration [feature](usage/#general-query-parameters).

To clean up, destroy the service using:
```Shell
$ docker stack rm example
```

You might also consider deleting trusted CA certificates added by this tutorial, and removing the **hosts** file entries we added - api.foobar.net and test.foobar.net.

