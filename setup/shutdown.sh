#!/bin/bash

###########################################################################
###         MACHINES              #########################################
MACHINES_CRATED=`docker-machine ls --quiet --filter driver=virtualbox | wc -l`
MACHINES_READY=` docker-machine ls --quiet --filter driver=virtualbox --filter state=Running | wc -l`

echo
echo "Checking for Docker virtualBox machines.. "

if [[ $MACHINES_READY > 0 ]]; then
  for i in {1..$MACHINES_READY}; do
      docker-machine stop swarm-$i
  done
fi

###########################################################################
# check fot any residual local stack
docker-compose -f config/stack/docker-compose-swarm.yml down
docker stack rm -c config/stack/proxy.composer.yml proxy
docker stack rm -c config/stack/exporters.yml exporter
docker stack rm -c config/stack/jenkins-scale.yml jenkins
docker stack rm -c config/stack/docker-monitoring-complete.yml monitor