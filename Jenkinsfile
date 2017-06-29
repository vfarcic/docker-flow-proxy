pipeline {
  agent {
    label "docker"
  }
  stages {
    stage("build") {
      steps {
        checkout scm
        sh "ls -l"
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