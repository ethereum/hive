package libdocker

import (
	"encoding/json"
	"os"
	"path"

	docker "github.com/fsouza/go-dockerclient"
)

// CredHelpers represents the data stored by default in $HOME/.docker/config.json
type CredHelpers struct {
	CredHelpers map[string]string `json:"credHelpers"`
	CredsStore  string            `json:"credsStore"`
}

// Authenticator holds the configuration for docker registry authentication.
type Authenticator interface {
	// AuthConfigs returns the auth configurations for all configured registries.
	AuthConfigs() docker.AuthConfigurations
}

func NewCredHelperAuthenticator() (CredHelperAuthenticator, error) {
	authConfigs, err := configureCredHelperAuth()
	return CredHelperAuthenticator{authConfigs}, err
}

type CredHelperAuthenticator struct {
	configs docker.AuthConfigurations
}

// AuthConfigs returns the auth configurations for all configured registries
func (c CredHelperAuthenticator) AuthConfigs() (a docker.AuthConfigurations) {
	return c.configs
}

// configureCredHelperAuth - configures authentication for the specified registry based on $HOME/.docker/config.json
func configureCredHelperAuth() (docker.AuthConfigurations, error) {
	authConfigurations := make(map[string]docker.AuthConfiguration)
	credsHelpers, err := loadCredsHelpers()
	if err != nil {
		return docker.AuthConfigurations{}, err
	}

	for registry := range credsHelpers.CredHelpers {
		authConfig, err := docker.NewAuthConfigurationsFromCredsHelpers(registry)
		if err != nil {
			return docker.AuthConfigurations{
				Configs: authConfigurations,
			}, err
		}
		authConfigurations[registry] = *authConfig
	}
	return docker.AuthConfigurations{Configs: authConfigurations}, nil
}

func loadCredsHelpers() (c CredHelpers, err error) {
	cfgPath, err := dockerConfigPath()
	if err != nil {
		return
	}
	b, err := os.ReadFile(cfgPath)
	if err != nil {
		return
	}
	err = json.Unmarshal(b, &c)
	return
}

func dockerConfigPath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return homeDir, err
	}
	return path.Join(homeDir, ".docker", "config.json"), nil
}
