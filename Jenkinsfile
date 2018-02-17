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
        dfBuild("docker-flow-proxy")
        sh "docker image build -t vfarcic/docker-flow-proxy:latest-packet-beat -f Dockerfile.packetbeat ."
        sh "docker image tag vfarcic/docker-flow-proxy:latest-packet-beat vfarcic/docker-flow-proxy:${currentBuild.displayName}-packet-beat"
      }
    }
    stage("staging") {
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
        dockerLogin()
        sh "docker image push vfarcic/docker-flow-proxy:latest-packet-beat"
        sh "docker image push vfarcic/docker-flow-proxy:${currentBuild.displayName}-packet-beat"
        dockerLogout()
        dfRelease("docker-flow-proxy")
        dfReleaseGithub("docker-flow-proxy")
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
        dfDeploy("docker-flow-proxy", "proxy_proxy", "proxy_docs")
      }
    }
  }
  post {
    always {
      sh "docker system prune -f -a --volumes"
    }
    failure {
      slackSend(
        color: "danger",
        message: "${env.JOB_NAME} failed: ${env.RUN_DISPLAY_URL}"
      )
    }
  }
}
