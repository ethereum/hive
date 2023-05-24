package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pkg/errors"

	"github.com/ethereum/hive/hivesim"
	"github.com/golang/snappy"
	"github.com/holiman/uint256"
	beacon_client "github.com/marioevz/eth-clients/clients/beacon"
	"github.com/protolambda/zrnt/eth2/beacon/common"
	"github.com/protolambda/zrnt/eth2/configs"
	"github.com/protolambda/ztyp/view"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

const (
	PortBeaconTCP    = 9000
	PortBeaconUDP    = 9000
	PortBeaconAPI    = 4000
	PortBeaconGRPC   = 4001
	PortMetrics      = 8080
	PortValidatorAPI = 5000
	FarFutureEpoch   = common.Epoch(0xffffffffffffffff)
)

var log = logrus.New()

func main() {
	// Create the test suite
	suite := hivesim.Suite{
		Name: "beacon-api",
	}
	// Add the tests to the suite
	suite.Add(hivesim.TestSpec{
		Name:        "test file loader",
		Description: "This is a meta-test. It loads the tests for the beacon api.",
		Run:         loaderTest,
		AlwaysRun:   true,
	})
	// Init logrus

	// Run the test suite
	hivesim.MustRunSuite(hivesim.New(), suite)
}

// loaderTest loads the blockchain test files and spawns the client tests.
func loaderTest(t *hivesim.T) {
	clientTypes, err := t.Sim.ClientTypes()
	if err != nil {
		t.Fatal("can't get client types:", err)
	}

	parallelism := 16
	if val, ok := os.LookupEnv("HIVE_PARALLELISM"); ok {
		if p, err := strconv.Atoi(val); err != nil {
			t.Logf("Warning: invalid HIVE_PARALLELISM value %q", val)
		} else {
			parallelism = p
		}
	}
	t.Log("parallelism:", parallelism)

	// Find the tests directory.
	/*
		testPath, isset := os.LookupEnv("TESTPATH")
		if !isset {
			t.Fatal("$TESTPATH not set")
		}
		fileRoot := fmt.Sprintf("%s/BlockchainTests/", testPath)
	*/

	// Spawn workers.
	var wg sync.WaitGroup
	var testCh = make(chan *BeaconAPITest)
	wg.Add(parallelism)
	for i := 0; i < parallelism; i++ {
		go func() {
			defer wg.Done()
			for test := range testCh {
				t.Run(hivesim.TestSpec{
					Name: test.Name,
					Run:  test.Run,
					// Regexp matching on Name is disabled here because it's already done
					// in loadTests. Matching in loadTests is better because it has access
					// to the full test file path.
					AlwaysRun: true,
				})
			}
		}()
	}

	_, testPattern := t.Sim.TestPattern()
	re := regexp.MustCompile(testPattern)

	// Deliver test cases.
	loadTests(t, "tests", re, func(tc BeaconAPITest) {
		for _, client := range clientTypes {
			if !client.HasRole("beacon") {
				continue
			}
			tc := tc // shallow copy
			tc.ClientType = client
			testCh <- &tc
		}
	})
	close(testCh)

	// Wait for workers to finish.
	wg.Wait()
}

