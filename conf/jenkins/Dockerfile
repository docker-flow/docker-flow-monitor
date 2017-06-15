FROM jenkins:alpine

# The names of the secrets containing Jenkins admin username and password
ENV JENKINS_USER_SECRET="jenkins-user" JENKINS_PASS_SECRET="jenkins-pass"

# Prefix of the URL path
ENV JENKINS_OPTS="--prefix=/jenkins"

# Whether to skip setup wizard
ENV JAVA_OPTS="-Djenkins.install.runSetupWizard=false"

# Creates username and password specified through environment variables JENKINS_USER_SECRET and JENKINS_PASS_SECRET
COPY security.groovy /usr/share/jenkins/ref/init.groovy.d/security.groovy

# Creates username and password specified through environment variables JENKINS_USER_SECRET and JENKINS_PASS_SECRET
COPY plugins.txt /usr/share/jenkins/ref/plugins.txt
RUN /usr/local/bin/install-plugins.sh < /usr/share/jenkins/ref/plugins.txt
