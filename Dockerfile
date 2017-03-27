FROM prom/prometheus:v1.5.2

ENTRYPOINT ["docker-flow-monitor"]

COPY docker-flow-monitor /bin/docker-flow-monitor
RUN chmod +x /bin/docker-flow-monitor

