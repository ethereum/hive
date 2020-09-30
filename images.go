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

	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// hiveImageNamespace is the prefix to assign to docker images when building them
// to avoid name collisions with local images.
const hiveImageNamespace = "hive"

// branchDelimiter is what separates the client name from the branch, eg: aleth_nightly, go-ethereum_master
const branchDelimiter = "_"

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
func buildShell(cacher *buildCacher) (string, error) {
	image := hiveImageNamespace + "/shell"
	return image, buildImage(image, "", ".", cacher, log15.Root(), "")
}

// buildClients iterates over all the known clients and builds a docker image for
// all unknown ones matching the given pattern.
func buildClients(clientList []string, cacher *buildCacher) (map[string]string, error) {
	return buildListedImages("clients", clientList, "client", cacher, false)
}

// buildPseudoClients iterates over all the known pseudo-clients and builds a docker image for
func buildPseudoClients(pattern string, cacher *buildCacher) (map[string]string, error) {
	return buildNestedImages("pseudoclients", pattern, "pseudoclient", cacher, false)
}

// fetchClientVersions downloads the version json specs from all clients that
// match the given patten.
func fetchClientVersions(cacher *buildCacher) (map[string]map[string]string, error) {

	// Iterate over the images and collect the versions
	versions := make(map[string]map[string]string)
	for client, image := range allClients {
		logger := log15.New("client", client)
		blob, err := downloadFromImage(image, "/version.json", logger)
		if err != nil {
			berr := &buildError{err: err, client: client}
			return nil, berr
		}
		var version map[string]string
		if err := json.Unmarshal(blob, &version); err != nil {
			berr := &buildError{err: err, client: client}
			return nil, berr
		}
		versions[client] = version
	}
	return versions, nil
}

// buildSimulators iterates over all the known simulators and builds a docker image
// for all unknown ones matching the given pattern.
func buildSimulators(pattern string, cacher *buildCacher) (map[string]string, error) {
	images, err := buildNestedImages("simulators", pattern, "simulator", cacher, *simRootContext)
	return images, err
}

// buildNestedImages iterates over a directory containing arbitrarilly nested
// docker image definitions and builds all of them matching the provided pattern.
func buildNestedImages(root string, pattern string, kind string, cacher *buildCacher, rootContext bool) (map[string]string, error) {

	var contextBuilder func(root string, path string) (string, string)

	if rootContext {
		contextBuilder = func(root string, path string) (string, string) {
			return root, strings.Replace(path+string(filepath.Separator)+"Dockerfile", "\\", "/", -1)
		}
	} else {
		contextBuilder = func(root string, path string) (string, string) {
			return filepath.Join(root, path), ""
		}
	}

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
			//return filepath.SkipDir
		}
		// Continue walking the path
		return nil
	}); err != nil {
		return nil, err
	}

	if len(names) < 1 {
		return nil, fmt.Errorf("could not find simulation %s", pattern)
	}

	// Iterate over all the matched specs and build their docker images
	images := make(map[string]string)
	for _, name := range names {
		var (
			context, dockerfile = contextBuilder(root, name)
			image               = strings.Replace(filepath.Join(hiveImageNamespace, root, name), string(os.PathSeparator), "/", -1)
			logger              = log15.New(kind, name)
		)
		if err := buildImage(image, "", context, cacher, logger, dockerfile); err != nil {
			berr := &buildError{err: fmt.Errorf("%s: %v", context, err), client: name}
			return nil, berr
		}
		images[name] = image
	}
	return images, nil
}