func checkTestFiles(dirPath string) bool {
	requiredFiles := []string{"post.ssz_snappy", "hive.yaml", "blocks_0.ssz_snappy", "meta.yaml"}
	// Check that all minimum required files are present
	for _, file := range requiredFiles {
		if _, err := os.Stat(path.Join(dirPath, file)); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func loadTests(t *hivesim.T, root string, re *regexp.Regexp, fn func(BeaconAPITest)) {
	// Walk through the tests directory, each directory is a test
	filepath.Walk(root, func(walkpath string, info os.FileInfo, err error) error {
		if err != nil {
			t.Logf("unable to walk path: %s", err)
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if !checkTestFiles(walkpath) {
			return nil // skip
		}
		if !re.MatchString(info.Name()) {
			fmt.Println("skip (regex string)", info.Name())
			return nil // skip
		}
		testName := walkpath
		testName = strings.TrimPrefix(testName, filepath.FromSlash(root+"/"))
		testName = strings.TrimPrefix(testName, filepath.FromSlash("hive/"))
		test := BeaconAPITest{
			Name: testName,
			Path: walkpath,
		}
		if err := test.LoadFromDirectory(walkpath); err != nil {
			fmt.Println("skip (load from dir)", walkpath, err)
			return nil
		}
		if preset_str, _, err := test.PresetFork(); err != nil {
			fmt.Println("skip (preset fork)", info.Name(), err)
			return nil
		} else if preset_str != "hive" {
			fmt.Println("skip", info.Name(), preset_str)
			return nil
		}
		fn(test)
		return nil
	})
}

type BeaconAPIConfig struct {
	GenesisTime int `yaml:"genesis_time"`
	Time        int `yaml:"time"`
}

type BeaconAPITest struct {
	// The name of the test
	Name string
	// Path to the test dir
	Path string
	// Post state
	Post []byte
	// Post state
	Pre []byte
	// Genesis state
	Genesis []byte
	// The blocks to process
	Blocks [][]byte
	// The checks to perform
	Verifications BeaconAPITestSteps
	// Config
	Config BeaconAPIConfig

	ClientType *hivesim.ClientDefinition
	TestP2P    *TestP2P
}

func (b *BeaconAPITest) LoadFromDirectory(dirPath string) error {
	// Read and uncompress post state
	compressedPost, err := os.ReadFile(path.Join(dirPath, "post.ssz_snappy"))
	if err != nil {
		return err
	}
	// Uncompress post state using snappy
	b.Post, err = snappy.Decode(nil, compressedPost)
	if err != nil {
		return err
	}
	// Read and uncompress genesis state
	compressedGenesis, err := os.ReadFile(path.Join(dirPath, "genesis.ssz_snappy"))
	if err != nil {
		return err
	}
	// Uncompress genesis state using snappy
	b.Genesis, err = snappy.Decode(nil, compressedGenesis)
	if err != nil {
		return err
	}

	// Read and uncompress blocks: each block is a file with the following format: blocks_{i}.ssz_snappy
	// where i is the index of the block
	// Read each block starting from zero until the block file does not exist
	b.Blocks = make([][]byte, 0)
	for i := 0; ; i++ {
		blockPath := path.Join(dirPath, fmt.Sprintf("blocks_%d.ssz_snappy", i))
		if _, err := os.Stat(blockPath); os.IsNotExist(err) {
			break
		}
		// Read and uncompress block
		compressedBlock, err := os.ReadFile(blockPath)
		if err != nil {
			return err
		}
		block, err := snappy.Decode(nil, compressedBlock)
		if err != nil {
			return err
		}
		b.Blocks = append(b.Blocks, block)
	}

	// Read checks
	checksPath := path.Join(dirPath, "hive.yaml")
	if _, err := os.Stat(checksPath); os.IsNotExist(err) {
		return err
	}
	checksFile, err := os.ReadFile(checksPath)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(checksFile, &b.Verifications); err != nil {
		panic(err)
	}

	return nil
}

func (b *BeaconAPITest) Run(t *hivesim.T) {
	// Get the simulator IP
	simIP, err := t.Sim.ContainerNetworkIP(
		t.SuiteID,
		"bridge",
		"simulation",
	)
	if err != nil {
		panic(err)
	}

	// Init P2P
	testP2P, err := NewTestP2P(net.ParseIP(simIP), 9000)
	if err != nil {
		t.Fatalf("failed to create p2p object: %v", err)
	}
	b.TestP2P = testP2P
	defer testP2P.Close()
	if err := testP2P.SetupStreams(); err != nil {
		t.Fatalf("failed to setup streams: %v", err)
	}

	t.Logf("P2P Host ID: %s", testP2P.Host.ID().Pretty())
	t.Logf("LocalNode: %s", testP2P.LocalNode.Node().String())

	configBundle, err := b.ConfigBundle(testP2P.LocalNode.Node().String())
	if err != nil {
		t.Fatalf("failed to create config bundle: %v", err)
	}
	cm := &HiveManagedClient{
		T:                    t,
		HiveClientDefinition: b.ClientType,
	}
	cm.extraStartOptions = []hivesim.StartOption{
		configBundle,
	}
	spec, err := b.ConsensusConfig()
	if err != nil {
		t.Fatalf("failed to create consensus config: %v", err)
	}

	cl := &beacon_client.BeaconClient{
		Client: cm,
		Logger: t,
		Config: beacon_client.BeaconClientConfig{
			ClientIndex:             0,
			TerminalTotalDifficulty: int64(0),
			Spec:                    spec,
			// GenesisValidatorsRoot:   &testnet.genesisValidatorsRoot,
			GenesisTime: &spec.MIN_GENESIS_TIME,
		},
	}
	if err := cl.Start(); err != nil {
		t.Fatalf("failed to start client: %v", err)
	}
	b.Verifications.DoVerifications(t, context.Background(), cl)
	time.Sleep(5 * time.Second)
}

func BytesSource(data []byte) func() (io.ReadCloser, error) {
	return func() (io.ReadCloser, error) {
		return ioutil.NopCloser(bytes.NewReader(data)), nil
	}
}

func (b *BeaconAPITest) PresetFork() (string, string, error) {
	paths := strings.Split(b.Path, string(os.PathSeparator))
	if len(paths) < 3 {
		return "", "", fmt.Errorf("unable to extract config from path: %s", b.Path)
	}
	return paths[1], paths[2], nil
}
func (b *BeaconAPITest) ConsensusConfig() (*common.Spec, error) {
	preset_str, fork_str, err := b.PresetFork()
	if err != nil {
		return nil, err
	}
	var spec *common.Spec
	switch preset_str {
	case "mainnet":
		specCpy := *configs.Mainnet
		spec = &specCpy
	case "minimal":
		specCpy := *configs.Minimal
		spec = &specCpy
	case "hive":
		specCpy := *configs.Mainnet
		spec = &specCpy
		spec.Config.GENESIS_FORK_VERSION = common.Version{0x00, 0x00, 0x00, 0x0a}
		spec.Config.ALTAIR_FORK_VERSION = common.Version{0x01, 0x00, 0x00, 0x0a}
		spec.Config.BELLATRIX_FORK_VERSION = common.Version{0x02, 0x00, 0x00, 0x0a}
		spec.Config.CAPELLA_FORK_VERSION = common.Version{0x03, 0x00, 0x00, 0x0a}
		spec.Config.DENEB_FORK_VERSION = common.Version{0x04, 0x00, 0x00, 0x0a}
	default:
		return nil, fmt.Errorf("unknown preset: %s", preset_str)
	}
	// Set genesis time

	// Reset fork epochs to far future
	spec.Config.ALTAIR_FORK_EPOCH = FarFutureEpoch
	spec.Config.BELLATRIX_FORK_EPOCH = FarFutureEpoch
	spec.Config.CAPELLA_FORK_EPOCH = FarFutureEpoch
	spec.Config.DENEB_FORK_EPOCH = FarFutureEpoch

	tdd := uint256.NewInt(0)

	switch fork_str {
	case "phase0":
		// Nothing to do
	case "altair":
		spec.Config.ALTAIR_FORK_EPOCH = 0
	case "bellatrix":
		spec.Config.ALTAIR_FORK_EPOCH = 0
		spec.Config.BELLATRIX_FORK_EPOCH = 0
	case "capella":
		spec.Config.ALTAIR_FORK_EPOCH = 0
		spec.Config.BELLATRIX_FORK_EPOCH = 0
		spec.Config.CAPELLA_FORK_EPOCH = 0
		spec.Config.TERMINAL_TOTAL_DIFFICULTY = view.Uint256View(*tdd)
	case "deneb":
		spec.Config.ALTAIR_FORK_EPOCH = 0
		spec.Config.BELLATRIX_FORK_EPOCH = 0
		spec.Config.CAPELLA_FORK_EPOCH = 0
		spec.Config.DENEB_FORK_EPOCH = 0
		spec.Config.TERMINAL_TOTAL_DIFFICULTY = view.Uint256View(*tdd)
	default:
		return nil, fmt.Errorf("unknown fork: %s", fork_str)
	}

	return spec, nil
}

func (b *BeaconAPITest) ConfigBundle(bootnodeENR string) (hivesim.StartOption, error) {
	// Get the config
	spec, err := b.ConsensusConfig()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get consensus config")
	}
	specConfig, err := yaml.Marshal(spec.Config)
	if err != nil {
		return nil, err
	}
	return hivesim.Bundle(

		hivesim.Params{
			"HIVE_CHECK_LIVE_PORT": fmt.Sprintf(
				"%d",
				PortBeaconAPI,
			),
			"HIVE_CLIENT_CLOCK":       fmt.Sprintf("%d", b.Config.Time),
			"HIVE_ETH2_BOOTNODE_ENRS": bootnodeENR,
		},
		hivesim.WithDynamicFile(
			"/hive/input/config.yaml",
			BytesSource(specConfig),
		),
		hivesim.WithDynamicFile(
			"/hive/input/genesis.ssz",
			BytesSource(b.Genesis),
		),
		hivesim.WithDynamicFile(
			"/hive/input/checkpoint_block.ssz",
			BytesSource(b.Blocks[len(b.Blocks)-1]),
		),
		hivesim.WithDynamicFile(
			"/hive/input/checkpoint_state.ssz",
			BytesSource(b.Post),
		),
	), nil
}
