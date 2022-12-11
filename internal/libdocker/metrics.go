package libdocker

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	docker "github.com/fsouza/go-dockerclient"

	"github.com/ethereum/hive/internal/libdocker/graf"
	"github.com/ethereum/hive/internal/libdocker/prom"
	"github.com/ethereum/hive/internal/libhive"
)

type Metrics struct {
	cb *ContainerBackend

	grafana    *libhive.ContainerInfo
	prometheus *libhive.ContainerInfo
}

// TODO: maybe add a node_exporter container so we can see how stressed the hive machine is while running tests?

// InitMetrics creates:
// - prometheus container that Hive will add/remove scrape targets to.
// - grafana container that will be pre-configured with dashboards that chart the metrics of the prometheus container.
// - a docker network for prometheus to connect with all containers Hive spins up, and grafana to reach prometheus.
func InitMetrics(ctx context.Context, cb *ContainerBackend, grafanaPort uint16, prometheusPort uint16) (*Metrics, error) {
	cb.logger.Info("initializing metrics")

	promOpts := libhive.ContainerOptions{CheckLive: 9090}
	if prometheusPort != 0 {
		promOpts.HostPorts = map[string][]string{"9090/tcp": {fmt.Sprintf("%d", prometheusPort)}}
	}
	// create prometheus
	promID, err := cb.CreateContainer(ctx, hivePrometheusTag, promOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to create prometheus container: %w", err)
	}
	promOpts.LogFile = filepath.Join(cb.config.Inventory.BaseDir, "workspace", "logs", fmt.Sprintf("prometheus-%s.log", promID))
	promContainer, err := cb.StartContainer(ctx, promID, promOpts)
	if err != nil {
		_ = cb.DeleteContainer(promID)
		return nil, fmt.Errorf("failed to start prometheus container %s: %w", promID, err)
	}

	if prometheusPort == 0 {
		cb.logger.Info("started prometheus for metrics collection")
	} else {
		cb.logger.Info(fmt.Sprintf("started prometheus, accessible at: http://localhost:%d", prometheusPort))
	}

	// Don't start grafana if the user only needs prometheus,
	// e.g. if we are running hive tests with metrics reports after the run.
	if grafanaPort == 0 {
		return &Metrics{
			cb:         cb,
			prometheus: promContainer,
		}, nil
	}

	grafOpts := libhive.ContainerOptions{
		CheckLive: 3000,
		HostPorts: map[string][]string{"3000/tcp": {fmt.Sprintf("%d", grafanaPort)}},
		Env: map[string]string{
			"HIVE_PROMETHEUS_PORT": "9090",
			"HIVE_PROMETHEUS_IP":   promContainer.IP,
		},
	}

	// create grafana
	grafID, err := cb.CreateContainer(ctx, hiveGrafanaTag, grafOpts)
	if err != nil {
		_ = cb.DeleteContainer(promID)
		return nil, fmt.Errorf("failed to create grafana container: %w", err)
	}

	// now insert prometheus as datasource into grafana.
	prometheusDatasource := `apiVersion: 1
datasources:
- name: prometheus
  type: prometheus
  access: proxy
  uid: "P1809F7CD0C75ACF3"
  url: "http://` + promContainer.IP + `:9090"
`
	datasourceFile, err := uploadReader("/etc/grafana/provisioning/datasources/prometheus.yaml", []byte(prometheusDatasource))
	if err != nil {
		_ = cb.DeleteContainer(promID)
		_ = cb.DeleteContainer(grafID)
		return nil, fmt.Errorf("failed to prepare prometheus datasource: %w", err)
	}
	if err := cb.client.UploadToContainer(grafID, docker.UploadToContainerOptions{
		Context:     ctx,
		InputStream: datasourceFile,
		Path:        "/",
	}); err != nil {
		_ = cb.DeleteContainer(promID)
		_ = cb.DeleteContainer(grafID)
		return nil, fmt.Errorf("failed to insert prometheus datasource into grafana container %s: %w", grafID, err)
	}

	grafOpts.LogFile = filepath.Join(cb.config.Inventory.BaseDir, "workspace", "logs", fmt.Sprintf("grafana-%s.log", grafID))
	grafContainer, err := cb.StartContainer(ctx, grafID, grafOpts)
	if err != nil {
		_ = cb.DeleteContainer(promID)
		_ = cb.DeleteContainer(grafID)
		return nil, fmt.Errorf("failed to start grafana container %s: %w", grafID, err)
	}

	cb.logger.Info(fmt.Sprintf("started grafana, accessible at: http://localhost:%d", grafanaPort))

	return &Metrics{
		cb:         cb,
		grafana:    grafContainer,
		prometheus: promContainer,
	}, nil
}

