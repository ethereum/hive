package prom

import "embed"

//go:embed Dockerfile prometheus.yml
var PrometheusSource embed.FS
