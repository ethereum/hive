package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"time"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// buildClients iterates over all the known clients and builds a docker image for
// all unknown ones.
func buildClients(daemon *docker.Client, pattern string, nocache bool) (map[string]string, error) {
	// Gather all the clients matching the pattern
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	folder, err := ioutil.ReadDir("clients")
	if err != nil {
		return nil, err
	}
	clients := make(map[string]string)
	for _, folder := range folder {
		if name := folder.Name(); re.MatchString(name) {
			clients[name] = clientImagePrefix + name
		}
	}
	// Iterate over all matched clients and build their docker images
	for client, image := range clients {
		var (
			context = filepath.Join("clients", client)
			logger  = log15.New("client", client)
		)
		if err := buildImage(daemon, image, context, nocache, logger); err != nil {
			return nil, fmt.Errorf("%s: %v", client, err)
		}
	}
	return clients, nil
}

// buildValidators iterates over all the known validators and builds a docker
// image for all unknown ones.
func buildValidators(daemon *docker.Client, pattern string, nocache bool) (map[string]string, error) {
	// Gather all the client validator tests
	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, err
	}
	suite, err := ioutil.ReadDir("validators")
	if err != nil {
		return nil, err
	}
	validators := make(map[string]string)
	for _, group := range suite {
		if group.IsDir() {
			tests, err := ioutil.ReadDir(filepath.Join("validators", group.Name()))
			if err != nil {
				return nil, err
			}
			for _, test := range tests {
				if test.IsDir() {
					if name := filepath.Join(group.Name(), test.Name()); re.MatchString(name) {
						validators[name] = validatorImagePrefix + name
					}
				}
			}
		}
	}
	// Iterate over all the matched validators and build their docker images
	for validator, image := range validators {
		var (
			context = filepath.Join("validators", validator)
			logger  = log15.New("validator", validator)
		)
		if err := buildImage(daemon, image, context, nocache, logger); err != nil {
			return nil, fmt.Errorf("%s: %v", validator, err)
		}
	}
	return validators, nil
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
	logger.Info("successfully built image")
	return nil
}
