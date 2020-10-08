package main

import (
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/inconshreveable/log15.v2"

	"github.com/ethereum/hive/simulators/common"

	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/hive/simulators/common/providers/hive"
)

func main() {
	host := hive.New()

	availableClients, err := host.GetClientTypes()
	if err != nil {
		log.Error("could not get client types: ", err.Error())
	}
	log.Info("Got clients", "clients", availableClients)

	logFile, _ := os.LookupEnv("HIVE_SIMLOG")

	for _, client := range availableClients {
		suiteID, err := host.StartTestSuite("iptest", "ip test", logFile)
		if err != nil {
			log.Error("unable to start test suite: ", err.Error())
			os.Exit(1)
		}

		testID, err := host.StartTest(suiteID, "iptest", "iptest")
		if err != nil {
			log.Error("unable to start test: ", err.Error())
			os.Exit(1)
		}

		env := map[string]string{
			"CLIENT": client,
		}
		files := map[string]string{}

		containerID, ip, _, err := host.GetNode(suiteID, testID, env, files)
		if err != nil {
			log.Error("could not get node", "err", err.Error())
			os.Exit(1)
		}

		ourOwnContainerID, err := host.GetSimContainerID(suiteID)
		if err != nil {
			log.Error("could not get sim container IP", "err", err.Error())
			os.Exit(1)
		}
		log.Info("OUR OWN CONTAINER ID", "ID", ourOwnContainerID)

		networkID, err := host.CreateNetwork(suiteID, "network1")
		if err != nil {
			log.Error("could not create network", "err", err.Error())
			os.Exit(1)
		}
		// TODO how to connect own sim container to this network
		network2ID, err := host.CreateNetwork(suiteID, "network2")
		if err != nil {
			log.Error("could not create network", "err", err.Error())
			os.Exit(1)
		}

		// connect client to network
		if err := host.ConnectContainerToNetwork(suiteID, networkID, containerID); err != nil {
			log.Error("could not connect container to network", "err", err.Error())
			os.Exit(1)
		}
		// connect sim to network
		if err := host.ConnectContainerToNetwork(suiteID, networkID, ourOwnContainerID); err != nil {
			log.Error("could not connect container to network", "err", err.Error())
			os.Exit(1)
		}
		// connect sim to network2
		if err := host.ConnectContainerToNetwork(suiteID, network2ID, ourOwnContainerID); err != nil {
			log.Error("could not connect container to network", "err", err.Error())
			os.Exit(1)
		}

		// get client ip
		clientIP, err := host.GetContainerNetworkIP(suiteID, networkID, containerID)
		if err != nil {
			log.Error("could not get client network ip addresses", "err", err.Error())
			os.Exit(1)
		}

		//get our own ip
		simIP, err := host.GetContainerNetworkIP(suiteID, networkID, ourOwnContainerID)
		if err != nil {
			log.Error("could not get simulation network ip addresses", "err", err.Error())
			os.Exit(1)
		}
		// get our 2nd IP on network 2
		simIP2, err := host.GetContainerNetworkIP(suiteID, network2ID, ourOwnContainerID)
		if err != nil {
			log.Error("could not get simulation network ip addresses", "err", err.Error())
			os.Exit(1)
		}
		// get sim bridge ip
		simIPBridge, err := host.GetContainerNetworkIP(suiteID, "5717936aecb1b2bd47bd4ddae9a780ba969719784956f5b6a3c47f4151e216c9", ourOwnContainerID) // TODO hardcoded ID of bridge, lets see if this works
		if err != nil {
			log.Error("could not get simulation network ip addresses", "err", err.Error())
			os.Exit(1)
		}
		// get client bridge ip
		clientIPBridge, err := host.GetContainerNetworkIP(suiteID, "5717936aecb1b2bd47bd4ddae9a780ba969719784956f5b6a3c47f4151e216c9", containerID) // TODO hardcoded ID of bridge, lets see if this works
		if err != nil {
			log.Error("could not get simulation network ip addresses", "err", err.Error())
			os.Exit(1)
		}

		log15.Crit("got bridge IP: ", "ip", ip)
		log15.Crit("got network1 ip for client", "ip", clientIP)
		log15.Crit("got network1 ip for sim", "ip", simIP)
		log15.Crit("got network2 ip for sim", "IP", simIP2)
		log15.Crit("got bridge ip for client", "ip", clientIPBridge)
		log15.Crit("got bridge ip for sim", "IP", simIPBridge)

		wg := sync.WaitGroup{}
		wg.Add(1)
		go func(wg sync.WaitGroup) {
			time.Sleep(5000)
			wg.Done()
		}(wg)
		wg.Wait()

		host.KillNode(suiteID, testID, containerID)
		host.EndTest(suiteID, testID, &common.TestResult{Pass: true, Details: fmt.Sprint("clientIP: %s, simIP: %s", clientIP, simIP)}, nil)
		host.EndTestSuite(suiteID)
	}
}
