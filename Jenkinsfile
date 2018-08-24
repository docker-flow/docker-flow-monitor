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
        sh "docker image build -t dockerflow/docker-flow-monitor ."
        sh "docker image build -t dockerflow/docker-flow-monitor-docs -f Dockerfile.docs ."
      }
    }
    stage("release") {
      when {
        branch "master"
      }
      steps {
        dfLogin()
        sh "docker tag dockerflow/docker-flow-monitor dockerflow/docker-flow-monitor:2-${currentBuild.displayName}"
        sh "docker image push dockerflow/docker-flow-monitor:latest"
        sh "docker image push dockerflow/docker-flow-monitor:2-${currentBuild.displayName}"
        sh "docker tag dockerflow/docker-flow-monitor-docs dockerflow/docker-flow-monitor-docs:2-${currentBuild.displayName}"
        sh "docker image push dockerflow/docker-flow-monitor-docs:latest"
        sh "docker image push dockerflow/docker-flow-monitor-docs:2-${currentBuild.displayName}"
        dockerLogout()
        dfReleaseGithub2("docker-flow-monitor")
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
        sh "helm upgrade -i docker-flow-monitor helm/docker-flow-monitor --namespace df --set image.tag=2-${currentBuild.displayName}"
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
