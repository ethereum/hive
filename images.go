// This file contains the utility methods to build docker images from contexts
// scattered over various folders and hierarchies.

package main

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// hiveImageNamespace is the prefix to assign to docker images when building them
// to avoid name collisions with local images.
const hiveImageNamespace = "hive"

// buildCacher defines the image building caching rules to allow requesting the
// rebuild of certain images once per run, while omitting rebuilding others.
type buildCacher struct {
	pattern *regexp.Regexp
	rebuilt map[string]bool
}

// newBuildCacher creates a new cache oracle for image building.
func newBuildCacher(pattern string) (*buildCacher, error) {
	// If no cache invalidation pattern was set, cache all
	if pattern == "" {
		return nil, nil
	}
	// Otherwise compile the pattern and set up the cacher
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	return &buildCacher{
		pattern: re,
		rebuilt: make(map[string]bool),
	}, nil
}

// buildShell builds the outer shell docker image for running the entirety of hive
// within an all encompassing container.
func buildShell(daemon *docker.Client, cacher *buildCacher) (string, error) {
	image := hiveImageNamespace + "/shell"
	return image, buildImage(daemon, image, ".", cacher, log15.Root())
}

// buildEthash builds the ethash DAG generator docker image to run before any real
// simulation needing it takes place.
func buildEthash(daemon *docker.Client, cacher *buildCacher) (string, error) {
	image := hiveImageNamespace + "/internal/ethash"
	return image, buildImage(daemon, image, filepath.Join("internal", "ethash"), cacher, log15.Root())
}

// buildClients iterates over all the known clients and builds a docker image for
// all unknown ones matching the given pattern.
func buildClients(daemon *docker.Client, pattern string, cacher *buildCacher) (map[string]string, error) {
	return buildNestedImages(daemon, "clients", pattern, "client", cacher)
}

// fetchClientVersions downloads the version json specs from all clients that
// match the given patten.
func fetchClientVersions(daemon *docker.Client, pattern string, cacher *buildCacher) (map[string]map[string]string, error) {
	// Build all the client that we need the versions of
	clients, err := buildClients(daemon, pattern, cacher)
	if err != nil {
		return nil, err
	}
	// Iterate over the images and collect the versions
	versions := make(map[string]map[string]string)
	for client, image := range clients {
		logger := log15.New("client", client)

		blob, err := downloadFromImage(daemon, image, "/version.json", logger)
		if err != nil {
			return nil, err
		}
		var version map[string]string
		if err := json.Unmarshal(blob, &version); err != nil {
			return nil, err
		}
		versions[client] = version
	}
	return versions, nil
}

// buildValidators iterates over all the known validators and builds a docker image
// for all unknown ones matching the given pattern.
func buildValidators(daemon *docker.Client, pattern string, cacher *buildCacher) (map[string]string, error) {
	return buildNestedImages(daemon, "validators", pattern, "validator", cacher)
}

// buildSimulators iterates over all the known simulators and builds a docker image
// for all unknown ones matching the given pattern.
func buildSimulators(daemon *docker.Client, pattern string, cacher *buildCacher) (map[string]string, error) {
	return buildNestedImages(daemon, "simulators", pattern, "simulator", cacher)
}

// buildBenchmarkers iterates over all the known benchmarkers and builds a docker image
// for all unknown ones matching the given pattern.
func buildBenchmarkers(daemon *docker.Client, pattern string, cacher *buildCacher) (map[string]string, error) {
	return buildNestedImages(daemon, "benchmarkers", pattern, "benchmarker", cacher)
}

// buildNestedImages iterates over a directory containing arbitrarilly nested
// docker image definitions and builds all of them matching the provided pattern.
func buildNestedImages(daemon *docker.Client, root string, pattern string, kind string, cacher *buildCacher) (map[string]string, error) {
	// Gather all the folders with Dockerfiles within them
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	names := []string{}
	if err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// If walking the images failed, bail out
		if err != nil {
			return err
		}
		// Otherwise if we've found a Dockerfile, add the parent
		if strings.HasSuffix(path, "Dockerfile") {
			if name := filepath.Dir(path); re.MatchString(name) {
				names = append(names, filepath.Join(strings.Split(name, string(filepath.Separator))[1:]...))
			}
			return filepath.SkipDir
		}
		// Continue walking the path
		return nil
	}); err != nil {
		return nil, err
	}
	// Iterate over all the matched specs and build their docker images
	images := make(map[string]string)
	for _, name := range names {
		var (
			context = filepath.Join(root, name)
			image   = filepath.Join(hiveImageNamespace, context)
			logger  = log15.New(kind, name)
		)
		if err := buildImage(daemon, image, context, cacher, logger); err != nil {
			return nil, fmt.Errorf("%s: %v", context, err)
		}
		images[name] = image
	}
	return images, nil
}

// buildImage builds a single docker image from the specified context.
func buildImage(daemon *docker.Client, image, context string, cacher *buildCacher, logger log15.Logger) error {
	var nocache bool
	if cacher != nil && cacher.pattern.MatchString(image) && !cacher.rebuilt[image] {
		cacher.rebuilt[image] = true
		nocache = true
	}
	logger.Info("building new docker image", "nocache", nocache)

	stream := io.Writer(new(bytes.Buffer))
	if *loglevelFlag > 5 {
		stream = os.Stderr
	}
	opts := docker.BuildImageOptions{
		Name:         image,
		ContextDir:   context,
		OutputStream: stream,
		NoCache:      nocache,
	}
	if err := daemon.BuildImage(opts); err != nil {
		logger.Error("failed to build docker image", "error", err)
		return err
	}
	return nil
}

// downloadFromImage retrieves a file from a docker image. To do so it creates a
// temporary container, downloads the file from it and destroys the container.
func downloadFromImage(daemon *docker.Client, image, path string, logger log15.Logger) ([]byte, error) {
	// Create the temporary container and ensure it's cleaned up
	cont, err := daemon.CreateContainer(docker.CreateContainerOptions{Config: &docker.Config{Image: image}})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := daemon.RemoveContainer(docker.RemoveContainerOptions{ID: cont.ID, Force: true}); err != nil {
			logger.Error("failed to delete temporary container", "id", cont.ID[:8], "error", err)
		}
	}()
	// Download a tarball of the file from the container
	download := new(bytes.Buffer)
	if err := daemon.DownloadFromContainer(cont.ID, docker.DownloadFromContainerOptions{
		Path:         path,
		OutputStream: download,
	}); err != nil {
		return nil, err
	}
	in := tar.NewReader(download)
	for {
		// Fetch the next file header from the archive
		header, err := in.Next()
		if err != nil {
			return nil, err
		}
		// If it's the file we're looking for, save its contents
		if header.Name == path[1:] {
			content := new(bytes.Buffer)
			if _, err := io.Copy(content, in); err != nil {
				return nil, err
			}
			return content.Bytes(), nil
		}
	}
}