type scrapeTarget struct {
	Targets []string          `json:"targets"`
	Labels  map[string]string `json:"labels"`
}

func uploadReader(filePath string, content []byte) (io.Reader, error) {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: filePath, Mode: 0777, Size: int64(len(content))}); err != nil {
		return nil, fmt.Errorf("tar writer header failed: %w", err)
	}
	if _, err := tw.Write(content); err != nil {
		return nil, fmt.Errorf("tar writer content failed: %w", err)
	}
	if err := tw.Close(); err != nil {
		return nil, fmt.Errorf("tar writer close failed: %w", err)
	}
	return &buf, nil
}

func packageScrapeTargets(filePath string, targets []scrapeTarget) (io.Reader, error) {
	var fileContent bytes.Buffer
	if err := json.NewEncoder(&fileContent).Encode(targets); err != nil {
		return nil, fmt.Errorf("failed to encode scrape target config: %w", err)
	}
	return uploadReader(filePath, fileContent.Bytes())
}

// ScrapeMetrics adds the given container as a scrape target to the prometheus container managed by Hive,
// and connects the container to the hive metrics network.
func (m *Metrics) ScrapeMetrics(ctx context.Context, info *libhive.ContainerInfo, opts *libhive.MetricsOptions) error {
	m.cb.logger.Debug("adding scrape target to metrics", "container_id", info.ID)
	// create a file for prometheus scrape-target-discovery, unique to the container we are scraping.
	filePath := fmt.Sprintf("/etc/prometheus/hive-metrics/hive-%s.json", info.ID)
	target := scrapeTarget{
		Targets: []string{fmt.Sprintf("%s:%d", info.IP, opts.Port)},
		Labels:  opts.Labels,
	}
	pkg, err := packageScrapeTargets(filePath, []scrapeTarget{target})
	if err != nil {
		return fmt.Errorf("failed to make scrape target: %w", err)
	}

	// Upload the tar stream into the destination container.
	if err := m.cb.client.UploadToContainer(m.prometheus.ID, docker.UploadToContainerOptions{
		Context:     ctx,
		InputStream: pkg,
		Path:        "/",
	}); err != nil {
		return fmt.Errorf("failed to upload metrics scrape target config of %s to prometheus container %s", info.ID, m.prometheus.ID)
	}
	return nil
}

// StopScrapingMetrics removes the scrape target of the given container from prometheus.
// It does not disconnect the container from the hive metrics network,
// it is assumed that the container is closed anyway, or will soon be.
func (m *Metrics) StopScrapingMetrics(ctx context.Context, id string) error {
	resultInfo, err := m.cb.RunProgram(ctx, m.prometheus.ID, []string{"rm", fmt.Sprintf("/etc/prometheus/hive-metrics/hive-%s.json", id)})
	if err != nil {
		return fmt.Errorf("failed to remove scrape target of container %s: %w", id, err)
	} else if resultInfo.ExitCode != 0 {
		return fmt.Errorf("failed to remove scrape target of container %s with exit code %d, err: %s, out: %s",
			id, resultInfo.ExitCode, resultInfo.Stderr, resultInfo.Stdout)
	}
	return nil
}

