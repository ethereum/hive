package libdocker

import (
	"archive/tar"
	"bytes"
	"context"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
	"gopkg.in/yaml.v3"

	"github.com/ethereum/hive/internal/libhive"
)

// Builder takes care of building docker images.
type Builder struct {
	client        *docker.Client
	config        *Config
	logger        log15.Logger
	authenticator Authenticator
}

func NewBuilder(client *docker.Client, cfg *Config, auth Authenticator) *Builder {
	b := &Builder{
		client:        client,
		config:        cfg,
		logger:        cfg.Logger,
		authenticator: auth,
	}
	if b.logger == nil {
		b.logger = log15.Root()
	}
	return b
}

// ReadClientMetadata reads metadata of the given client.
func (b *Builder) ReadClientMetadata(client libhive.ClientBuildInfo) (*libhive.ClientMetadata, error) {
	dir := b.config.Inventory.ClientDirectory(client)
	f, err := os.Open(filepath.Join(dir, "hive.yaml"))
	if err != nil {
		if os.IsNotExist(err) {
			// Eth1 client by default.
			return &libhive.ClientMetadata{Roles: []string{"eth1"}}, nil
		} else {
			return nil, fmt.Errorf("failed to read hive metadata file in '%s': %v", dir, err)
		}
	}
	defer f.Close()
	var out libhive.ClientMetadata
	if err := yaml.NewDecoder(f).Decode(&out); err != nil {
		return nil, fmt.Errorf("failed to decode hive metadata file in '%s': %v", dir, err)
	}
	return &out, nil
}

// BuildClientImage builds a docker image of the given client.
func (b *Builder) BuildClientImage(ctx context.Context, client libhive.ClientBuildInfo) (string, error) {
	dir := b.config.Inventory.ClientDirectory(client)
	tag := fmt.Sprintf("hive/clients/%s:latest", client.String())
	dockerFile := "Dockerfile"
	if client.DockerFile != "" {
		// Custom Dockerfile.
		dockerFile += "." + client.DockerFile
	}
	buildArgs := make([]docker.BuildArg, 0)
	for key, value := range client.BuildArguments {
		buildArgs = append(buildArgs, docker.BuildArg{Name: key, Value: value})
	}
	err := b.buildImage(ctx, dir, dockerFile, tag, buildArgs...)
	return tag, err
}

// BuildSimulatorImage builds a docker image of a simulator.
func (b *Builder) BuildSimulatorImage(ctx context.Context, name string) (string, error) {
	dir := b.config.Inventory.SimulatorDirectory(name)
	buildContextPath := dir
	buildDockerfile := "Dockerfile"
	// build context dir of simulator can be overridden with "hive_context.txt" file containing the desired build path
	if contextPathBytes, err := os.ReadFile(filepath.Join(filepath.FromSlash(dir), "hive_context.txt")); err == nil {
		buildContextPath = filepath.Join(dir, strings.TrimSpace(string(contextPathBytes)))
		if strings.HasPrefix(buildContextPath, "../") {
			return "", fmt.Errorf("cannot access build directory outside of Hive root: %q", buildContextPath)
		}
		if p, err := filepath.Rel(buildContextPath, filepath.Join(filepath.FromSlash(dir), "Dockerfile")); err != nil {
			return "", fmt.Errorf("failed to derive relative simulator Dockerfile path: %v", err)
		} else {
			buildDockerfile = p
		}
	}
	tag := fmt.Sprintf("hive/simulators/%s:latest", name)
	err := b.buildImage(ctx, buildContextPath, buildDockerfile, tag)
	return tag, err
}

// BuildImage creates a container by archiving the given file system,
// which must contain a file called "Dockerfile".
func (b *Builder) BuildImage(ctx context.Context, name string, fsys fs.FS) error {
	opts := b.buildConfig(ctx, name)
	pipeR, pipeW := io.Pipe()
	opts.InputStream = pipeR
	go b.archiveFS(ctx, pipeW, fsys)

	b.logger.Info("building image", "image", name, "nocache", opts.NoCache, "pull", b.config.PullEnabled)
	if err := b.client.BuildImage(opts); err != nil {
		b.logger.Error("image build failed", "image", name, "err", err)
		return err
	}
	return nil
}

func (b *Builder) buildConfig(ctx context.Context, name string) docker.BuildImageOptions {
	nocache := false
	if b.config.NoCachePattern != nil {
		nocache = b.config.NoCachePattern.MatchString(name)
	}
	opts := docker.BuildImageOptions{
		Context:      ctx,
		Name:         name,
		OutputStream: io.Discard,
		NoCache:      nocache,
		Pull:         b.config.PullEnabled,
	}
	if b.authenticator != nil {
		opts.AuthConfigs = b.authenticator.AuthConfigs()
	}
	if b.config.BuildOutput != nil {
		opts.OutputStream = b.config.BuildOutput
	}
	return opts
}

func (b *Builder) archiveFS(ctx context.Context, out io.WriteCloser, fsys fs.FS) error {
	defer out.Close()

	w := tar.NewWriter(out)
	err := fs.WalkDir(fsys, ".", func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if ctx.Err() != nil {
			return err
		}

		// Write header.
		if e.Type()&fs.ModeSymlink != 0 {
			return fmt.Errorf("%s: symlinks are not supported in BuildImage", path)
		}
		info, err := e.Info()
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		hdr, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		hdr.Name = path
		if err := w.WriteHeader(hdr); err != nil {
			return err
		}

		// Write file content.
		if e.Type().IsRegular() {
			file, err := fsys.Open(path)
			if err != nil {
				return err
			}
			if _, err := io.Copy(w, file); err != nil {
				file.Close()
				return err
			}
			file.Close()
		}

		return nil
	})

	if err != nil {
		return err
	}

	// TODO: errors
	w.Flush()
	w.Close()
	return nil
}

// ReadFile returns the content of a file in the given image. To do so, it creates a
// temporary container, downloads the file from it and destroys the container.
func (b *Builder) ReadFile(ctx context.Context, image, path string) ([]byte, error) {
	// Create the temporary container and ensure it's cleaned up.
	opt := docker.CreateContainerOptions{
		Context: ctx,
		Config:  &docker.Config{Image: image},
	}
	cont, err := b.client.CreateContainer(opt)
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
	dlopt := docker.DownloadFromContainerOptions{
		Path:         path,
		OutputStream: download,
		Context:      ctx,
	}
	if err := b.client.DownloadFromContainer(cont.ID, dlopt); err != nil {
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
func (b *Builder) buildImage(ctx context.Context, contextDir, dockerFile, imageTag string, buildArgs ...docker.BuildArg) error {
	logger := b.logger.New("image", imageTag)
	context, err := filepath.Abs(contextDir)
	if err != nil {
		logger.Error("can't find path to context directory", "err", err)
		return err
	}

	opts := b.buildConfig(ctx, imageTag)
	opts.ContextDir = context
	opts.Dockerfile = dockerFile
	logctx := []interface{}{"dir", contextDir, "nocache", opts.NoCache, "pull", opts.Pull}
	if len(buildArgs) > 0 {
		for _, arg := range buildArgs {
			logctx = append(logctx, arg.Name, arg.Value)
		}
		opts.BuildArgs = buildArgs
	}

	logger.Info("building image", logctx...)
	if err := b.client.BuildImage(opts); err != nil {
		logger.Error("image build failed", "err", err)
		return err
	}
	return nil
}
