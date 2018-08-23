FROM golang:1.9.2 AS build
ADD . /src
WORKDIR /src
RUN go get -t github.com/stretchr/testify/suite
RUN go get -d -v -t
RUN go test --cover ./... --run UnitTest -p 1
RUN CGO_ENABLED=0 GOOS=linux go build -v -o docker-flow-monitor



FROM prom/prometheus:v2.3.2

ENV GLOBAL_SCRAPE_INTERVAL=10s \
    ARG_CONFIG_FILE=/etc/prometheus/prometheus.yml \
    ARG_STORAGE_TSDB_PATH=/prometheus \
    ARG_WEB_CONSOLE_LIBRARIES=/usr/share/prometheus/console_libraries \
    ARG_WEB_CONSOLE_TEMPLATES=/usr/share/prometheus/consoles \
    CONFIGS_DIR="/run/secrets"

EXPOSE 8080

ENTRYPOINT ["docker-flow-monitor"]

HEALTHCHECK --interval=5s CMD /bin/check.sh

COPY --from=build /src/docker-flow-monitor /bin/docker-flow-monitor
COPY check.sh /bin/check.sh
COPY conf/shortcuts.yaml /etc/dfm/shortcuts.yaml

USER root
RUN chmod +x /bin/check.sh
RUN chmod +x /bin/docker-flow-monitor
USER nobody

