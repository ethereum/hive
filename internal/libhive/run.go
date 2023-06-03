package libhive

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"gopkg.in/inconshreveable/log15.v2"
)

var (
	errSimInterrupt = errors.New("simulation interrupted")
	errSimTimeout   = errors.New("simulation timed out")
)

// Runner executes a simulation runs.
type Runner struct {
	inv       Inventory
	container ContainerBackend
	builder   Builder

	// This holds the image names of all built simulators.
	simImages  map[string]string
	clientDefs map[string]*ClientDefinition
}

func NewRunner(inv Inventory, b Builder, cb ContainerBackend) *Runner {
	return &Runner{
		inv:       inv,
		builder:   b,
		container: cb,
	}
}

// Build builds client and simulator images.
func (r *Runner) Build(ctx context.Context, clientList []ClientDesignator, simList []string) error {
	if err := r.container.Build(ctx, r.builder); err != nil {
		return err
	}
	if err := r.buildClients(ctx, clientList); err != nil {
		return err
	}
	return r.buildSimulators(ctx, simList)
}

// buildClients builds client images.
func (r *Runner) buildClients(ctx context.Context, clientList []ClientDesignator) error {
	if len(clientList) == 0 {
		return errors.New("client list is empty, cannot simulate")
	}

	r.clientDefs = make(map[string]*ClientDefinition, len(clientList))

	var anyBuilt bool
	log15.Info(fmt.Sprintf("building %d clients...", len(clientList)))
	for _, client := range clientList {
		image, err := r.builder.BuildClientImage(ctx, client)
		if err != nil {
			continue
		}
		anyBuilt = true
		version, err := r.builder.ReadFile(ctx, image, "/version.txt")
		if err != nil {
			log15.Warn("can't read version info of "+client.Client, "image", image, "err", err)
		}
		r.clientDefs[client.Name()] = &ClientDefinition{
			Name:    client.Name(),
			Version: strings.TrimSpace(string(version)),
			Image:   image,
			Meta:    r.inv.Clients[client.Client].Meta,
		}
	}
	if !anyBuilt {
		return errors.New("all clients failed to build")
	}
	return nil
}

// buildSimulators builds simulator images.
func (r *Runner) buildSimulators(ctx context.Context, simList []string) error {
	r.simImages = make(map[string]string)

	log15.Info(fmt.Sprintf("building %d simulators...", len(simList)))
	for _, sim := range simList {
		image, err := r.builder.BuildSimulatorImage(ctx, sim)
		if err != nil {
			return err
		}
		r.simImages[sim] = image
	}
	return nil
}

func (r *Runner) Run(ctx context.Context, sim string, env SimEnv) (SimResult, error) {
	if err := createWorkspace(env.LogDir); err != nil {
		return SimResult{}, err
	}
	writeInstanceInfo(env.LogDir)
	return r.run(ctx, sim, env)
}

// RunDevMode starts simulator development mode. In this mode, the simulator is not
// launched and the API server runs on the local network instead of listening for requests
// on the docker network.
//
// Note: Sim* options in env are ignored, but Client* options and LogDir still apply.
func (r *Runner) RunDevMode(ctx context.Context, env SimEnv, endpoint string) error {
	if err := createWorkspace(env.LogDir); err != nil {
		return err
	}

	tm := NewTestManager(env, r.container, r.clientDefs)
	defer func() {
		if err := tm.Terminate(); err != nil {
			log15.Error("could not terminate test manager", "error", err)
		}
	}()

	log15.Debug("starting simulator API proxy")
	proxy, err := r.container.ServeAPI(ctx, tm.API())
	if err != nil {
		log15.Error("can't start proxy", "err", err)
		return err
	}
	defer shutdownServer(proxy)

	log15.Debug("starting local API server")
	listener, err := net.Listen("tcp", endpoint)
	if err != nil {
		log15.Error("can't start TCP server", "err", err)
		return err
	}
	httpsrv := &http.Server{Handler: tm.API()}
	defer httpsrv.Close()
	go func() { httpsrv.Serve(listener) }()

	fmt.Printf(`---
Welcome to hive --dev mode. Run with me:

HIVE_SIMULATOR=http://%v
---
`, listener.Addr())

	// Wait for interrupt.
	<-ctx.Done()
	return nil
}

