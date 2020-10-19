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
	"path"
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
func buildClients(clientList []string, cacher *buildCacher, errorReport *HiveErrorReport) (map[string]string, error) {
	return buildListedImages("clients", clientList, "client", cacher, false, errorReport)
}

// buildPseudoClients iterates over all the known pseudo-clients and builds a docker image for
func buildPseudoClients(pattern string, cacher *buildCacher, errorReport *HiveErrorReport) (map[string]string, error) {
	return buildNestedImages("pseudoclients", pattern, "pseudoclient", cacher, false, errorReport)
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
func buildSimulators(pattern string, cacher *buildCacher, errorReport *HiveErrorReport) (map[string]string, error) {
	images, err := buildNestedImages("simulators", pattern, "simulator", cacher, *simRootContext, errorReport)
	return images, err
}

// buildNestedImages iterates over a directory containing arbitrarilly nested
// docker image definitions and builds all of them matching the provided pattern.
func buildNestedImages(root string, pattern string, kind string, cacher *buildCacher, rootContext bool, errorReport *HiveErrorReport) (map[string]string, error) {

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
		errorReport.AddErrorReport(ContainerError{
			Name:    pattern,
			Details: "could not find simulation",
		})
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

// buildListedImages iterates over a directory containing docker image definitions and
// builds those whose directory names contain the image name string in the client List,
// with one image per branch.
//
// For example, if the clientList contains geth_master,geth_beta and the directory
// contains a Dockerfile in clients/geth, this will create two images: clients/geth_master
// and clients/geth_beta.
func buildListedImages(root string, clientList []string, kind string, cacher *buildCacher, rootContext bool, errorReport *HiveErrorReport) (map[string]string, error) {
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

	// Get a list of matching clients, including a branch suffix after '_' (branchDelimiter).
	var names []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if filepath.Base(path) == "Dockerfile" {
			names = append(names, matchNames(path, clientList)...)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// list all given client names that were not found in the `clients` directory
	notFound := notFound(names, clientList)
	if len(notFound) > 0 && len(names) != len(clientList) {
		for _, notFoundDockerfile := range notFound {
			log15.Crit("Could not find client image", "image", notFoundDockerfile)
			errorReport.AddErrorReport(ContainerError{
				Name:    notFoundDockerfile,
				Details: "could not find client image",
			})
		}
		return nil, fmt.Errorf("invalid client image(s) specified")
	}
	if len(names) == 0 {
		return nil, fmt.Errorf("no client images to build")
	}

	// Iterate over all the matched specs and build their docker images
	images := make(map[string]string)
	for _, name := range names {
		var (
			context, branch, dockerfile = contextBuilder(root, name)
			image                       = path.Join(hiveImageNamespace, root, name)
			logger                      = log15.New(kind, name)
		)
		if err := buildImage(image, branch, context, cacher, logger, dockerfile); err != nil {
			errorDetails := fmt.Errorf("%s: %v", context, err)
			berr := &buildError{err: errorDetails, client: name}
			// report error
			errorReport.AddErrorReport(ContainerError{
				Name:    image,
				Details: errorDetails.Error(),
			})
			// if there is only one client to test and it fails, error out,
			// otherwise proceed building other clients and log error
			if len(names) < 2 {
				return nil, berr
			}
			log15.Crit("image failed to build", "error", berr)
		} else {
			images[name] = image
		}
	}
	return images, nil
}

// notFound returns the elements of 'all' which are not contained in 'names'.
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

// getBranch returns the branch name component of 'name'.
func getBranch(name string) string {
	branch := ""
	if ix := strings.LastIndex(name, branchDelimiter); ix > 0 {
		branch = name[ix+1:]
	}
	return branch
}

// matchNames matches a Dockerfile path against all specified client names. Note that
// clients may be given multiple times with different branch names. The returned slice
// contains the matching client names.
func matchNames(dockerfile string, clientList []string) []string {
	dir := filepath.Dir(dockerfile)
	base := filepath.Base(dir)

	var m []string
	for _, client := range clientList {
		branch := getBranch(client)
		if len(branch) > 0 {
			branch = branchDelimiter + branch
		}
		clientWithoutBranch := strings.TrimSuffix(client, branch)
		if base == clientWithoutBranch {
			m = append(m, client)
		}
	}
	return m
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
	}
	if branch != "" {
		opts.BuildArgs = []docker.BuildArg{docker.BuildArg{Name: "branch", Value: branch}}
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
