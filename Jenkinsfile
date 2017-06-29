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
        docker image build -t vfarcic/docker-flow-proxy
      }
    }
  }
//  post {
//    failure {
//      slackSend(
//        color: "danger",
//        message: """$service could not be scaled.
//Please check Jenkins logs for the job ${env.JOB_NAME} #${env.BUILD_NUMBER}
//${env.RUN_DISPLAY_URL}"""
//      )
//    }
//  }
}