// run runs one simulation.
func (r *Runner) run(ctx context.Context, sim string, env SimEnv) (SimResult, error) {
	log15.Info(fmt.Sprintf("running simulation: %s", sim))

	clientDefs := make(map[string]*ClientDefinition)
	if env.ClientList == nil {
		// Unspecified, make all clients available.
		for name, def := range r.clientDefs {
			clientDefs[name] = def
		}
	} else {
		for _, client := range env.ClientList {
			def, ok := r.clientDefs[client.Client]
			if !ok {
				return SimResult{}, fmt.Errorf("unknown client %q in simulation client list", client.Client)
			}
			clientDefs[client.Client] = def
		}
	}

	// Start the simulation API.
	tm := NewTestManager(env, r.container, clientDefs)
	defer func() {
		if err := tm.Terminate(); err != nil {
			log15.Error("could not terminate test manager", "error", err)
		}
	}()

	log15.Debug("starting simulator API server")
	server, err := r.container.ServeAPI(ctx, tm.API())
	if err != nil {
		log15.Error("can't start API server", "err", err)
		return SimResult{}, err
	}
	defer shutdownServer(server)

	// Create the simulator container.
	opts := ContainerOptions{
		Env: map[string]string{
			"HIVE_SIMULATOR":    "http://" + server.Addr().String(),
			"HIVE_PARALLELISM":  strconv.Itoa(env.SimParallelism),
			"HIVE_LOGLEVEL":     strconv.Itoa(env.SimLogLevel),
			"HIVE_TEST_PATTERN": env.SimTestPattern,
		},
	}
	containerID, err := r.container.CreateContainer(ctx, r.simImages[sim], opts)
	if err != nil {
		return SimResult{}, err
	}

	// Set the log file, and notify TestManager about the container.
	logbasename := fmt.Sprintf("%d-simulator-%s.log", time.Now().Unix(), containerID)
	opts.LogFile = filepath.Join(env.LogDir, logbasename)
	tm.SetSimContainerInfo(containerID, logbasename)

	log15.Debug("starting simulator container")
	sc, err := r.container.StartContainer(ctx, containerID, opts)
	if err != nil {
		return SimResult{}, err
	}
	slogger := log15.New("sim", sim, "container", sc.ID[:8])
	slogger.Debug("started simulator container")
	defer func() {
		slogger.Debug("deleting simulator container")
		r.container.DeleteContainer(sc.ID)
	}()

	// Wait for simulator exit.
	done := make(chan struct{})
	go func() {
		sc.Wait()
		close(done)
	}()

	// if we have a simulation time limit, apply it.
	var timeout <-chan time.Time
	if env.SimDurationLimit != 0 {
		tt := time.NewTimer(env.SimDurationLimit)
		defer tt.Stop()
		timeout = tt.C
	}

	// Wait for simulation to end.
	select {
	case <-done:
	case <-timeout:
		slogger.Info("simulation timed out")
		err = errSimTimeout
	case <-ctx.Done():
		slogger.Info("interrupted, shutting down")
		err = errSimInterrupt
	}

	// Count the results.
	var result SimResult
	for _, suite := range tm.Results() {
		var suiteFailCounted bool
		result.Suites++
		for _, test := range suite.TestCases {
			result.Tests++
			if !test.SummaryResult.Pass {
				result.TestsFailed++
				if !suiteFailCounted {
					result.SuitesFailed++
					suiteFailCounted = true
				}
			}
		}
	}

	return result, err
}

// shutdownServer gracefully terminates the HTTP server.
func shutdownServer(server APIServer) {
	log15.Debug("terminating simulator API server")
	if err := server.Close(); err != nil {
		log15.Debug("API server shutdown failed", "err", err)
	}
}

// createWorkspace ensures that the hive output directory exists.
func createWorkspace(logdir string) error {
	stat, err := os.Stat(logdir)
	if err != nil {
		if os.IsNotExist(err) {
			log15.Info("creating output directory", "folder", logdir)
			err = os.MkdirAll(logdir, 0755)
			if err != nil {
				log15.Crit("failed to create output directory", "err", err)
			}
		}
		return err
	}
	if !stat.IsDir() {
		return errors.New("log output directory is a file")
	}
	return nil
}

func writeInstanceInfo(logdir string) {
	var obj HiveInstance
	obj.SourceCommit, obj.SourceDate = hiveVersion()
	buildDate := hiveBuildTime()
	if !buildDate.IsZero() {
		obj.BuildDate = buildDate.Format("2006-01-02T15:04:05Z")
	}

	enc, _ := json.Marshal(&obj)
	err := os.WriteFile(filepath.Join(logdir, "hive.json"), enc, 0644)
	if err != nil {
		log15.Warn("can't write hive.json", "err", err)
	}
}

func hiveVersion() (commit, date string) {
	if buildInfo, ok := debug.ReadBuildInfo(); ok {
		for _, v := range buildInfo.Settings {
			switch v.Key {
			case "vcs.revision":
				commit = v.Value
			case "vcs.time":
				date = v.Value
			}
		}
	}
	return commit, date
}

func hiveBuildTime() time.Time {
	exe, err := os.Executable()
	if err != nil {
		return time.Time{}
	}
	stat, err := os.Stat(exe)
	if err != nil {
		return time.Time{}
	}
	return stat.ModTime()
}
