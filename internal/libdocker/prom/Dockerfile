FROM prom/prometheus

ADD ./prometheus.yml /etc/prometheus/prometheus.yml

RUN mkdir -p /etc/prometheus/hive-metrics

ENTRYPOINT prometheus --config.file=/etc/prometheus/prometheus.yml
