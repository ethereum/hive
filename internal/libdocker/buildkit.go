package libdocker

import (
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/containerd/console"
	dockerclient "github.com/fsouza/go-dockerclient"
	"github.com/moby/buildkit/client"
	dockerfile "github.com/moby/buildkit/frontend/dockerfile/builder"
	"github.com/moby/buildkit/util/appdefaults"
	"github.com/moby/buildkit/util/progress/progressui"
	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type buildkitOptions struct {
	buildkitdAddr string

	buildArgs  []string
	buildCtx   string
	dockerfile string
	tag        string
	noCache    bool
}

func (b *Builder) buildWithBuildkit(ctx context.Context, opt *buildkitOptions) error {
	addr := appdefaults.Address
	if opt.buildkitdAddr != "" {
		addr = opt.buildkitdAddr
	} else if ea := os.Getenv("BUILDKIT_HOST"); ea != "" {
		addr = ea
	}

	c, err := client.New(ctx, addr, client.WithFailFast())
	if err != nil {
		return err
	}

	pipeR, pipeW := io.Pipe()
	solveOpt, err := newSolveOpt(opt, pipeW)
	if err != nil {
		return err
	}

	ch := make(chan *client.SolveStatus)
	eg, ctx := errgroup.WithContext(ctx)
	eg.Go(func() error {
		var err error
		_, err = c.Build(ctx, *solveOpt, "", dockerfile.Build, ch)
		return err
	})
	eg.Go(func() error {
		var c console.Console
		if cn, err := console.ConsoleFromFile(os.Stderr); err == nil {
			c = cn
		}
		// Not using shared context to not disrupt display but let it finish reporting
		// errors.
		_, err = progressui.DisplaySolveStatus(context.TODO(), "", c, os.Stdout, ch)
		return err
	})
	eg.Go(func() error {
		if err := b.loadImageTarball(ctx, pipeR); err != nil {
			return err
		}
		return pipeR.Close()
	})
	return eg.Wait()
}

func newSolveOpt(opt *buildkitOptions, w io.WriteCloser) (*client.SolveOpt, error) {
	if opt.buildCtx == "" {
		return nil, errors.New("please specify build context (e.g. \".\" for the current directory)")
	}

	file := opt.dockerfile
	if file == "" {
		file = filepath.Join(opt.buildCtx, "Dockerfile")
	}
	localDirs := map[string]string{
		"context":    opt.buildCtx,
		"dockerfile": filepath.Dir(file),
	}

	frontendAttrs := map[string]string{
		"filename": filepath.Base(file),
	}
	if opt.noCache {
		frontendAttrs["no-cache"] = ""
	}

	for _, buildArg := range opt.buildArgs {
		kv := strings.SplitN(buildArg, "=", 2)
		if len(kv) != 2 {
			return nil, errors.Errorf("invalid build-arg value %s", buildArg)
		}
		frontendAttrs["build-arg:"+kv[0]] = kv[1]
	}

	solveOpt := &client.SolveOpt{
		Exports: []client.ExportEntry{
			{
				Type: "docker", // TODO: use containerd image store when it is integrated to Docker
				Attrs: map[string]string{
					"name": opt.tag,
				},
				Output: func(_ map[string]string) (io.WriteCloser, error) {
					return w, nil
				},
			},
		},
		LocalDirs:     localDirs,
		Frontend:      "",
		FrontendAttrs: frontendAttrs,
	}
	return solveOpt, nil
}

func (b *Builder) loadImageTarball(ctx context.Context, r io.Reader) error {
	return b.client.LoadImage(dockerclient.LoadImageOptions{
		Context:     ctx,
		InputStream: r,
	})
}
