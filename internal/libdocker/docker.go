package libdocker

import (
	"fmt"
	"io"
	"log/slog"
	"regexp"

	"github.com/ethereum/hive/internal/libhive"
	docker "github.com/fsouza/go-dockerclient"
)

const apiVersion = "1.44"

// Config is the configuration of the docker backend.
type Config struct {
	Inventory libhive.Inventory

	Logger *slog.Logger

	// When building containers, any client or simulator image build matching the pattern
	// will avoid the docker cache.
	NoCachePattern *regexp.Regexp

	// This forces pulling of base images when building clients and simulators.
	PullEnabled bool

	// These two are log destinations for output from docker.
	ContainerOutput io.Writer
	BuildOutput     io.Writer

	// This tells the docker client whether to authenticate requests
	UseAuthentication bool
}

func Connect(dockerEndpoint string, cfg *Config) (*Builder, *ContainerBackend, error) {
	logger := cfg.Logger
	if logger == nil {
		logger = slog.Default()
	}
	var client *docker.Client
	var err error
	if dockerEndpoint == "" {
		client, err = docker.NewVersionedClientFromEnv(apiVersion)
	} else {
		client, err = docker.NewVersionedClient(dockerEndpoint, apiVersion)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("can't connect to docker: %v", err)
	}
	env, err := client.Version()
	if err != nil {
		return nil, nil, fmt.Errorf("can't get docker version: %v", err)
	}
	logger.Debug("docker daemon online", "version", env.Get("Version"))

	builder, err := createBuilder(client, cfg)
	if err != nil {
		return nil, nil, err
	}
	backend := NewContainerBackend(client, cfg)
	return builder, backend, nil
}

func createBuilder(client *docker.Client, cfg *Config) (*Builder, error) {
	var auth *docker.AuthConfigurations
	var err error
	if cfg.UseAuthentication {
		auth, err = docker.NewAuthConfigurationsFromDockerCfg()
		if err != nil {
			return nil, err
		}
	}
	return NewBuilder(client, cfg, auth), nil
}
