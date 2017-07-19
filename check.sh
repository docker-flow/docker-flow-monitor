wget -qO- "localhost:8080/v1/docker-flow-monitor/ping"

if [ "$?" -ne "0" ]; then exit; fi

ps aux | grep prometheu[s]
