pipeline {
  agent {
    label "docker"
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
      }
    }
  }
}

// TODO: Notification to slack
// TODO: GitHub WebHook