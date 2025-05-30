package libdocker

import (
	docker "github.com/fsouza/go-dockerclient"
)

// Authenticator holds the configuration for docker registry authentication.
type Authenticator interface {
	// AuthConfigs returns the auth configurations for all configured registries.
	AuthConfigs() docker.AuthConfigurations
}

func NewCredHelperAuthenticator() (CredHelperAuthenticator, error) {
	authConfigs, err := docker.NewAuthConfigurationsFromDockerCfg()
	if err != nil {
		return CredHelperAuthenticator{}, err
	}
	return CredHelperAuthenticator{*authConfigs}, nil
}

type CredHelperAuthenticator struct {
	configs docker.AuthConfigurations
}

// AuthConfigs returns the auth configurations for all configured registries
func (c CredHelperAuthenticator) AuthConfigs() docker.AuthConfigurations {
	return c.configs
}
