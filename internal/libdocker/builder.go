package libdocker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"path/filepath"

	"github.com/ethereum/hive/internal/libhive"
	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// Builder takes care of building docker images.
type Builder struct {
	client *docker.Client
	config *Config
	logger log15.Logger
}

func NewBuilder(client *docker.Client, cfg *Config) *Builder {
	b := &Builder{client: client, config: cfg, logger: cfg.Logger}
	if b.logger == nil {
		b.logger = log15.Root()
	}
	return b
}

// BuildClientImage builds a docker image of the given client.
func (b *Builder) BuildClientImage(ctx context.Context, name string) (string, error) {
	dir := b.config.Inventory.ClientDirectory(name)
	_, branch := libhive.SplitClientName(name)
	tag := fmt.Sprintf("hive/clients/%s:latest", name)
	err := b.buildImage(ctx, dir, branch, tag)
	return tag, err
}

// BuildSimulatorImage builds a docker image of a simulator.
func (b *Builder) BuildSimulatorImage(ctx context.Context, name string) (string, error) {
	dir := b.config.Inventory.SimulatorDirectory(name)
	tag := fmt.Sprintf("hive/simulators/%s:latest", name)
	err := b.buildImage(ctx, dir, "", tag)
	return tag, err
}

// ReadFile returns the content of a file in the given image. To do so, it creates a
// temporary container, downloads the file from it and destroys the container.
func (b *Builder) ReadFile(image, path string) ([]byte, error) {
	// Create the temporary container and ensure it's cleaned up.
	cont, err := b.client.CreateContainer(docker.CreateContainerOptions{Config: &docker.Config{Image: image}})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := b.client.RemoveContainer(docker.RemoveContainerOptions{ID: cont.ID, Force: true}); err != nil {
			b.logger.Error("can't remove temporary container", "id", cont.ID[:8], "err", err)
		}
	}()

	// Download a tarball of the file from the container.
	download := new(bytes.Buffer)
	if err := b.client.DownloadFromContainer(cont.ID, docker.DownloadFromContainerOptions{
		Path:         path,
		OutputStream: download,
	}); err != nil {
		return nil, err
	}
	in := tar.NewReader(download)
	for {
		// Fetch the next file header from the archive.
		header, err := in.Next()
		if err != nil {
			return nil, err
		}
		// If it's the file we're looking for, save its contents.
		if header.Name == path[1:] {
			content := new(bytes.Buffer)
			if _, err := io.Copy(content, in); err != nil {
				return nil, err
			}
			return content.Bytes(), nil
		}
	}
}

// buildImage builds a single docker image from the specified context.
// branch specifes a build argument to use a specific base image branch or github source branch.
func (b *Builder) buildImage(ctx context.Context, contextDir, branch, imageTag string) error {
	nocache := false
	if b.config.NoCachePattern != nil {
		nocache = b.config.NoCachePattern.MatchString(imageTag)
	}

	logger := b.logger.New("image", imageTag)
	context, err := filepath.Abs(contextDir)
	if err != nil {
		logger.Error("failed to build docker image", "err", err)
		return err
	}
	opts := docker.BuildImageOptions{
		Context:      ctx,
		Name:         imageTag,
		ContextDir:   context,
		OutputStream: ioutil.Discard,
		Dockerfile:   "Dockerfile",
		NoCache:      nocache,
		Pull:         b.config.PullEnabled,
	}
	if b.config.BuildOutput != nil {
		opts.OutputStream = b.config.BuildOutput
	}
	logctx := []interface{}{"dir", contextDir, "nocache", opts.NoCache, "pull", opts.Pull}
	if branch != "" {
		logctx = append(logctx, "branch", branch)
		opts.BuildArgs = []docker.BuildArg{{Name: "branch", Value: branch}}
	}

	logger.Info("building image", logctx...)
	if err := b.client.BuildImage(opts); err != nil {
		logger.Error("failed to build docker image", "err", err)
		return err
	}
	return nil
}
