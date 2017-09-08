import java.text.SimpleDateFormat

pipeline {
  agent {
    label "test"
  }
  options {
    buildDiscarder(logRotator(numToKeepStr: '2'))
    disableConcurrentBuilds()
  }
  stages {
    stage("build") {
      steps {
        script {
          def dateFormat = new SimpleDateFormat("yy.MM.dd")
          currentBuild.displayName = dateFormat.format(new Date()) + "-" + env.BUILD_NUMBER
        }
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
        sh "docker image push vfarcic/docker-flow-proxy:beta"
        sh "docker image build -t vfarcic/docker-flow-proxy-test -f Dockerfile.test ."
        sh "docker image push vfarcic/docker-flow-proxy-test"
        sh "docker image build -t vfarcic/docker-flow-proxy-docs -f Dockerfile.docs ."
        sh "docker tag vfarcic/docker-flow-proxy vfarcic/docker-flow-proxy:${currentBuild.displayName}"
        sh "docker tag vfarcic/docker-flow-proxy-docs vfarcic/docker-flow-proxy-docs:${currentBuild.displayName}"
      }
    }
    stage("test") {
      environment {
        DOCKER_HUB_USER = "vfarcic"
      }
      steps {
        script {
            hostIp = sh returnStdout: true, script: 'ifconfig eth0 | grep \'inet addr:\'  | cut -d: -f2 | awk \'{ print $1}\''
            hostIp = hostIp.trim()
            sh "HOST_IP=$hostIp docker-compose -f docker-compose-test.yml run --rm staging-swarm"
        }
      }
    }
    stage("release") {
      when {
        branch "master"
      }
      steps {
        sh "docker push vfarcic/docker-flow-proxy:latest"
        sh "docker push vfarcic/docker-flow-proxy:${currentBuild.displayName}"
        sh "docker push vfarcic/docker-flow-proxy-docs:latest"
        sh "docker push vfarcic/docker-flow-proxy-docs:${currentBuild.displayName}"
      }
    }
    stage("deploy") {
      when {
        branch "master"
      }
      agent {
        label "prod"
      }
      steps {
        sh "docker service update --image vfarcic/docker-flow-proxy:${currentBuild.displayName} proxy_proxy"
        sh "docker service update --image vfarcic/docker-flow-proxy-docs:${currentBuild.displayName} proxy_docs"
      }
    }
  }
  post {
    always {
      sh "docker system prune -f"
    }
    failure {
      slackSend(
        color: "danger",
        message: "${env.JOB_NAME} failed: ${env.RUN_DISPLAY_URL}"
      )
    }
  }
}
