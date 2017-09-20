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
        sh "docker image build -t vfarcic/docker-flow-monitor ."
        sh "docker image build -t vfarcic/docker-flow-monitor-docs -f Dockerfile.docs ."
      }
    }
    stage("release") {
      when {
        branch "master"
      }
      steps {
        withCredentials([usernamePassword(
          credentialsId: "docker",
          usernameVariable: "USER",
          passwordVariable: "PASS"
        )]) {
          sh "docker login -u $USER -p $PASS"
        }
        sh "docker tag vfarcic/docker-flow-monitor vfarcic/docker-flow-monitor:${currentBuild.displayName}"
        sh "docker image push vfarcic/docker-flow-monitor:latest"
        sh "docker image push vfarcic/docker-flow-monitor:${currentBuild.displayName}"
        sh "docker tag vfarcic/docker-flow-monitor-docs vfarcic/docker-flow-monitor-docs:${currentBuild.displayName}"
        sh "docker image push vfarcic/docker-flow-monitor-docs:latest"
        sh "docker image push vfarcic/docker-flow-monitor-docs:${currentBuild.displayName}"
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
        sh "docker service update --image vfarcic/docker-flow-monitor:${currentBuild.displayName} monitor_monitor"
        sh "docker service update --image vfarcic/docker-flow-monitor-docs:${currentBuild.displayName} monitor_docs"
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
