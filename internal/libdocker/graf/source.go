package graf

import "embed"

//go:embed Dockerfile annotate_metrics.sh dashboards_provider.yaml grafana.ini dashboards/*
var GrafanaSource embed.FS
