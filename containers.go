// This file contains the utility methods for managing docker containers.

package main

import (
	"archive/tar"
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/fsouza/go-dockerclient"
	"gopkg.in/inconshreveable/log15.v2"
)

// hiveEnvvarPrefix is the prefix of the environment variables names that should
// be moved from test images to client container to fine tune their setup.
const hiveEnvvarPrefix = "HIVE_"

// hiveLogsFolder is the directory in which to place runtime logs from each of
// the docker containers.
var hiveLogsFolder = filepath.Join("workspace", "logs")

// createShellContainer creates a docker container from the hive shell's image.
func createShellContainer(daemon *docker.Client, image string, overrides []string) (*docker.Container, error) {
	// Configure any workspace requirements for the container
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	for _, dir := range []string{"docker", "ethash", "logs"} {
		if err := os.MkdirAll(filepath.Join(pwd, "workspace", dir), os.ModePerm); err != nil {
			return nil, err
		}
	}
	// Create the list of bind points to make host files available internally
	binds := make([]string, 0, len(overrides)+2)
	for _, override := range overrides {
		file := override
		if strings.Contains(override, ":") {
			file = override[strings.LastIndex(override, ":")+1:]
		}
		if path, err := filepath.Abs(file); err == nil {
			binds = append(binds, fmt.Sprintf("%s:%s:ro", path, path)) // Mount to the same place, read only
		}
	}
	binds = append(binds, []string{
		fmt.Sprintf("%s/workspace/docker:/var/lib/docker", pwd),                                       // Surface any docker-in-docker data caches
		fmt.Sprintf("%s/workspace/ethash:/gopath/src/github.com/karalabe/hive/workspace/ethash", pwd), // Surface any generated DAGs from the shell
		fmt.Sprintf("%s/workspace/logs:/gopath/src/github.com/karalabe/hive/workspace/logs", pwd),     // Surface all the log files from the shell
	}...)

	// Create and return the actual docker container
	return daemon.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: image,
			Env:   []string{fmt.Sprintf("UID=%d", os.Getuid())}, // Forward the user ID for the workspace permissions
			Cmd:   os.Args[1:],
		},
		HostConfig: &docker.HostConfig{
			Privileged: true, // Docker in docker requires privileged mode
			Binds:      binds,
		},
	})
}

// createEthashContainer creates a docker container to generate ethash DAGs.
func createEthashContainer(daemon *docker.Client, image string) (*docker.Container, error) {
	// Configure the workspace for ethash generation
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ethash := filepath.Join(pwd, "workspace", "ethash")
	if err := os.MkdirAll(ethash, os.ModePerm); err != nil {
		return nil, err
	}
	// Create and return the actual docker container
	return daemon.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: image,
			Env:   []string{fmt.Sprintf("UID=%d", os.Getuid())}, // Forward the user ID for the workspace permissions
		},
		HostConfig: &docker.HostConfig{
			Binds: []string{fmt.Sprintf("%s:/root/.ethash", ethash)},
		},
	})
}

