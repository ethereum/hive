// This file contains the utility methods to build docker images from contexts
// scattered over various folders and hierarchies.

package main

import (
	"bytes"
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

// buildShell builds the outer shell docker image for running the entirety of hive
// within an all encompassing container.
func buildShell(daemon *docker.Client) (string, error) {
	image := hiveImageNamespace + "/shell"
	return image, buildImage(daemon, image, ".", log15.Root())
}

// buildEthash builds the ethash DAG generator docker image to run before any real
// simulation needing it takes place.
func buildEthash(daemon *docker.Client) (string, error) {
	image := hiveImageNamespace + "/internal/ethash"
	return image, buildImage(daemon, image, filepath.Join("internal", "ethash"), log15.Root())
}

// buildClients iterates over all the known clients and builds a docker image for
// all unknown ones matching the given pattern.
func buildClients(daemon *docker.Client, pattern string) (map[string]string, error) {
	return buildNestedImages(daemon, "clients", pattern, "client")
}

// buildValidators iterates over all the known validators and builds a docker image
// for all unknown ones matching the given pattern.
func buildValidators(daemon *docker.Client, pattern string) (map[string]string, error) {
	return buildNestedImages(daemon, "validators", pattern, "validator")
}

// buildSimulators iterates over all the known simulators and builds a docker image
// for all unknown ones matching the given pattern.
func buildSimulators(daemon *docker.Client, pattern string) (map[string]string, error) {
	return buildNestedImages(daemon, "simulators", pattern, "simulator")
}

// buildBenchmarkers iterates over all the known benchmarkers and builds a docker image
// for all unknown ones matching the given pattern.
func buildBenchmarkers(daemon *docker.Client, pattern string) (map[string]string, error) {
	return buildNestedImages(daemon, "benchmarkers", pattern, "benchmarker")
}

// buildNestedImages iterates over a directory containing arbitrarilly nested
// docker image definitions and builds all of them matching the provided pattern.
func buildNestedImages(daemon *docker.Client, root string, pattern string, kind string) (map[string]string, error) {
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
		if err := buildImage(daemon, image, context, logger); err != nil {
			return nil, fmt.Errorf("%s: %v", context, err)
		}
		images[name] = image
	}
	return images, nil
}

// buildImage builds a single docker image from the specified context.
func buildImage(daemon *docker.Client, image, context string, logger log15.Logger) error {
	logger.Info("building new docker image")

	stream := io.Writer(new(bytes.Buffer))
	if *loglevelFlag > 5 {
		stream = os.Stderr
	}
	opts := docker.BuildImageOptions{
		Name:         image,
		ContextDir:   context,
		OutputStream: stream,
	}
	if err := daemon.BuildImage(opts); err != nil {
		logger.Error("failed to build docker image", "error", err)
		return err
	}
	return nil
}
