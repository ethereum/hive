FROM grafana/grafana

ADD annotate_metrics.sh /hive/annotate_metrics.sh

# see https://grafana.com/tutorials/provision-dashboards-and-data-sources/
# for more information about grafana provisioning configuration

ADD grafana.ini /etc/grafana/config.ini

ADD dashboards_provider.yaml /etc/grafana/provisioning/dashboards/dashboards_provider.yaml
ADD dashboards /hive/grafana-dashboards



