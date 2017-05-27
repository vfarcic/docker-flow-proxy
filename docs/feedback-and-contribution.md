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

#### Setup

```bash
# Change to the IP of your host
export HOST_IP=[...]
```

#### Unit Tests

```bash
docker-compose \
    -f docker-compose-test.yml \
    run --rm unit
```

#### Staging (Integration) Tests

```bash
# Change to your user in hub.docker.com
export DOCKER_HUB_USER=[...]

docker image build \
    -t $DOCKER_HUB_USER/docker-flow-proxy:beta \
    .

docker image push \
    $DOCKER_HUB_USER/docker-flow-proxy:beta

docker-compose \
    -f docker-compose-test.yml \
    run --rm staging-swarm
```

##### Locally simulating CI

All above can be executed in same manner as CI is running it before a build using the command that follows.

```bash
./scripts/local-ci.sh
```

The script requires:

* DOCKER_HUB_USER environment variable to be set
* HOST_IP to be set
* docker logged in to docker hub with $DOCKER_HUB_USER user

### Pull Request

Once the feature is done, create a pull request.
