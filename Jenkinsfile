pipeline {
  agent {
    label "test"
  }
  options {
    buildDiscarder(logRotator(numToKeepStr: '2'))
  }
  stages {
    stage("build") {
      steps {
        checkout scm
        sh "docker image build -t vfarcic/docker-flow-proxy ."
        sh "docker tag vfarcic/docker-flow-proxy vfarcic/docker-flow-proxy:beta"
        withCredentials([usernamePassword(
          credentialsId: "docker",
          usernameVariable: "USER",
          passwordVariable: "PASS"
        )]) {
          sh "docker login -u $USER -p $PASS"
        }
        sh "docker push vfarcic/docker-flow-proxy:beta"
        sh "docker image build -t vfarcic/docker-flow-proxy-test -f Dockerfile.test ."
        sh "docker push vfarcic/docker-flow-proxy-test"
      }
    }
    stage("test") {
      environment {
        HOST_IP = "test.dockerflow.com"
        DOCKER_HUB_USER = "vfarcic"
      }
      steps {
        sh "docker-compose -f docker-compose-test.yml run --rm staging-swarm"
      }
    }
    stage("release") {
      when {
        branch "master"
      }
      sh "docker tag vfarcic/docker-flow-proxy vfarcic/docker-flow-proxy:2.${env.BUILD_NUMBER}"
      // sh "docker push vfarcic/docker-flow-proxy:2.${env.BUILD_NUMBER}"
      // sh "docker push vfarcic/docker-flow-proxy"
      sh "docker-compose -f docker-compose-test.yml run --rm docs"
      sh "docker build -t vfarcic/docker-flow-proxy-docs -f Dockerfile.docs ."
      sh "docker tag vfarcic/docker-flow-proxy-docs vfarcic/docker-flow-proxy-docs:2.${env.BUILD_NUMBER}"
      sh "docker push vfarcic/docker-flow-proxy-docs:2.${env.BUILD_NUMBER}"
      sh "docker push vfarcic/docker-flow-proxy-docs"
    }
  }
}

// TODO: Notification to slack
// TODO: GitHub WebHook
// TODO: Run `docker system prune -f` periodically