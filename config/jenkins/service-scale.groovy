pipeline {
  agent {
    label "prod"
  }
  parameters {
    string(
      name: "service",
      defaultValue: "",
      description: "The name of the service that should be scaled"
    )
    string(
      name: "scale",
      defaultValue: "",
      description: "Number of replicas to add or remove."
    )
  }
  stages {
    stage("Scale") {
      steps {
        script {
          def inspectOut = sh(
            script: "docker service inspect $service",
            returnStdout: true
          )
          def inspectJson = readJSON text: inspectOut.trim()
          println(inspectJson)
          println("---------------------------------------------------------")
          def currentReplicas = inspectJson[0].Spec.Mode.Replicated.Replicas
          println(" Current replicas: "+ currentReplicas)
          
          def newReplicas = currentReplicas + scale.toInteger()
          println(" New replicas: "+ newReplicas)
          
          def minReplicas = inspectJson[0].Spec.TaskTemplate.ContainerSpec.Labels["com.df.scaleMin"].toInteger()
          println(" MIN replicas: "+ minReplicas)

          def maxReplicas = inspectJson[0].Spec.TaskTemplate.ContainerSpec.Labels["com.df.scaleMax"].toInteger()
          println(" MAX replicas: "+ maxReplicas)

          if (newReplicas > maxReplicas) {
            error "$service is already scaled to the maximum number of $maxReplicas replicas"
          } else if (newReplicas < minReplicas) {
            error "$service is already descaled to the minimum number of $minReplicas replicas"
          } else {
            sh "docker service scale $service=$newReplicas"
            echo "$service was scaled from $currentReplicas to $newReplicas replicas"
          }
        }
      }
    }
  }
  post {
    failure {
      slackSend(
        color: "danger",
        message: """$service could not be scaled.
Please check Jenkins logs for the job ${env.JOB_NAME} #${env.BUILD_NUMBER}
${env.BUILD_URL}console"""
      )
    }
  }
}