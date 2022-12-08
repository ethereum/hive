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

func NewAuthenticator(useCredentialHelper bool) (Authenticator, error) {

	if useCredentialHelper {
		return NewCredHelperAuthenticator()
	}
	return NullAuthenticator{}, nil
}

// Authenticator is able to return the 2 go-dockerclient authentication primitives
type Authenticator interface {
	// AuthConfig returns the auth configuration for a specific registry
	AuthConfig(registry string) docker.AuthConfiguration
	// AuthConfigs returns the auth configurations for all configured registries
	AuthConfigs() docker.AuthConfigurations
}

// NullAuthenticator returns empty authentication configs
type NullAuthenticator struct{}

// AuthConfig returns the auth configuration for a specific registry
func (n NullAuthenticator) AuthConfig(registry string) (a docker.AuthConfiguration) { return }

// AuthConfigs returns the auth configurations for all configured registries
func (n NullAuthenticator) AuthConfigs() (a docker.AuthConfigurations) { return }

func NewCredHelperAuthenticator() (CredHelperAuthenticator, error) {
	authConfigs, err := configureCredHelperAuth()
	return CredHelperAuthenticator{authConfigs}, err
}

type CredHelperAuthenticator struct {
	configs docker.AuthConfigurations
}

// AuthConfig returns the auth configuration for a specific registry
func (c CredHelperAuthenticator) AuthConfig(registry string) (a docker.AuthConfiguration) {
	return c.configs.Configs[registry]
}

// AuthConfigs returns the auth configurations for all configured registries
func (c CredHelperAuthenticator) AuthConfigs() (a docker.AuthConfigurations) { return c.configs }

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