// grafanaAnnotation encodes a grafana annotation, see:
// https://grafana.com/docs/grafana/latest/developers/http_api/annotations/
type grafanaAnnotation struct {
	Time    int64  `json:"time"`
	TimeEnd int64  `json:"timeEnd"`
	Text    string `json:"text"`
	// We don't use optional fields dashboardUID,panelId,tags
}

// AnnotateMetrics adds an annotation to the grafana metrics, if grafana is running.
func (m *Metrics) AnnotateMetrics(ctx context.Context, startTime, endTime time.Time, text string) error {
	if m.grafana == nil {
		return nil
	}
	annotation := grafanaAnnotation{
		Time:    startTime.UnixMilli(),
		TimeEnd: endTime.UnixMilli(),
		Text:    text,
	}
	annotationData, err := json.Marshal(&annotation)
	if err != nil {
		return fmt.Errorf("failed to encode annotation: %w", err)
	}
	resultInfo, err := m.cb.RunProgram(ctx, m.grafana.ID, []string{"bash", "/hive/annotate_metrics.sh", string(annotationData)})
	if err != nil {
		return fmt.Errorf("failed to annotate metrics: %w", err)
	} else if resultInfo.ExitCode != 0 {
		return fmt.Errorf("failed to annotate metrics with exit code %d, err: %s, out: %s",
			resultInfo.ExitCode, resultInfo.Stderr, resultInfo.Stdout)
	}
	return nil
}

const (
	hivePrometheusTag = "hive/hiveprom"
	hiveGrafanaTag    = "hive/grafana"
)

// BuildMetrics builds the docker images for metrics usage: prometheus and grafana.
func (b *ContainerBackend) BuildMetrics(ctx context.Context, builder libhive.Builder) error {
	if err := builder.BuildImage(ctx, hiveGrafanaTag, graf.GrafanaSource); err != nil {
		return fmt.Errorf("failed to build grafana container: %w", err)
	}

	return builder.BuildImage(ctx, hivePrometheusTag, prom.PrometheusSource)
}

// InitMetrics starts the docker network and docker containers for metrics collection
func (b *ContainerBackend) InitMetrics(ctx context.Context, grafanaPort uint, prometheusPort uint) error {
	if uint(uint16(grafanaPort)) != grafanaPort {
		return fmt.Errorf("invalid grafana port: %d", grafanaPort)
	}
	if uint(uint16(prometheusPort)) != prometheusPort {
		return fmt.Errorf("invalid prometheus port: %d", prometheusPort)
	}
	m, err := InitMetrics(ctx, b, uint16(grafanaPort), uint16(prometheusPort))
	if err != nil {
		return err
	}
	b.metrics = m
	return nil
}

func (b *ContainerBackend) CloseMetrics() {
	if b.metrics != nil {
		if b.metrics.prometheus != nil {
			err := b.client.RemoveContainer(docker.RemoveContainerOptions{ID: b.metrics.prometheus.ID, Force: true})
			if err != nil {
				b.logger.Error("failed to close hive prometheus container: %w", err)
			}
		}
		if b.metrics.grafana != nil {
			err := b.client.RemoveContainer(docker.RemoveContainerOptions{ID: b.metrics.grafana.ID, Force: true})
			if err != nil {
				b.logger.Error("failed to close hive grafana container: %w", err)
			}
		}
	}
}

// AnnotateMetrics adds an annotation to the grafana metrics, if metrics are enabled and grafana is running.
func (b *ContainerBackend) AnnotateMetrics(ctx context.Context, startTime, endTime time.Time, text string) error {
	b.logger.Debug("annotating metrics", "startTime", startTime, "endTime", endTime, "text", text)
	if b.metrics != nil {
		return b.metrics.AnnotateMetrics(ctx, startTime, endTime, text)
	}
	return nil
}
