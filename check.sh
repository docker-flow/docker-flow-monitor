wget -qO- "localhost:8080/v1/docker-flow-monitor/ping"

if [ "$?" -ne "0" ]; then exit; fi

wget -qO- "localhost:9090/api/v1/targets"
