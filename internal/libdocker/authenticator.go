package libdocker

import (
	"fmt"

	docker "github.com/fsouza/go-dockerclient"
)

type AuthType string

const (
	AuthTypeNone       AuthType = ""
	AuthTypeCredHelper AuthType = "cred-helper"
)

type ErrUnsupportedAuthType struct{ authType string }

func (e ErrUnsupportedAuthType) Error() string {
	return fmt.Sprintf("unsupported auth type: %q", e.authType)
}

func NewAuthenticator(authType AuthType, registries ...string) (Authenticator, error) {
	switch authType {
	case AuthTypeNone:
		return NullAuthenticator{}, nil
	case AuthTypeCredHelper:
		return NewCredHelperAuthenticator(registries...)
	default:
		return nil, ErrUnsupportedAuthType{authType: string(authType)}
	}
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

func NewCredHelperAuthenticator(registries ...string) (CredHelperAuthenticator, error) {
	authConfigs, err := configureCredHelperAuth(registries...)
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

// configureAuth - configures authentication for the specified registry based on $HOME/.docker/config.json
func configureCredHelperAuth(registries ...string) (docker.AuthConfigurations, error) {
	authConfigurations := make(map[string]docker.AuthConfiguration)
	for _, registry := range registries {
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
