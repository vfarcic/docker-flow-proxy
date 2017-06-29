pipeline {
  agent {
    label "docker"
  }
  options {
    buildDiscarder(logRotator(numToKeepStr: '2')) }
  }
  stages {
    stage("build") {
      steps {
        checkout scm
        sh "docker image build -t vfarcic/docker-flow-proxy"
      }
    }
  }
}

// TODO: Notification to slack
// TODO: GitHub WebHook