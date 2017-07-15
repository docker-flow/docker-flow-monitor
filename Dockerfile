FROM golang:1.6 AS build
ADD . /src
WORKDIR /src
RUN go get -t github.com/stretchr/testify/suite
RUN go get -d -v -t
RUN go test --cover ./... --run UnitTest
RUN go build -v -o docker-flow-monitor



FROM prom/prometheus:v1.5.2

ENV GLOBAL_SCRAPE_INTERVAL=10s \
    ARG_CONFIG_FILE=/etc/prometheus/prometheus.yml \
    ARG_STORAGE_LOCAL_PATH=/prometheus \
    ARG_WEB_CONSOLE_LIBRARIES=/usr/share/prometheus/console_libraries \
    ARG_WEB_CONSOLE_TEMPLATES=/usr/share/prometheus/consoles

ENTRYPOINT ["docker-flow-monitor"]

COPY --from=build /src/docker-flow-monitor /bin/docker-flow-monitor
RUN chmod +x /bin/docker-flow-monitor