// createClientContainer creates a docker container from a client image and moves
// any hive environment variables and initial chain configuration files from the
// tester image into the new client. Dynamic chain configs may also be pulled from
// a live container.
//
// A batch of environment variables may be specified to override from originating
// from the tester image. This is useful in particular during simulations where
// the tester itself can fine tune parameters for individual nodes.
//
// Also a batch of files may be specified to override either the chain configs or
// the client binaries. This is useful in particular during client development as
// local executables may be injected into a client docker container without them
// needing to be rebuilt inside hive.
func createClientContainer(daemon *docker.Client, client string, tester string, live *docker.Container, overrideFiles []string, overrideEnvs map[string]string) (*docker.Container, error) {
	// Configure the client for ethash consumption
	pwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	ethash := filepath.Join(pwd, "workspace", "ethash")

	// Gather all the hive environment variables from the tester
	ti, err := daemon.InspectImage(tester)
	if err != nil {
		return nil, err
	}
	vars := []string{}
	for _, envvar := range ti.Config.Env {
		if strings.HasPrefix(envvar, hiveEnvvarPrefix) && overrideEnvs[envvar] == "" {
			vars = append(vars, envvar)
		}
	}
	// Inject any explicit envvar overrides
	for key, val := range overrideEnvs {
		if strings.HasPrefix(key, hiveEnvvarPrefix) {
			vars = append(vars, key+"="+val)
		}
	}
	// Create the client container with tester envvars injected
	c, err := daemon.CreateContainer(docker.CreateContainerOptions{
		Config: &docker.Config{
			Image: client,
			Env:   vars,
		},
		HostConfig: &docker.HostConfig{
			Binds: []string{fmt.Sprintf("%s:/root/.ethash", ethash)},
		},
	})
	if err != nil {
		return nil, err
	}
	// Inject all the chain configuration files from the tester (or live container) into the client
	t, err := daemon.CreateContainer(docker.CreateContainerOptions{Config: &docker.Config{Image: tester}})
	if err != nil {
		return nil, err
	}
	defer daemon.RemoveContainer(docker.RemoveContainerOptions{ID: t.ID, Force: true})

	if path := overrideEnvs["HIVE_INIT_GENESIS"]; path != "" {
		err = copyBetweenContainers(daemon, c.ID, live.ID, path, "/genesis.json", false)
	} else {
		err = copyBetweenContainers(daemon, c.ID, t.ID, "", "/genesis.json", false)
	}
	if err != nil {
		return nil, err
	}
	if path := overrideEnvs["HIVE_INIT_CHAIN"]; path != "" {
		err = copyBetweenContainers(daemon, c.ID, live.ID, path, "/chain.rlp", true)
	} else {
		err = copyBetweenContainers(daemon, c.ID, t.ID, "", "/chain.rlp", true)
	}
	if err != nil {
		return nil, err
	}
	if path := overrideEnvs["HIVE_INIT_BLOCKS"]; path != "" {
		err = copyBetweenContainers(daemon, c.ID, live.ID, path, "/blocks", true)
	} else {
		err = copyBetweenContainers(daemon, c.ID, t.ID, "", "/blocks", true)
	}
	if err != nil {
		return nil, err
	}
	if path := overrideEnvs["HIVE_INIT_KEYS"]; path != "" {
		err = copyBetweenContainers(daemon, c.ID, live.ID, path, "/keys", true)
	} else {
		err = copyBetweenContainers(daemon, c.ID, t.ID, "", "/keys", true)
	}
	if err != nil {
		return nil, err
	}
	// Inject any explicit file overrides into the client container
	overrides := make([]string, 0, len(overrideFiles))
	for _, override := range overrideFiles {
		// Split the override into a pattern/path combo
		pattern, file := ".", override
		if strings.Contains(override, ":") {
			pattern = override[:strings.LastIndex(override, ":")]
			file = override[strings.LastIndex(override, ":")+1:]
		}
		// If the pattern matches the client image, override the file
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, err
		}
		if re.MatchString(client) {
			overrides = append(overrides, file)
		}
	}
	if err := uploadToContainer(daemon, c.ID, overrides); err != nil {
		daemon.RemoveContainer(docker.RemoveContainerOptions{ID: c.ID, Force: true})
		return nil, err
	}
	return c, nil
}

// uploadToContainer injects a batch of files into the target container.
func uploadToContainer(daemon *docker.Client, id string, files []string) error {
	// Short circuit if there are no files to upload
	if len(files) == 0 {
		return nil
	}
	// Create a tarball archive with all the data files
	tarball := new(bytes.Buffer)
	tw := tar.NewWriter(tarball)

	for _, path := range files {
		// Fetch the next file to inject into the container
		file, err := os.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		data, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}
		info, err := file.Stat()
		if err != nil {
			return err
		}
		// Insert the file into the tarball archive
		header := &tar.Header{
			Name: filepath.Base(file.Name()),
			Mode: int64(info.Mode()),
			Size: int64(len(data)),
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
	}
	if err := tw.Close(); err != nil {
		return err
	}
	// Upload the tarball into the destination container
	return daemon.UploadToContainer(id, docker.UploadToContainerOptions{
		InputStream: tarball,
		Path:        "/",
	})
}

