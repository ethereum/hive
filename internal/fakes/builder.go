package fakes

import (
	"context"
	"io/fs"

	"github.com/ethereum/hive/internal/libhive"
)

// BuilderHooks can be used to override the behavior of the fake builder.
type BuilderHooks struct {
	BuildClientImage    func(context.Context, string) (string, error)
	BuildSimulatorImage func(context.Context, string) (string, error)
	ReadFile            func(ctx context.Context, image string, file string) ([]byte, error)
	ReadClientMetadata  func(name string) (*libhive.ClientMetadata, error)
}

// fakeBuilder implements Backend without docker.
type fakeBuilder struct {
	hooks BuilderHooks
}

// NewBuilder creates a new fake builder.
func NewBuilder(hooks *BuilderHooks) libhive.Builder {
	b := &fakeBuilder{}
	if hooks != nil {
		b.hooks = *hooks
	}
	return b
}

func (b *fakeBuilder) BuildClientImage(ctx context.Context, client string) (string, error) {
	if b.hooks.BuildClientImage != nil {
		return b.hooks.BuildClientImage(ctx, client)
	}
	return "fakebuild/client/" + client + ":latest", nil
}

func (b *fakeBuilder) BuildSimulatorImage(ctx context.Context, sim string) (string, error) {
	if b.hooks.BuildSimulatorImage != nil {
		return b.hooks.BuildSimulatorImage(ctx, sim)
	}
	return "fakebuild/simulator/" + sim + ":latest", nil
}

func (b *fakeBuilder) BuildImage(ctx context.Context, name string, fsys fs.FS) error {
	return nil
}

func (b *fakeBuilder) ReadClientMetadata(name string) (*libhive.ClientMetadata, error) {
	if b.hooks.ReadClientMetadata != nil {
		return b.hooks.ReadClientMetadata(name)
	}
	m := libhive.ClientMetadata{Roles: []string{"eth1"}}
	return &m, nil
}

func (b *fakeBuilder) ReadFile(ctx context.Context, image, file string) ([]byte, error) {
	if b.hooks.ReadFile != nil {
		return b.hooks.ReadFile(ctx, image, file)
	}
	return []byte{}, nil
}
