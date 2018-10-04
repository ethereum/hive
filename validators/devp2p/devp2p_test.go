package main

import (
	"flag"
	"os"
	"testing"
)

var ()

// Examples:

func TestMain(m *testing.M) {
	flag.Parse()
	//... = flag.Args()

	os.Exit(m.Run())
}

// TestDiscovery tests the set of discovery protocols
func TestDiscovery(t *testing.T) {
	// discovery v4 test suites
	t.Run("discoveryv4", func(t *testing.T) {
		//
		t.Run("ping", func(t *testing.T) {

			//with the use of helper functions
			//.signal that the other hive client should be reset
			//.get the endpoint of the other hive client
			//.generate a ping message and send to the other hive client
			//.wait for messages and decode them
			//TODO: research geth for above.

		})
	})

	t.Run("discoveryv5", func(t *testing.T) {
		//
		t.Run("ping", func(t *testing.T) {

		})
	})

}

// TestRLPx checks the RLPx handshaking
func TestRLPx(t *testing.T) {
	// discovery v4 test suites
	t.Run("connect", func(t *testing.T) {
		//
		t.Run("basic", func(t *testing.T) {

		})
	})

}