// copyBetweenContainers copies a file from one docker container to another one.
func copyBetweenContainers(daemon *docker.Client, dest, src string, path, target string, optional bool) error {
	// If no path was specified, use the target as the default
	if path == "" {
		path = target
	}
	// Download a tarball of the file from the source container
	download, upload := new(bytes.Buffer), new(bytes.Buffer)
	if err := daemon.DownloadFromContainer(src, docker.DownloadFromContainerOptions{
		Path:         path,
		OutputStream: download,
	}); err != nil {
		// Check whether we're missing an optional file only
		if err.(*docker.Error).Status == 404 && optional {
			return nil
		}
		return err
	}
	// Rewrite all the paths in the tarball to the default ones
	in, out := tar.NewReader(download), tar.NewWriter(upload)
	for {
		// Fetch the next file header from the download archive
		header, err := in.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		// Rewrite the path and push into the upload archive
		header.Name = strings.Replace(header.Name, path[1:], target[1:], -1)
		if err := out.WriteHeader(header); err != nil {
			return err
		}
		// Copy the file content over from the download to the upload archive
		if _, err := io.Copy(out, in); err != nil {
			return err
		}
	}
	// Upload the tarball into the destination container
	if err := daemon.UploadToContainer(dest, docker.UploadToContainerOptions{
		InputStream: upload,
		Path:        "/",
	}); err != nil {
		return err
	}
	return nil
}

// runContainer attaches to the output streams of an existing container, then
// starts executing the container and returns the CloseWaiter to allow the caller
// to wait for termination.
func runContainer(daemon *docker.Client, id string, logger log15.Logger, logfile string, shell bool) (docker.CloseWaiter, error) {
	// If we're the outer shell, log straight to stderr, nothing fancy
	stdout := io.Writer(os.Stdout)
	stream := io.Writer(os.Stderr)
	if !shell {
		// For non shell containers, create and open the log file for the output
		if err := os.MkdirAll(filepath.Dir(logfile), os.ModePerm); err != nil {
			return nil, err
		}
		log, err := os.OpenFile(logfile, os.O_WRONLY|os.O_CREATE|os.O_SYNC|os.O_TRUNC, os.ModePerm)
		if err != nil {
			return nil, err
		}
		stream = io.Writer(log)

		// If console logging was requested, tee the output and tag it with the container id
		if *loglevelFlag > 5 {
			// Hook into the containers output stream and tee it out
			hookedR, hookedW := io.Pipe()
			stream = io.MultiWriter(log, hookedW)

			// Tag all log messages with the container ID if not the outer shell
			copy := func(dst io.Writer, src io.Reader) (int64, error) {
				scanner := bufio.NewScanner(src)
				for scanner.Scan() {
					dst.Write([]byte(fmt.Sprintf("[%s] %s\n", id[:8], scanner.Text())))
				}
				return 0, nil
			}
			go copy(os.Stderr, hookedR)
		}
		// Only the shell gets to keep its standard output
		stdout = stream
	}
	logger.Debug("attaching to container")
	waiter, err := daemon.AttachToContainerNonBlocking(docker.AttachToContainerOptions{
		Container:    id,
		OutputStream: stdout,
		ErrorStream:  stream,
		Stream:       true,
		Stdout:       true,
		Stderr:       true,
	})
	if err != nil {
		logger.Error("failed to attach to container", "error", err)
		return nil, err
	}
	// Start the requested container and wait until it terminates
	logger.Debug("starting container")
	if err := daemon.StartContainer(id, nil); err != nil {
		logger.Error("failed to start container", "error", err)
		return nil, err
	}
	return waiter, nil
}
