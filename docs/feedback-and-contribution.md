# Feedback and Contribution

The *Docker Flow Proxy* project welcomes, and depends, on contributions from developers and users in the open source community. Contributions can be made in a number of ways, a few examples are:

* Code patches or new features via pull requests
* Documentation improvements
* Bug reports and patch reviews

## Reporting an Issue

Feel fee to [create a new issue](https://github.com/vfarcic/docker-flow-proxy/issues). Include as much detail as you can.

If an issue is a bug, please provide steps to reproduce it.

If an issue is a request for a new feature, please specify the use-case behind it.

## Discussion

Please join the [DevOps20](http://slack.devops20toolkit.com/) Slack channel if you'd like to discuss the project or have a problem you'd like us to solve.

## Contributing To The Project

I encourage you to contribute to the *Docker Flow Proxy* project.

The project is developed using *Test Driven Development* and *Continuous Deployment* process. Test are divided into unit and integration tests. Every code file has an equivalent with tests (e.g. `reconfigure.go` and `reconfigure_test.go`). Ideally, I expect you to write a test that defines that should be developed, run all the unit tests and confirm that the test fails, write just enough code to make the test pass, repeat. If you are new to testing, feel free to create a pull request indicating that tests are missing and I'll help you out.

Once you are finish implementing a new feature or fixing a bug, run the *Complete Cycle*. You'll find the instructions below.

### Repository

Fork [docker-flow-proxy](https://github.com/vfarcic/docker-flow-proxy).

### Unit Testing

```bash
go get -d -v -t

go test ./... -cover -run UnitTest
```

### Building

```bash
export DOCKER_HUB_USER=[...] # Change to your user in hub.docker.com

docker-compose -f docker-compose-test.yml run --rm unit

docker image build -t $DOCKER_HUB_USER/docker-flow-proxy .
```

### The Complete Cycle (Unit, Build, Staging)

#### Manually

* On your laptop

```bash
export DOCKER_HUB_USER=[...] # Change to your user in hub.docker.com

docker image build -t $DOCKER_HUB_USER/docker-flow-proxy:beta .

docker image push $DOCKER_HUB_USER/docker-flow-proxy:beta

docker image build -t $DOCKER_HUB_USER/docker-flow-proxy-test -f Dockerfile.test .

docker image push $DOCKER_HUB_USER/docker-flow-proxy-test
```

* Inside a Swarm cluster

```bash
export DOCKER_HUB_USER=[...] # Change to your user in hub.docker.com

export HOST_IP=[...] # Change to a domain or an IP of one of Swarm nodes

docker-compose -f docker-compose-test.yml run --rm staging-swarm
```

#### Through Jenkins

Make a PR and let Jenkins do the work. You can monitor the status from the Jenkins job [Viktor Farcic / docker-flow-proxy](http://jenkins.dockerflow.com/blue/organizations/jenkins/vfarcic%2Fdocker-flow-proxy/activity).

Please [create an issue](https://github.com/vfarcic/docker-flow-proxy/issues) if you'd like to add your repository to the builds.


