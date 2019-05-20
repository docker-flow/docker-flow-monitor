#!/usr/bin/env bash
# Exit When Any Command Fails
set -e -u
# set debugging
trap 'echo -e "\v>> ERROR: Be sure that none of the steps fail while starting the cluster.. "' EXIT
set -x

docker -v
docker-machine -v
vboxmanage --version
echo

####------ USER
# sudo usermod -aG docker $USER

if [[ "$(uname -s )" == "Linux" ]]; then
  export VIRTUALBOX_SHARE_FOLDER="$PWD:$PWD"
fi

if [ $(systemctl is-active virtualbox) != 'active' ]; then 
  service virtualbox start
fi
###########################################################################
###         MACHINES              #########################################
# creates machines with virtualbox driver
MACHINES_CREATED_NUM=`docker-machine ls --quiet --filter driver=virtualbox | wc -l`
MACHINES_READY_NUM=`docker-machine ls --quiet --filter driver=virtualbox --filter state=Running | wc -l`
MACHINES_STOPPED=`docker-machine ls --quiet --filter driver=virtualbox --filter state=Stopped --filter state=Saved`

echo
echo "Checking for Docker virtualBox machines.. "

if [[ $MACHINES_CREATED_NUM == 0 ]]; then
  for i in 1 2; do
    docker-machine create -d virtualbox --virtualbox-cpu-count 2 swarm-$i
  done

# if they have been already created
elif [[ $MACHINES_READY_NUM == 0 ]]; then
  # machines were correctly stopped
  for machine in $(seq 1 $MACHINES_STOPPED); do
    docker-machine start $machine
  done

  export MANAGER_IP=$(docker-machine ip swarm-1):2377
  docker-machine env swarm-1
  eval $(docker-machine env swarm-1)
  echo "The machines are already Running.."

else
  # if machines are in timeout
  for i in $(seq 1 $MACHINES_CREATED_NUM); do
    docker-machine stop  swarm-$i
    docker-machine start swarm-$i
    docker-machine regenerate-certs swarm-$i
  done
  echo "Wait for machines to start please.."

fi

sleep 5
docker-machine ls

# Connect shell to the manager machine
eval $(docker-machine env swarm-1)

# Promote first machine as manager
docker swarm init --advertise-addr $(docker-machine ip swarm-1)

# Add workers to the swarm 
TOKEN=$(docker swarm join-token -q manager)
for i in 2; do
    eval $(docker-machine env swarm-$i)

    docker swarm join \
        --token $TOKEN \
        --advertise-addr $(docker-machine ip swarm-$i) \
        $(docker-machine ip swarm-1):2377
done

export MANAGER_IP=$(docker-machine ip swarm-1):2377
docker-machine env swarm-1


## ----- Exchange SSH keys --------
# user: docker
# psw:  tcuser
echo " Exchanging certificate keys with docker machines.."
echo
ssh-keygen -t rsa -b 2048
#   put the certificate on the virtual machine
for i in 1 2; do
  ssh-copy-id docker@${docker machine ip swarm-$i} 
done
## --------------------------------

## -----Env setup------------------
for i in 2; do
  eval $(docker-machine env swarm-$i)
  export CURRENT_TIME=$(date +%H:%M:%S)
  cat setup/setup-docker-machine.sh | ssh docker@$(docker-machine ip swarm-$i)
done
## ----------------------------------

echo ">> The swarm cluster is up and running"