// buildListedImages iterates over a directory containing arbitrarilly nested
// docker image definitions and builds those whose directory names contain the
// image name string in the client List, with one image per branch.
// For example, if the clientList contained geth_master, geth_beta and
// the clients folder contained a dockerfile for clients\geth, then this
// will created two images, one for clients\geth_master and one for clients\geth_beta
func buildListedImages(root string, clientList []string, kind string, cacher *buildCacher, rootContext bool) (map[string]string, error) {

	var contextBuilder func(root string, path string) (string, string, string)

	if rootContext {
		contextBuilder = func(root string, path string) (string, string, string) {
			branch := getBranch(path)
			path = strings.TrimSuffix(path, branchDelimiter+branch)
			return root, branch, strings.Replace(path+string(filepath.Separator)+"Dockerfile", "\\", "/", -1)
		}
	} else {
		contextBuilder = func(root string, path string) (string, string, string) {
			branch := getBranch(path)
			path = strings.TrimSuffix(path, branchDelimiter+branch)
			return filepath.Join(root, path), branch, ""
		}
	}

	names := []string{}
	if err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		// If walking the images failed, bail out
		if err != nil {
			return err
		}
		// Otherwise if we've found a Dockerfile, add the parent
		if strings.HasSuffix(path, "Dockerfile") {
			name := filepath.Dir(path)

			//get a list of matching clients, including a branch suffix after '_' (branchDelimiter)
			//TODO - update docs to say that folders under 'clients' should not use _ underscore
			matchNames(name, clientList, &names)

		}
		// Continue walking the path
		return nil
	}); err != nil {
		return nil, err
	}
	// list all given client names that were not found in the `clients` directory
	notFound := notFound(names, clientList)
	// only throw error if the given client pattern was not found (e.g. "bes" is technically incorrect,
	// but the pattern matches "besu", so the client is still found)
	if len(notFound) > 0 && len(names) != len(clientList) {
		for _, notFoundDockerfile := range notFound {
			log15.Crit("Could not find client image", "image", notFoundDockerfile)
		}
	}
	// if no clients were found, error out
	if len(names) < 1 {
		return nil, fmt.Errorf("no client images to build") // TODO fix err message
	}

	// Iterate over all the matched specs and build their docker images
	images := make(map[string]string)
	for _, name := range names {
		var (
			context, branch, dockerfile = contextBuilder(root, name)
			image                       = strings.Replace(filepath.Join(hiveImageNamespace, root, name), string(os.PathSeparator), "/", -1)
			logger                      = log15.New(kind, name)
		)
		if err := buildImage(image, branch, context, cacher, logger, dockerfile); err != nil {
			berr := &buildError{err: fmt.Errorf("%s: %v", context, err), client: name}
			return nil, berr
		}
		images[name] = image
	}
	return images, nil
}

func notFound(names []string, all []string) []string {
	found := make(map[string]string, len(names))
	for _, name := range names {
		found[name] = name
	}

	var notFound []string
	for _, client := range all {
		if _, exists := found[client]; !exists {
			notFound = append(notFound, client)
		}
	}

	return notFound
}

func getBranch(name string) string {
	branch := ""
	if branchIndex := strings.LastIndex(name, branchDelimiter); branchIndex > 0 && branchIndex < len(name) {
		branch = name[branchIndex+1:]
	}
	return branch
}

func matchNames(name string, clientList []string, names *[]string) {
	for _, client := range clientList {

		branch := getBranch(client)
		if len(branch) > 0 {
			branch = branchDelimiter + branch
		}
		clientWithoutBranch := strings.TrimSuffix(client, branch)

		if strings.Contains(name, clientWithoutBranch) {
			*names = append(*names, filepath.Join(strings.Split(name, string(filepath.Separator))[1:]...)+branch)
		}
	}
}

type buildError struct {
	err    error
	client string
}

func (b *buildError) Error() string {
	return b.err.Error()
}

func (b *buildError) Client() string {
	return b.client
}

// buildImage builds a single docker image from the specified context.
// branch specifes a build argument to use a specific base image branch or github source branch
func buildImage(image, branch, context string, cacher *buildCacher, logger log15.Logger, dockerfile string) error {
	var nocache bool
	if cacher != nil && cacher.pattern.MatchString(image) && !cacher.rebuilt[image] {
		cacher.rebuilt[image] = true
		nocache = true
	}
	logger.Info("building new docker image", "image", image, "context", context, "dockerfile", dockerfile,
		"branch", branch, "nocache", nocache)

	context, err := filepath.Abs(context)
	if err != nil {
		logger.Error("failed to build docker image", "image", image, "context", context, "dockerfile", dockerfile,
			"branch", branch, "error", err)
		return err
	}
	stream := io.Writer(new(bytes.Buffer))
	if *loglevelFlag > 5 {
		stream = os.Stderr
	}
	opts := docker.BuildImageOptions{
		Name:         image,
		ContextDir:   context,
		Dockerfile:   dockerfile,
		OutputStream: stream,
		NoCache:      nocache,
		BuildArgs:    []docker.BuildArg{docker.BuildArg{Name: "branch", Value: branch}},
	}
	if err := dockerClient.BuildImage(opts); err != nil {
		logger.Error("failed to build docker image",
			"image", image, "context", context, "dockerfile", dockerfile,
			"branch", branch, "error", err)
		return err
	}
	return nil
}

// downloadFromImage retrieves a file from a docker image. To do so it creates a
// temporary container, downloads the file from it and destroys the container.
func downloadFromImage(image, path string, logger log15.Logger) ([]byte, error) {
	// Create the temporary container and ensure it's cleaned up
	cont, err := dockerClient.CreateContainer(docker.CreateContainerOptions{Config: &docker.Config{Image: image}})
	if err != nil {
		return nil, err
	}
	defer func() {
		if err := dockerClient.RemoveContainer(docker.RemoveContainerOptions{ID: cont.ID, Force: true}); err != nil {
			logger.Error("failed to delete temporary container", "id", cont.ID[:8], "error", err)
		}
	}()
	// Download a tarball of the file from the container
	download := new(bytes.Buffer)
	if err := dockerClient.DownloadFromContainer(cont.ID, docker.DownloadFromContainerOptions{
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
