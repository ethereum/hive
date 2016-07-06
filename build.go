// This file contains the utility methods to build docker images from contexts
// scattered over various folders and hierarchies.

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// hiveImageNamespace is the prefix to assign to docker images when building them
// to avoid name collisions with local images.
const hiveImageNamespace = "hive"

// buildClients iterates over all the known clients and builds a docker image for
// all unknown ones matching the given pattern. If nocache was requested, images
// are attempted to be rebuilt.
func buildClients(daemon *docker.Client, pattern string, nocache bool) (map[string]string, error) {
	return buildNestedImages(daemon, "clients", pattern, "client", nocache)
}

// buildValidators iterates over all the known validators and builds a docker image
// for all unknown ones matching the given pattern. If nocache was requested, images
// are attempted to be rebuilt.
func buildValidators(daemon *docker.Client, pattern string, nocache bool) (map[string]string, error) {
	return buildNestedImages(daemon, "validators", pattern, "validator", nocache)
}

// buildSimulators iterates over all the known simulators and builds a docker image
// for all unknown ones matching the given pattern. If nocache was requested, images
// are attempted to be rebuilt.
func buildSimulators(daemon *docker.Client, pattern string, nocache bool) (map[string]string, error) {
	return buildNestedImages(daemon, "simulators", pattern, "simulator", nocache)
}

// buildNestedImages iterates over a directory containing arbitrarilly nested
// docker image definitions and builds all of them matching the provided pattern.
func buildNestedImages(daemon *docker.Client, root string, pattern string, kind string, nocache bool) (map[string]string, error) {
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
		if err := buildImage(daemon, image, context, nocache, logger); err != nil {
			return nil, fmt.Errorf("%s: %v", context, err)
		}
		images[context] = image
	}
	return images, nil
}

// buildImage builds a single docker image from the specified context.
func buildImage(daemon *docker.Client, image, context string, nocache bool, logger log15.Logger) error {
	// Skip the build overhead if an image already exists
	if !nocache {
		if info, err := daemon.InspectImage(image); err == nil {
			logger.Info("docker image already exists", "age", time.Since(info.Created))
			return nil
		}
	}
	// Otherwise build a new docker image for the specified context
	logger.Info("building new docker image")

	r, w := io.Pipe()
	go io.Copy(os.Stderr, r)

	opts := docker.BuildImageOptions{
		Name:         image,
		ContextDir:   context,
		OutputStream: w,
	}
	if err := daemon.BuildImage(opts); err != nil {
		logger.Error("failed to build docker image", "error", err)
		return err
	}
	return nil
}
